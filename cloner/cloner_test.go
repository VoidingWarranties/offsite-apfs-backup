package cloner

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"apfs-snapshot-diff-clone/diskutil"
)

func TestCloneableSource(t *testing.T) {
	d := &fakeDevices{
		volumes:   make(map[string]diskutil.VolumeInfo),
		snapshots: make(map[string][]diskutil.Snapshot),
	}
	d.AddVolume(diskutil.VolumeInfo{
		Name:       "foo-name",
		UUID:       "123-foo-uuid",
		MountPoint: "/foo/mount/point",
		FileSystem: "apfs",
	})
	du := &fakeDiskUtil{d}
	c := New(withDiskUtil(&readonlyFakeDiskUtil{du: du}))
	if err := c.CloneableSource("/foo/mount/point"); err != nil {
		t.Errorf("CloneableSource returned error: %q, want: nil", err)
	}
}

func TestCloneableSource_Errors(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T, *fakeDevices)
		source string
	}{
		{
			name:   "not a device",
			setup:  func(*testing.T, *fakeDevices) {},
			source: "not-a-volume-uuid",
		},
		{
			name: "not not an APFS volume",
			setup: func(t *testing.T, d *fakeDevices) {
				err := d.AddVolume(diskutil.VolumeInfo{
					Name:       "HFS Volume",
					UUID:       "hfs-volume-uuid",
					MountPoint: "/hfs/volume/mount/point",
					Device:     "/dev/hfs-device",
					FileSystem: "hfs",
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			source: "/hfs/volume/mount/point",
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
			if err := c.CloneableSource(test.source); err == nil {
				t.Error("CloneableSource returnd error: nil, want: non-nil")
			}
		})
	}
}

func TestCloneableTarget(t *testing.T) {
	d := &fakeDevices{
		volumes:   make(map[string]diskutil.VolumeInfo),
		snapshots: make(map[string][]diskutil.Snapshot),
	}
	d.AddVolume(diskutil.VolumeInfo{
		Name:       "foo-name",
		UUID:       "123-foo-uuid",
		MountPoint: "/foo/mount/point",
		FileSystem: "apfs",
		Writable:   true,
	})
	du := &fakeDiskUtil{d}
	c := New(withDiskUtil(&readonlyFakeDiskUtil{du: du}))
	if err := c.CloneableTarget("/foo/mount/point"); err != nil {
		t.Errorf("CloneableTarget returned error: %q, want: nil", err)
	}
}

func TestCloneableTarget_Errors(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T, *fakeDevices)
		source string
	}{
		{
			name:   "not a device",
			setup:  func(*testing.T, *fakeDevices) {},
			source: "not-a-volume-uuid",
		},
		{
			name: "not not an APFS volume",
			setup: func(t *testing.T, d *fakeDevices) {
				err := d.AddVolume(diskutil.VolumeInfo{
					Name:       "HFS Volume",
					UUID:       "hfs-volume-uuid",
					MountPoint: "/hfs/volume/mount/point",
					Device:     "/dev/hfs-device",
					FileSystem: "hfs",
					Writable:   true,
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			source: "/hfs/volume/mount/point",
		},
		{
			name: "not writable",
			setup: func(t *testing.T, d *fakeDevices) {
				err := d.AddVolume(diskutil.VolumeInfo{
					Name:       "Readonly Volume",
					UUID:       "readonly-volume-uuid",
					MountPoint: "/readonly/volume/mount/point",
					Device:     "/dev/readonly-device",
					FileSystem: "apfs",
					Writable:   false,
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			source: "/readonly/volume/mount/point",
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
			if err := c.CloneableTarget(test.source); err == nil {
				t.Error("CloneableTarget returnd error: nil, want: non-nil")
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
