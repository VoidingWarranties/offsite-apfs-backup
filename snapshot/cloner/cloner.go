package cloner

import (
	"errors"
	"fmt"
	"log"

	"apfs-snapshot-diff-clone/asr"
	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/snapshot"
)

func New() Cloner {
	du := diskutil.DiskUtil{}
	return Cloner {
		snapshotLister:   du,
		volumeInfoer:     du,
		volumeRenamer:    du,
		snapshotRestorer: asr.ASR{},
	}
}

type Cloner struct {
	snapshotLister   snapshotLister
	volumeInfoer     volumeInfoer
	volumeRenamer    volumeRenamer
	snapshotRestorer snapshotRestorer
}

type snapshotLister interface {
	ListSnapshots(volume string) ([]snapshot.Snapshot, error)
}

type volumeInfoer interface {
	Info(volume string) (diskutil.VolumeInfo, error)
}

type volumeRenamer interface {
	Rename(volume string, name string) error
}

type snapshotRestorer interface {
	Restore(source, target string, to, from snapshot.Snapshot) error
}

func (c Cloner) Clone(source, target string) error {
	log.Printf("Cloning %q to %q...", source, target)

	sourceInfo, err := c.volumeInfoer.Info(source)
	if err != nil {
		return fmt.Errorf("error getting volume info of source %q: %v", source, err)
	}
	targetInfo, err := c.volumeInfoer.Info(target)
	if err != nil {
		return fmt.Errorf("error getting volume info of target %q: %v", target, err)
	}

	sourceSnaps, err := c.snapshotLister.ListSnapshots(sourceInfo.UUID)
	if err != nil {
		return fmt.Errorf("error listing snapshots of source %q: %v", source, err)
	}
	targetSnaps, err := c.snapshotLister.ListSnapshots(targetInfo.UUID)
	if err != nil {
		return fmt.Errorf("error listing snapshots of target %q: %v", target, err)
	}
	commonSnap, err := latestCommonSnapshot(sourceSnaps, targetSnaps)
	if err != nil {
		return fmt.Errorf("error finding latest snapshot in common between source %q and target %q: %v", source, target, err)
	}
	log.Printf("Found snapshot in common: %s", commonSnap)

	// TODO: document that this relies on the snapshots being in the right order.
	latestSourceSnap := sourceSnaps[0]
	log.Printf("Restoring to latest snapshot in source, %s, from common snapshot", latestSourceSnap)
	if err := c.snapshotRestorer.Restore(sourceInfo.MountPoint, targetInfo.MountPoint, latestSourceSnap, commonSnap); err != nil {
		return fmt.Errorf("error restoring: %v", err)
	}

	return c.volumeRenamer.Rename(targetInfo.UUID, targetInfo.Name)
}

// TODO: document that this relies on the snapshots being in the right order.
func latestCommonSnapshot(source, target []snapshot.Snapshot) (snapshot.Snapshot, error) {
	commonSourceI, commonTargetI, exists := latestCommonSnapshotIndices(source, target)
	if !exists {
		return snapshot.Snapshot{}, errors.New("no snapshot in common")
	}
	if commonSourceI == 0 && commonTargetI == 0 {
		return snapshot.Snapshot{}, errors.New("both source and target have the same latest snapshot")
	}
	if commonSourceI == 0 {
		return snapshot.Snapshot{}, errors.New("target has a snapshot ahead of source")
	}
	return source[commonSourceI], nil
}

func latestCommonSnapshotIndices(source, target []snapshot.Snapshot) (sourceIndex, targetIndex int, exists bool) {
	for targetIndex, ts := range target {
		for sourceIndex, ss := range source {
			if ss.UUID == ts.UUID {
				return sourceIndex, targetIndex, true
			}
		}
	}
	return 0, 0, false
}
