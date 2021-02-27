package cloner

import (
	"errors"
	"testing"
	"time"

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
