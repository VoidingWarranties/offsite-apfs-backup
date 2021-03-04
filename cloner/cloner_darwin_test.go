// +build darwin

package cloner_test

import (
	"path/filepath"
	"testing"

	"apfs-snapshot-diff-clone/cloner"
	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/testutils/diskimage"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	sourceImg = filepath.Join("../testutils/diskimage", diskimage.SourceImg)
	targetImg = filepath.Join("../testutils/diskimage", diskimage.TargetImg)

	sourceInfo = diskimage.SourceInfo
	targetInfo = diskimage.TargetInfo

	sourceSnaps = diskimage.SourceSnaps
	targetSnaps = diskimage.TargetSnaps
)

func TestClone(t *testing.T) {
	tests := []struct {
		name            string
		opts            []cloner.Option
		setup           func(*testing.T) (source, target string)
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name: "default options",
			setup: func(t *testing.T) (source, target string) {
				_, sourceDevice := diskimage.MountRO(t, sourceImg)
				_, targetDevice := diskimage.MountRW(t, targetImg)
				return sourceDevice, targetDevice
			},
			wantTargetSnaps: sourceSnaps[:],
		},
		{
			name: "prune",
			opts: []cloner.Option{cloner.Prune(true)},
			setup: func(t *testing.T) (source, target string) {
				_, sourceDevice := diskimage.MountRO(t, sourceImg)
				_, targetDevice := diskimage.MountRW(t, targetImg)
				return sourceDevice, targetDevice
			},
			wantTargetSnaps: sourceSnaps[:1],
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

			c := cloner.New(test.opts...)
			if err := c.Clone(source, target); err != nil {
				t.Fatalf("Clone returned unexpected error: %v, want: nil", err)
			}

			gotTargetInfo, err := du.Info(target)
			if err != nil {
				t.Fatal(err)
			}
			// Ignore mount point because `asr` remounts the target in the default
			// /Volumes mount root, which will be different than our temporary test
			// directory mount point.
			ignoreMountPointOpt := cmpopts.IgnoreFields(diskutil.VolumeInfo{}, "MountPoint")
			if diff := cmp.Diff(wantTargetInfo, gotTargetInfo, ignoreMountPointOpt); diff != "" {
				t.Errorf("Clone resulted in unexpected target VolumeInfo. -want +got:\n%s", diff)
			}

			gotTargetSnaps, err := du.ListSnapshots(gotTargetInfo)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.wantTargetSnaps, gotTargetSnaps); diff != "" {
				t.Errorf("Clone resulted in unexpected target snapshots. -want +got:\n%s", diff)
			}
		})
	}
}

func TestCloner_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (source, target string)
	}{
		{
			name: "source and target swapped",
			setup: func(t *testing.T) (source, target string) {
				_, sourceDevice := diskimage.MountRO(t, targetImg)
				_, targetDevice := diskimage.MountRW(t, sourceImg)
				return sourceDevice, targetDevice
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, target := test.setup(t)
			c := cloner.New()
			if err := c.Clone(source, target); err == nil {
				t.Fatal("Clone returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
