package cloner

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"apfs-snapshot-diff-clone/diskutil"
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
		name    string
		setup   func(*testing.T, *fakeDevices)
		source  string
		targets []string
	}{
		{
			name: "single target",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap2, commonSnap1); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target1, commonSnap1); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.MountPoint,
			targets: []string{target1.MountPoint},
		},
		{
			name: "multiple targets",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap2, commonSnap1); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target1, commonSnap1); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target2, commonSnap2); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.Device,
			targets: []string{target1.Device, target2.UUID},
		},
		{
			name: "Case-sensitive APFS",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(caseSensitiveSource, latestSnap, commonSnap1); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(caseSensitiveTarget1, commonSnap1); err != nil {
					t.Fatal(err)
				}
			},
			source:  caseSensitiveSource.Device,
			targets: []string{caseSensitiveTarget1.MountPoint},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &fakeDevices{
				volumes:   make(map[string]diskutil.VolumeInfo),
				snapshots: make(map[string][]diskutil.Snapshot),
			}
			test.setup(t, d)
			du := &fakeDiskUtil{d}
			c := New(withDiskUtil(&readonlyFakeDiskUtil{du: du}))
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
		name    string
		setup   func(*testing.T, *fakeDevices)
		source  string
		targets []string
	}{
		{
			name: "source not a device",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  "not-a-volume-uuid",
			targets: []string{target.UUID},
		},
		{
			name: "one of the targets is not a device",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID, "not-a-volume-uuid"},
		},
		{
			name: "same target repeated multiple times",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID, target.MountPoint},
		},
		{
			name: "source and target are same",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{source.MountPoint},
		},
		{
			name: "target has no snapshots in common with source",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, uncommonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source has no snapshots",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target has no snapshots",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source and target have same latest snapshot",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target snapshot is ahead of latest source snapshot",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "source is not an APFS volume",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(hfs); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  hfs.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target is not an APFS volume",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(hfs); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{hfs.UUID},
		},
		{
			name: "source and target have same filesystem type, but different file systems",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(caseSensitiveAPFS, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(target, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  caseSensitiveAPFS.UUID,
			targets: []string{target.UUID},
		},
		{
			name: "target not writable",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(source, latestSnap, commonSnap); err != nil {
					t.Fatal(err)
				}
				if err := d.AddVolume(readonly, commonSnap); err != nil {
					t.Fatal(err)
				}
			},
			source:  source.UUID,
			targets: []string{readonly.UUID},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &fakeDevices{
				volumes:   make(map[string]diskutil.VolumeInfo),
				snapshots: make(map[string][]diskutil.Snapshot),
			}
			test.setup(t, d)
			du := &fakeDiskUtil{d}
			c := New(withDiskUtil(&readonlyFakeDiskUtil{du: du}))
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
		name   string
		setup  func(*testing.T, *fakeDevices)
		opts   []Option
		source string
		target string

		wantSourceSnaps []diskutil.Snapshot
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name: "incremental clone - default options",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
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
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := fakeDevices{
				volumes:   make(map[string]diskutil.VolumeInfo),
				snapshots: make(map[string][]diskutil.Snapshot),
			}
			du := &fakeDiskUtil{&d}
			test.setup(t, &d)
			opts := append([]Option{
				withDiskUtil(du),
				withASR(&fakeASR{&d}),
			}, test.opts...)
			c := New(opts...)
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
		name   string
		setup  func(*testing.T, *fakeDevices)
		source string
		target string
	}{
		{
			name: "source not found",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
			source: "/not/a/volume",
			target: "foo/mount/point",
		},
		{
			name: "target not found",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
			source: "/foo/mount/point",
			target: "/not/a/volume",
		},
		{
			name: "no snapshots",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "same latest snapshot",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap2,
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
		{
			name: "no snapshots in common",
			setup: func(t *testing.T, d *fakeDevices) {
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "foo-name",
						UUID:       "123-foo-uuid",
						MountPoint: "/foo/mount/point",
					},
					snap1,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
				if err := d.AddVolume(
					diskutil.VolumeInfo{
						Name:       "bar-name",
						UUID:       "123-bar-uuid",
						MountPoint: "/bar/mount/point",
					},
					snap2,
				); err != nil {
					t.Fatalf("error adding volume: %v", err)
				}
			},
			source: "/foo/mount/point",
			target: "/bar/mount/point",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := fakeDevices{
				volumes:   make(map[string]diskutil.VolumeInfo),
				snapshots: make(map[string][]diskutil.Snapshot),
			}
			test.setup(t, &d)
			opts := []Option{
				// readonly so that test panics if any modifying methods are called.
				withDiskUtil(&readonlyFakeDiskUtil{
					du: &fakeDiskUtil{&d},
				}),
				// nil so that test panics if asr is called.
				withASR(nil),
			}
			c := New(opts...)
			if err := c.Clone(test.source, test.target); err == nil {
				t.Fatal("Clone(...) returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
