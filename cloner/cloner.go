package cloner

import (
	"errors"
	"fmt"
	"log"

	"apfs-snapshot-diff-clone/asr"
	"apfs-snapshot-diff-clone/diskutil"
)

type Option func(*Cloner)

func Prune(prune bool) Option {
	return func(c *Cloner) {
		c.prune = prune
	}
}

func New(opts ...Option) Cloner {
	c := Cloner {
		diskutil: diskutil.DiskUtil{},
		asr:      asr.ASR{},
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type Cloner struct {
	diskutil du
	asr      restorer

	prune bool
}

type du interface {
	Info(volume string) (diskutil.VolumeInfo, error)
	Rename(volume string, name string) error
	ListSnapshots(volume string) ([]diskutil.Snapshot, error)
	DeleteSnapshot(volume string, snap diskutil.Snapshot) error
}

type restorer interface {
	Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error
}

func (c Cloner) Clone(source, target string) error {
	log.Printf("Cloning %q to %q...", source, target)

	sourceInfo, err := c.diskutil.Info(source)
	if err != nil {
		return fmt.Errorf("error getting volume info of source %q: %v", source, err)
	}
	targetInfo, err := c.diskutil.Info(target)
	if err != nil {
		return fmt.Errorf("error getting volume info of target %q: %v", target, err)
	}

	sourceSnaps, err := c.diskutil.ListSnapshots(sourceInfo.UUID)
	if err != nil {
		return fmt.Errorf("error listing snapshots of source %q: %v", source, err)
	}
	targetSnaps, err := c.diskutil.ListSnapshots(targetInfo.UUID)
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
	if err := c.asr.Restore(sourceInfo, targetInfo, latestSourceSnap, commonSnap); err != nil {
		return fmt.Errorf("error restoring: %v", err)
	}
	if err := c.diskutil.Rename(targetInfo.UUID, targetInfo.Name); err != nil {
		return fmt.Errorf("error renaming volume to original name: %v", err)
	}

	if c.prune {
		log.Print("Pruning common snapshot from target...")
		if err := c.diskutil.DeleteSnapshot(targetInfo.UUID, commonSnap); err != nil {
			return fmt.Errorf("error deleting snapshot %q from target", commonSnap)
		}
	}
	return nil
}

// TODO: document that this relies on the snapshots being in the right order.
func latestCommonSnapshot(source, target []diskutil.Snapshot) (diskutil.Snapshot, error) {
	commonSourceI, commonTargetI, exists := latestCommonSnapshotIndices(source, target)
	if !exists {
		return diskutil.Snapshot{}, errors.New("no snapshot in common")
	}
	if commonSourceI == 0 && commonTargetI == 0 {
		return diskutil.Snapshot{}, errors.New("both source and target have the same latest snapshot")
	}
	if commonSourceI == 0 {
		return diskutil.Snapshot{}, errors.New("target has a snapshot ahead of source")
	}
	return source[commonSourceI], nil
}

func latestCommonSnapshotIndices(source, target []diskutil.Snapshot) (sourceIndex, targetIndex int, exists bool) {
	for targetIndex, ts := range target {
		for sourceIndex, ss := range source {
			if ss.UUID == ts.UUID {
				return sourceIndex, targetIndex, true
			}
		}
	}
	return 0, 0, false
}
