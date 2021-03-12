package cloner

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/voidingwarranties/offsite-apfs-backup/asr"
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

func TestCloneable(t *testing.T) {
	source := diskutil.VolumeInfo{
		Name:           "source-name",
		UUID:           "123-source-uuid",
		MountPoint:     "/source/mount/point",
		Device:         "/dev/disk-source",
		Writable:       false,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	caseSensitiveSource := diskutil.VolumeInfo{
		Name:           "case-sensitive-source-name",
		UUID:           "123-case-sensitive-source-uuid",
		MountPoint:     "/case-senstive-source/mount/point",
		Device:         "/dev/disk-case-sensitive-source",
		Writable:       false,
		FileSystemType: "apfs",
		FileSystem:     "Case-sensitive APFS",
	}
	target1 := diskutil.VolumeInfo{
		Name:           "target1-name",
		UUID:           "123-target1-uuid",
		MountPoint:     "/target1/mount/point",
		Device:         "/dev/disk-target1",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	caseSensitiveTarget1 := diskutil.VolumeInfo{
		Name:           "target1-name",
		UUID:           "123-target1-uuid",
		MountPoint:     "/target1/mount/point",
		Device:         "/dev/disk-target1",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "Case-sensitive APFS",
	}
	target2 := diskutil.VolumeInfo{
		Name:           "target2-name",
		UUID:           "123-target2-uuid",
		MountPoint:     "/target2/mount/point",
		Device:         "/dev/disk-target2",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	uninitializedTarget := diskutil.VolumeInfo{
		Name:           "uninitialized-target-name",
		UUID:           "123-unitialized-target-uuid",
		MountPoint:     "/uninitialized-target/mount/point",
		Device:         "/dev/disk-uninitialized-target",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}

	latestSnap := diskutil.Snapshot{
		Name: "latest-snap",
		UUID: "latest-snap-uuid",
	}
	commonSnap1 := diskutil.Snapshot{
		Name: "common-snap-1",
		UUID: "common-snap-1-uuid",
	}
	commonSnap2 := diskutil.Snapshot{
		Name: "common-snap-2",
		UUID: "common-snap-2-uuid",
	}

	tests := []struct {
		name        string
		fakeDevices *fakeDevices
		opts        []Option
		source      string
		targets     []string
	}{
		{
			name: "single target",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap2, commonSnap1),
				withFakeVolume(target1, commonSnap1),
			),
			source:  source.MountPoint,
			targets: []string{target1.MountPoint},
		},
		{
			name: "multiple targets",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap2, commonSnap1),
				withFakeVolume(target1, commonSnap1),
				withFakeVolume(target2, commonSnap2),
			),
			source:  source.Device,
			targets: []string{target1.Device, target2.UUID},
		},
		{
			name: "Case-sensitive APFS",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(caseSensitiveSource, latestSnap, commonSnap1),
				withFakeVolume(caseSensitiveTarget1, commonSnap1),
			),
			source:  caseSensitiveSource.Device,
			targets: []string{caseSensitiveTarget1.MountPoint},
		},
		{
			name: "initialize target",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap),
				withFakeVolume(uninitializedTarget),
			),
			opts:    []Option{InitializeTargets(true)},
			source:  source.Device,
			targets: []string{uninitializedTarget.Device},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// readonly so that test panics if any modifying methods are called.
			du := &readonlyFakeDiskUtil{
				du: &fakeDiskUtil{test.fakeDevices},
			}
			// nil so that test panics if asr is called.
			var r asr.ASR = nil

			c := New(du, r, test.opts...)
			if err := c.Cloneable(test.source, test.targets...); err != nil {
				t.Errorf("Cloneable returned error: %q, want: nil", err)
			}
		})
	}
}

func TestCloneable_Errors(t *testing.T) {
	source := diskutil.VolumeInfo{
		Name:           "source-name",
		UUID:           "123-source-uuid",
		MountPoint:     "/source/mount/point",
		Device:         "/dev/disk-source",
		Writable:       false,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	target := diskutil.VolumeInfo{
		Name:           "target-name",
		UUID:           "123-target-uuid",
		MountPoint:     "/target/mount/point",
		Device:         "/dev/disk-target",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	uninitializedTarget := diskutil.VolumeInfo{
		Name:           "uninitialized-target-name",
		UUID:           "123-unitialized-target-uuid",
		MountPoint:     "/uninitialized-target/mount/point",
		Device:         "/dev/disk-uninitialized-target",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	caseSensitiveAPFS := diskutil.VolumeInfo{
		Name:           "case-sensitive-apfs-name",
		UUID:           "123-case-sensitive-apfs-uuid",
		MountPoint:     "/case-sensitive-apfs/mount/point",
		Device:         "/dev/disk-case-sensitive-apfs",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "Case-sensitive APFS",
	}
	hfs := diskutil.VolumeInfo{
		Name:           "hfs-name",
		UUID:           "123-hfs-uuid",
		MountPoint:     "/hfs/mount/point",
		Device:         "/dev/disk-hfs",
		Writable:       true,
		FileSystemType: "hfs",
		FileSystem:     "HFS+",
	}
	readonly := diskutil.VolumeInfo{
		Name:           "readonly-name",
		UUID:           "123-readonly-uuid",
		MountPoint:     "/readonly/mount/point",
		Device:         "/dev/disk-readonly",
		Writable:       false,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}

	latestSnap := diskutil.Snapshot{
		Name: "latest-snap",
		UUID: "latest-snap-uuid",
	}
	commonSnap := diskutil.Snapshot{
		Name: "common-snap",
		UUID: "common-snap-uuid",
	}
	uncommonSnap := diskutil.Snapshot{
		Name: "uncommon-snap",
		UUID: "uncommon-snap-uuid",
	}

	tests := []struct {
		name        string
		fakeDevices *fakeDevices
		opts        []Option
		source      string
		targets     []string
	}{
		{
			name: "source not a device",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(target, commonSnap),
			),
			source:  "not-a-volume-uuid",
			targets: []string{target.UUID},
		},
		{
			name: "one of the targets is not a device",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(target, commonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID, "not-a-volume-uuid"},
		},
		{
			name: "same target repeated multiple times",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(target, commonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID, target.MountPoint},
		},
		{
			name: "source and target are same",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
			),
			source:  source.UUID,
			targets: []string{source.MountPoint},
		},
		{
			name: "target has no snapshots in common with source",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap),
				withFakeVolume(target, uncommonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source has no snapshots",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source),
				withFakeVolume(target, commonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target has no snapshots",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(target),
			),
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source and target have same latest snapshot",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(target, latestSnap, commonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target snapshot is ahead of latest source snapshot",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, commonSnap),
				withFakeVolume(target, latestSnap, commonSnap),
			),
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source is not an APFS volume",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(hfs),
				withFakeVolume(target, commonSnap),
			),
			source:  hfs.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target is not an APFS volume",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(hfs),
			),
			source:  source.UUID,
			targets: []string{hfs.UUID},
		},
		{
			name: "source and target have same filesystem type, but different file systems",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(caseSensitiveAPFS, latestSnap, commonSnap),
				withFakeVolume(target, commonSnap),
			),
			source:  caseSensitiveAPFS.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target not writable",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(readonly, commonSnap),
			),
			source:  source.UUID,
			targets: []string{readonly.UUID},
		},
		{
			name: "initialize - target has snapshots",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(source, latestSnap, commonSnap),
				withFakeVolume(uninitializedTarget, commonSnap),
			),
			opts:    []Option{InitializeTargets(true)},
			source:  source.Device,
			targets: []string{uninitializedTarget.Device},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// readonly so that test panics if any modifying methods are called.
			du := &readonlyFakeDiskUtil{
				du: &fakeDiskUtil{test.fakeDevices},
			}
			// nil so that test panics if asr is called.
			var r asr.ASR = nil

			c := New(du, r, test.opts...)
			if err := c.Cloneable(test.source, test.targets...); err == nil {
				t.Error("Cloneable returnd error: nil, want: non-nil")
			}
		})
	}
}

func TestClone(t *testing.T) {
	snap1 := diskutil.Snapshot{
		Name:    "common-snap",
		UUID:    "123-common-uuid",
		Created: time.Time{},
	}
	snap2 := diskutil.Snapshot{
		Name:    "latest-snap",
		UUID:    "123-latest-uuid",
		Created: snap1.Created.Add(time.Hour),
	}

	tests := []struct {
		name        string
		fakeDevices *fakeDevices
		opts        []Option
		source      string
		target      string

		wantSourceSnaps []diskutil.Snapshot
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name: "incremental clone - default options",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				),
			),
			source: "/foo/mount/point",
			target: "/bar/mount/point",
			wantSourceSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
			wantTargetSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
		},
		{
			name: "incremental clone - prune target",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				),
			),
			opts:   []Option{Prune(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
			wantSourceSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
			wantTargetSnaps: []diskutil.Snapshot{
				snap2,
			},
		},
		{
			name: "initialize clone",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
				),
			),
			opts:   []Option{InitializeTargets(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
			wantSourceSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
			wantTargetSnaps: []diskutil.Snapshot{
				snap2,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			du := &fakeDiskUtil{test.fakeDevices}
			r := &fakeASR{test.fakeDevices}
			c := New(du, r, test.opts...)
			if err := c.Clone(test.source, test.target); err != nil {
				t.Fatalf("Clone(...) returned unexpected error: %q, want: nil", err)
			}

			sourceInfo, err := du.Info(test.source)
			if err != nil {
				t.Fatal(err)
			}
			targetInfo, err := du.Info(test.target)
			if err != nil {
				t.Fatal(err)
			}
			gotSourceSnaps, err := du.ListSnapshots(sourceInfo)
			if err != nil {
				t.Fatalf("error listing snapshots: %v", err)
			}
			gotTargetSnaps, err := du.ListSnapshots(targetInfo)
			if err != nil {
				t.Fatalf("error listing snapshots: %v", err)
			}
			cmpOpts := []cmp.Option{
				cmpopts.SortSlices(func(lhs, rhs diskutil.Snapshot) bool {
					return lhs.UUID < rhs.UUID
				}),
			}
			if diff := cmp.Diff(test.wantSourceSnaps, gotSourceSnaps, cmpOpts...); diff != "" {
				t.Errorf("Clone(...) resulted in unexpected snapshots in source. -want +got:\n%s", diff)
			}
			if diff := cmp.Diff(test.wantTargetSnaps, gotTargetSnaps, cmpOpts...); diff != "" {
				t.Errorf("Clone(...) resulted in unexpected snapshots in target. -want +got:\n%s", diff)
			}
		})
	}
}

func TestClone_DryRun(t *testing.T) {
	snap1 := diskutil.Snapshot{
		Name:    "common-snap",
		UUID:    "123-common-uuid",
		Created: time.Time{},
	}
	snap2 := diskutil.Snapshot{
		Name:    "latest-snap",
		UUID:    "123-latest-uuid",
		Created: snap1.Created.Add(time.Hour),
	}

	tests := []struct {
		name        string
		fakeDevices *fakeDevices
		opts        []Option
		source      string
		target      string

		wantSourceSnaps []diskutil.Snapshot
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name: "incremental prune dryrun",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				),
			),
			opts:   []Option{Prune(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
			wantSourceSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
			wantTargetSnaps: []diskutil.Snapshot{
				snap1,
			},
		},
		{
			name: "initialize dryrun",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
				),
			),
			opts:   []Option{InitializeTargets(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
			wantSourceSnaps: []diskutil.Snapshot{
				snap2,
				snap1,
			},
			wantTargetSnaps: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a readonly fake diskutil underlying the dryrun
			// diskutil so that the test panics if any modifying
			// methods of the underlying diskutil are called.
			du := diskutil.NewDryRun(&readonlyFakeDiskUtil{
				du: &fakeDiskUtil{test.fakeDevices},
			})
			r := asr.NewDryRun()
			c := New(du, r, test.opts...)
			if err := c.Clone(test.source, test.target); err != nil {
				t.Fatalf("Clone(...) returned unexpected error: %q, want: nil", err)
			}

			sourceInfo, err := du.Info(test.source)
			if err != nil {
				t.Fatal(err)
			}
			targetInfo, err := du.Info(test.target)
			if err != nil {
				t.Fatal(err)
			}
			gotSourceSnaps, err := du.ListSnapshots(sourceInfo)
			if err != nil {
				t.Fatalf("error listing snapshots: %v", err)
			}
			gotTargetSnaps, err := du.ListSnapshots(targetInfo)
			if err != nil {
				t.Fatalf("error listing snapshots: %v", err)
			}
			cmpOpts := []cmp.Option{
				cmpopts.SortSlices(func(lhs, rhs diskutil.Snapshot) bool {
					return lhs.UUID < rhs.UUID
				}),
				cmpopts.EquateEmpty(),
			}
			if diff := cmp.Diff(test.wantSourceSnaps, gotSourceSnaps, cmpOpts...); diff != "" {
				t.Errorf("Clone(...) resulted in unexpected snapshots in source. -want +got:\n%s", diff)
			}
			if diff := cmp.Diff(test.wantTargetSnaps, gotTargetSnaps, cmpOpts...); diff != "" {
				t.Errorf("Clone(...) resulted in unexpected snapshots in target. -want +got:\n%s", diff)
			}
		})
	}
}

func TestClone_Errors(t *testing.T) {
	snap1 := diskutil.Snapshot{
		Name:    "common-snap",
		UUID:    "123-common-uuid",
		Created: time.Time{},
	}
	snap2 := diskutil.Snapshot{
		Name:    "latest-snap",
		UUID:    "123-latest-uuid",
		Created: snap1.Created.Add(time.Hour),
	}

	tests := []struct {
		name        string
		fakeDevices *fakeDevices
		opts        []Option
		source      string
		target      string
	}{
		{
			name: "source not found",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
			),
			source: "/not/a/volume",
			target: "foo/mount/point",
		},
		{
			name: "target not found",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
			),
			source: "/foo/mount/point",
			target: "/not/a/volume",
		},
		{
			name: "incremental - no snapshots",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
				),
			),
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "incremental - same latest snapshot",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap2,
					snap1,
				),
			),
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "incremental - no snapshots in common",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap2,
				),
			),
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "incremental clone - no source snaps",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				),
			),
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "initialize - no source snaps",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
				),
			),
			opts:   []Option{InitializeTargets(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "initialize - target has snaps",
			fakeDevices: newFakeDevices(t,
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				),
				withFakeVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				),
			),
			opts:   []Option{InitializeTargets(true)},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// readonly so that test panics if any modifying methods are called.
			du := &readonlyFakeDiskUtil{
				du: &fakeDiskUtil{test.fakeDevices},
			}
			// nil so that test panics if asr is called.
			var r asr.ASR = nil

			c := New(du, r, test.opts...)
			if err := c.Clone(test.source, test.target); err == nil {
				t.Fatal("Clone(...) returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
