// +build darwin

package cloner_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/voidingwarranties/offsite-apfs-backup/asr"
	"github.com/voidingwarranties/offsite-apfs-backup/cloner"
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
	"github.com/voidingwarranties/offsite-apfs-backup/testutils/diskimage"
)

var mounter = diskimage.Mounter{
	Relpath: "../testutils/diskimage",
}

func TestCloneable(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T)
		opts    []cloner.Option
		source  string
		targets []string
	}{
		{
			name: "incremental clone",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.TargetImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.TargetImg.UUID(t)},
		},
		{
			name: "initialize targets",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.UninitializedTargetImg)
			},
			opts:    []cloner.Option{cloner.InitializeTargets(true)},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.UninitializedTargetImg.UUID(t)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)
			du := diskutil.New()
			// nil so that test panics of any asr methods are called.
			var r asr.ASR = nil
			c := cloner.New(du, r, test.opts...)
			if err := c.Cloneable(test.source, test.targets...); err != nil {
				t.Errorf("Cloneable returned error: %q, want: nil", err)
			}
		})
	}
}

func TestCloneable_Errors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T)
		opts    []cloner.Option
		source  string
		targets []string
	}{
		{
			name: "source not a volume",
			setup: func(t *testing.T) {
				mounter.MountRW(t, diskimage.TargetImg)
			},
			source:  "not-a-uuid",
			targets: []string{diskimage.TargetImg.UUID(t)},
		},
		{
			name: "one of the targets is not a volume",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.TargetImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.TargetImg.UUID(t), "not-a-uuid"},
		},
		{
			name: "same target repeated multiple times",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.TargetImg)
			},
			source: diskimage.SourceImg.UUID(t),
			targets: []string{
				diskimage.TargetImg.UUID(t),
				diskimage.TargetImg.UUID(t),
			},
		},
		{
			name: "source and target are same",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.SourceImg.UUID(t)},
		},
		{
			name: "target not writable",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRO(t, diskimage.TargetImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.TargetImg.UUID(t)},
		},
		{
			name: "target not an APFS volume",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.HFSImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.HFSImg.UUID(t)},
		},
		{
			name: "target is case sensitive, but source is not",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.CaseSensitiveAPFSImg)
			},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.CaseSensitiveAPFSImg.UUID(t)},
		},
		{
			name: "initialize - target has snaps",
			setup: func(t *testing.T) {
				mounter.MountRO(t, diskimage.SourceImg)
				mounter.MountRW(t, diskimage.TargetImg)
			},
			opts:    []cloner.Option{cloner.InitializeTargets(true)},
			source:  diskimage.SourceImg.UUID(t),
			targets: []string{diskimage.TargetImg.UUID(t)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)
			du := diskutil.New()
			// nil so that test panics of any asr methods are called.
			var r asr.ASR = nil
			c := cloner.New(du, r, test.opts...)
			if err := c.Cloneable(test.source, test.targets...); err == nil {
				t.Error("Cloneable returned error: nil, want: non-nil")
			}
		})
	}
}

func TestClone_DryRun(t *testing.T) {
	sourceInfo := mounter.MountRO(t, diskimage.SourceImg)
	targetInfo := mounter.MountRW(t, diskimage.TargetImg)

	wantTargetSnaps := diskimage.TargetImg.Snapshots(t)

	du := diskutil.NewDryRun(diskutil.New())
	r := asr.NewDryRun()
	c := cloner.New(du, r)
	if err := c.Clone(sourceInfo.Device, targetInfo.Device); err != nil {
		t.Fatalf("Clone returned unexpected error: %q, want: nil", err)
	}

	t.Run("target's volume not modified", func(t *testing.T) {
		gotInfo, err := du.Info(targetInfo.Device)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(targetInfo, gotInfo); diff != "" {
			t.Errorf("Clone resulted in unexpected target info. -want +got:\n%s", diff)
		}
	})
	t.Run("target's snapshots not modified", func(t *testing.T) {
		gotSnaps, err := du.ListSnapshots(targetInfo)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(wantTargetSnaps, gotSnaps); diff != "" {
			t.Errorf("Clone resulted in unexpected target snapshots. -want +got:\n%s", diff)
		}
	})
}

func TestClone_Incremental(t *testing.T) {
	tests := []struct {
		name            string
		opts            []cloner.Option
		setup           func(*testing.T) (source, target string)
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name: "default options",
			setup: func(t *testing.T) (source, target string) {
				sourceInfo := mounter.MountRO(t, diskimage.SourceImg)
				targetInfo := mounter.MountRW(t, diskimage.TargetImg)
				return sourceInfo.Device, targetInfo.Device
			},
			wantTargetSnaps: diskimage.SourceImg.Snapshots(t),
		},
		{
			name: "prune",
			opts: []cloner.Option{cloner.Prune(true)},
			setup: func(t *testing.T) (source, target string) {
				sourceInfo := mounter.MountRO(t, diskimage.SourceImg)
				targetInfo := mounter.MountRW(t, diskimage.TargetImg)
				return sourceInfo.Device, targetInfo.Device
			},
			wantTargetSnaps: diskimage.SourceImg.Snapshots(t)[:1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, target := test.setup(t)
			du := diskutil.New()
			wantTargetInfo, err := du.Info(target)
			if err != nil {
				t.Fatal(err)
			}

			c := cloner.New(diskutil.New(), asr.New(), test.opts...)
			if err := c.Clone(source, target); err != nil {
				t.Fatalf("Clone returned unexpected error: %v, want: nil", err)
			}

			gotTargetInfo, err := du.Info(target)
			if err != nil {
				t.Fatal(err)
			}
			t.Run("target renamed to original name", func(t *testing.T) {
				// Ignore mount point because `asr` remounts the target in the default
				// /Volumes mount root, which will be different than our temporary test
				// directory mount point.
				ignoreMountPointOpt := cmpopts.IgnoreFields(diskutil.VolumeInfo{}, "MountPoint")
				if diff := cmp.Diff(wantTargetInfo, gotTargetInfo, ignoreMountPointOpt); diff != "" {
					t.Errorf("Clone resulted in unexpected target VolumeInfo. -want +got:\n%s", diff)
				}
			})
			t.Run("target has expected snapshots", func(t *testing.T) {
				gotTargetSnaps, err := du.ListSnapshots(gotTargetInfo)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(test.wantTargetSnaps, gotTargetSnaps); diff != "" {
					t.Errorf("Clone resulted in unexpected target snapshots. -want +got:\n%s", diff)
				}
			})
		})
	}
}

func TestClone_InitializeTargets(t *testing.T) {
	source := mounter.MountRO(t, diskimage.SourceImg).Device
	target := mounter.MountRW(t, diskimage.UninitializedTargetImg).Device

	du := diskutil.New()
	wantTargetInfo, err := du.Info(target)
	if err != nil {
		t.Fatal(err)
	}

	c := cloner.New(diskutil.New(), asr.New(), cloner.InitializeTargets(true))
	if err := c.Clone(source, target); err != nil {
		t.Fatalf("Clone returned unexpected error: %v, want: nil", err)
	}

	gotTargetInfo, err := du.Info(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("target renamed to original name", func(t *testing.T) {
		// Ignore mount point because `asr` remounts the target in the default
		// /Volumes mount root, which will be different than our temporary test
		// directory mount point.
		//
		// Ignore UUID because `asr` without a `--fromSnapshot` arg
		// will change the UUID of a volume.
		cmpOpt := cmpopts.IgnoreFields(diskutil.VolumeInfo{}, "MountPoint", "UUID")
		if diff := cmp.Diff(wantTargetInfo, gotTargetInfo, cmpOpt); diff != "" {
			t.Errorf("Clone resulted in unexpected target VolumeInfo. -want +got:\n%s", diff)
		}
	})
	t.Run("target has latest source snapshot", func(t *testing.T) {
		gotTargetSnaps, err := du.ListSnapshots(gotTargetInfo)
		if err != nil {
			t.Fatal(err)
		}
		wantTargetSnaps := []diskutil.Snapshot{
			diskimage.SourceImg.Snapshots(t)[0],
		}
		if diff := cmp.Diff(wantTargetSnaps, gotTargetSnaps); diff != "" {
			t.Errorf("Clone resulted in unexpected target snapshots. -want +got:\n%s", diff)
		}
	})
}

func TestClone_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (source, target string)
		opts  []cloner.Option
	}{
		{
			name: "source and target swapped",
			setup: func(t *testing.T) (source, target string) {
				sourceInfo := mounter.MountRO(t, diskimage.TargetImg)
				targetInfo := mounter.MountRW(t, diskimage.SourceImg)
				return sourceInfo.Device, targetInfo.Device
			},
		},
		{
			name: "initialize targets - target has snaps",
			setup: func(t *testing.T) (source, target string) {
				sourceInfo := mounter.MountRO(t, diskimage.SourceImg)
				targetInfo := mounter.MountRW(t, diskimage.TargetImg)
				return sourceInfo.Device, targetInfo.Device
			},
			opts: []cloner.Option{cloner.InitializeTargets(true)},
		},
		{
			name: "initialize targets - source does not have snaps",
			setup: func(t *testing.T) (source, target string) {
				sourceInfo := mounter.MountRO(t, diskimage.UninitializedTargetImg)
				targetInfo := mounter.MountRW(t, diskimage.TargetImg)
				return sourceInfo.Device, targetInfo.Device
			},
			opts: []cloner.Option{cloner.InitializeTargets(true)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, target := test.setup(t)
			du := diskutil.New()
			// nil so that test panics of any asr methods are called.
			var r asr.ASR = nil
			c := cloner.New(du, r, test.opts...)
			if err := c.Clone(source, target); err == nil {
				t.Fatal("Clone returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
