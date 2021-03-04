package cloner

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"apfs-snapshot-diff-clone/diskutil"
)

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

	tests := []struct{
		name   string
		setup  func(*testing.T, *fakeDevices)
		opts   []Option
		source string
		target string

		wantSourceSnaps []diskutil.Snapshot
		wantTargetSnaps []diskutil.Snapshot
	}{
		{
			name:   "incremental clone - default options",
			setup:  func(t *testing.T, d *fakeDevices) {
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
			name:   "incremental clone - prune target",
			setup:  func(t *testing.T, d *fakeDevices) {
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
			opts: []Option{Prune(true)},
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

	tests := []struct{
		name string
		setup  func(*testing.T, *fakeDevices)
		source string
		target string
	}{
		{
			name: "source not found",
			setup:  func(t *testing.T, d *fakeDevices) {
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
			setup:  func(t *testing.T, d *fakeDevices) {
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
			setup:  func(t *testing.T, d *fakeDevices) {
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
			setup:  func(t *testing.T, d *fakeDevices) {
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
			setup:  func(t *testing.T, d *fakeDevices) {
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
