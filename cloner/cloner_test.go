package cloner_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"apfs-snapshot-diff-clone/cloner"
	"apfs-snapshot-diff-clone/diskutil"
)

type fakeDevices struct {
	// Map of volume UUID to volume info.
	volumes map[string]diskutil.VolumeInfo
	// Map of volume UUID to snapshots.
	snapshots map[string][]diskutil.Snapshot
}

func (d *fakeDevices) Volume(id string) (diskutil.VolumeInfo, error) {
	for _, info := range d.volumes {
		if info.UUID == id || info.Name == id || info.MountPoint == id {
			return info, nil
		}
	}
	return diskutil.VolumeInfo{}, errors.New("volume does not exist")
}

func (d *fakeDevices) AddVolume(volume diskutil.VolumeInfo, snapshots ...diskutil.Snapshot) error {
	if _, exists := d.volumes[volume.UUID]; exists {
		return errors.New("volume already exists")
	}
	d.volumes[volume.UUID] = volume
	d.snapshots[volume.UUID] = snapshots
	return nil
}

func (d *fakeDevices) RemoveVolume(uuid string) error {
	if _, exists := d.volumes[uuid]; !exists {
		return errors.New("volume does not exist")
	}
	delete(d.volumes, uuid)
	delete(d.snapshots, uuid)
	return nil
}

func (d *fakeDevices) Snapshots(volumeUUID string) ([]diskutil.Snapshot, error) {
	snaps, exists := d.snapshots[volumeUUID]
	if !exists {
		return nil, errors.New("volume not found")
	}
	return snaps, nil
}

func (d *fakeDevices) Snapshot(volumeUUID, snapshotUUID string) (diskutil.Snapshot, error) {
	snaps, err := d.Snapshots(volumeUUID)
	if err != nil {
		return diskutil.Snapshot{}, err
	}
	for _, s := range snaps {
		if s.UUID == snapshotUUID {
			return s, nil
		}
	}
	return diskutil.Snapshot{}, errors.New("snapshot not found")
}

func (d *fakeDevices) AddSnapshot(volumeUUID string, snapshot diskutil.Snapshot) error {
	if _, exists := d.volumes[volumeUUID]; !exists {
		return errors.New("volume not found")
	}
	for _, s := range d.snapshots[volumeUUID] {
		if s.UUID == snapshot.UUID {
			return errors.New("snapshot already exists")
		}
	}
	d.snapshots[volumeUUID] = append(d.snapshots[volumeUUID], snapshot)
	return nil
}

func (d *fakeDevices) DeleteSnapshot(volumeUUID, snapshotUUID string) error {
	snaps, err := d.Snapshots(volumeUUID)
	if err != nil {
		return err
	}
	snapI := -1
	for i, s := range snaps {
		if s.UUID == snapshotUUID {
			snapI = i
			break
		}
	}
	if snapI < 0 {
		return errors.New("snapshot not found")
	}
	d.snapshots[volumeUUID] = append(snaps[:snapI], snaps[snapI+1:]...)
	return nil
}

type fakeDiskUtil struct {
	devices *fakeDevices
}

func (du *fakeDiskUtil) Info(volume string) (diskutil.VolumeInfo, error) {
	return du.devices.Volume(volume)
}

func (du *fakeDiskUtil) Rename(volume string, name string) error {
	info, err := du.devices.Volume(volume)
	if err != nil {
		return err
	}
	snaps, err := du.devices.Snapshots(info.UUID)
	if err != nil {
		return err
	}

	if err := du.devices.RemoveVolume(info.UUID); err != nil {
		return err
	}
	info.Name = name
	return du.devices.AddVolume(info, snaps...)
}

func (du *fakeDiskUtil) ListSnapshots(volume string) ([]diskutil.Snapshot, error) {
	info, err := du.devices.Volume(volume)
	if err != nil {
		return nil, err
	}
	return du.devices.Snapshots(info.UUID)
}

func (du *fakeDiskUtil) DeleteSnapshot(volume string, snap diskutil.Snapshot) error {
	info, err := du.devices.Volume(volume)
	if err != nil {
		return err
	}
	return du.devices.DeleteSnapshot(info.UUID, snap.UUID)
}

type fakeASR struct {
	devices *fakeDevices
}

func (asr *fakeASR) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	// Validate source and target volumes exist.
	if _, err := asr.devices.Volume(source.UUID); err != nil {
		return err
	}
	if _, err := asr.devices.Volume(target.UUID); err != nil {
		return err
	}

	// Validate that `from` exists in both source and target.
	if _, err := asr.devices.Snapshot(source.UUID, from.UUID); err != nil {
		return err
	}
	if _, err := asr.devices.Snapshot(target.UUID, from.UUID); err != nil {
		return err
	}
	// Valdiate that `to` exists in source.
	_, err := asr.devices.Snapshot(source.UUID, to.UUID)
	if err != nil {
		return err
	}
	// Add `to` snapshot to target.
	asr.devices.AddSnapshot(target.UUID, to)
	// Rename target to source name.
	snaps, err := asr.devices.Snapshots(target.UUID)
	if err != nil {
		return err
	}
	if err := asr.devices.RemoveVolume(target.UUID); err != nil {
		return err
	}
	target.Name = source.Name
	return asr.devices.AddVolume(target, snaps...)
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

	tests := []struct{
		name   string
		setup  func(*testing.T, *fakeDevices)
		opts   []cloner.Option
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
			opts: []cloner.Option{cloner.Prune(true)},
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
			opts := append([]cloner.Option{
				cloner.WithDiskUtil(du),
				cloner.WithASR(&fakeASR{&d}),
			}, test.opts...)
			c := cloner.New(opts...)
			if err := c.Clone(test.source, test.target); err != nil {
				t.Fatalf("Clone(...) returned unexpected error: %q, want: nil", err)
			}

			gotSourceSnaps, err := du.ListSnapshots(test.source)
			if err != nil {
				t.Fatalf("error listing snapshots: %v", err)
			}
			gotTargetSnaps, err := du.ListSnapshots(test.target)
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

type DiskUtil interface {
	Info(volume string) (diskutil.VolumeInfo, error)
	Rename(volume string, name string) error
	ListSnapshots(volume string) ([]diskutil.Snapshot, error)
	DeleteSnapshot(volume string, snap diskutil.Snapshot) error
}

type readonlyFakeDiskUtil struct {
	du *fakeDiskUtil

	DiskUtil
}

func (du *readonlyFakeDiskUtil) Info(volume string) (diskutil.VolumeInfo, error) {
	return du.du.Info(volume)
}

func (du *readonlyFakeDiskUtil) ListSnapshots(volume string) ([]diskutil.Snapshot, error) {
	return du.du.ListSnapshots(volume)
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
			opts := []cloner.Option{
				// readonly so that test panics if any modifying methods are called.
				cloner.WithDiskUtil(&readonlyFakeDiskUtil{
					du: &fakeDiskUtil{&d},
				}),
				// nil so that test panics if asr is called.
				cloner.WithASR(nil),
			}
			c := cloner.New(opts...)
			if err := c.Clone(test.source, test.target); err == nil {
				t.Fatal("Clone(...) returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
