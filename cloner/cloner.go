// Package cloner implements cloning APFS volumes using APFS snapshot diffs.
package cloner

import (
	"errors"
	"fmt"
	"log"

	"github.com/voidingwarranties/offsite-apfs-backup/asr"
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

// Option configures Cloner.
type Option func(*Cloner)

// Prune returns an Option that, if prune is true, deletes the snapshot that
// source and target had in common iff a clone is completed successfully.
func Prune(prune bool) Option {
	return func(c *Cloner) {
		c.prune = prune
	}
}

// InitializeTargets returns an Option that, if initTargets is true, changes
// the behavior of Clone to do a destructive clone of source's latest snapshot
// to target, rather than a nondestructive incremental clone. To avoid
// accidentally deleting data, target must not have any snapshots, otherwise
// Cloneable and Clone returne errors.
func InitializeTargets(initTargets bool) Option {
	return func(c *Cloner) {
		c.initTargets = initTargets
	}
}

// TODO: document.
func DryRun(dryrun bool) Option {
	return func(c *Cloner) {
		withDiskUtil(diskutil.NewDryRun())(c)
		withASR(asr.NewDryRun())(c)
	}
}

func withDiskUtil(du du) Option {
	return func(c *Cloner) {
		c.diskutil = du
	}
}

func withASR(r restorer) Option {
	return func(c *Cloner) {
		c.asr = r
	}
}

// New returns a new Cloner with the given options.
func New(opts ...Option) Cloner {
	c := Cloner{
		diskutil: diskutil.New(),
		asr:      asr.New(),

		prune:       false,
		initTargets: false,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Cloner clones APFS volumes using APFS snapshot diffs.
type Cloner struct {
	diskutil du
	asr      restorer

	prune       bool
	initTargets bool
}

type du interface {
	Info(volume string) (diskutil.VolumeInfo, error)
	Rename(volume diskutil.VolumeInfo, name string) error
	ListSnapshots(volume diskutil.VolumeInfo) ([]diskutil.Snapshot, error)
	DeleteSnapshot(volume diskutil.VolumeInfo, snap diskutil.Snapshot) error
}

type restorer interface {
	Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error
	DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error
}

// Cloneable returns nil if source is cloneable to all targets, where cloneable
// is defined as:
//   - All source and target volumes exist, and are APFS volumes.
//   - All source and target volumes have the same file system.
//     i.e. all must be non-case-sensitive, or all must be case-sensitive.
//   - All targets are writable.
//   - All targets must have a snapshot in common with source.
//   - The snapshot in common must not be the latest snapshot in source.
func (c Cloner) Cloneable(source string, targets ...string) error {
	sourceInfo, err := c.diskutil.Info(source)
	if err != nil {
		return fmt.Errorf("invalid source volume: %v", err)
	}
	if sourceInfo.FileSystemType != "apfs" {
		return errors.New("invalid source volume: does not contain an APFS file system")
	}
	sourceSnaps, err := c.diskutil.ListSnapshots(sourceInfo)
	if err != nil {
		return fmt.Errorf("error listing snapshots of source: %v", err)
	}
	if len(sourceSnaps) == 0 {
		return errors.New("invalid source: no snapshots to clone")
	}

	if len(targets) == 0 {
		return errors.New("no targets")
	}
	// Map of target UUIDs to the target argument.
	targetUUIDs := make(map[string]string)
	for _, t := range targets {
		targetInfo, err := c.diskutil.Info(t)
		if err != nil {
			return fmt.Errorf("invalid target volume: %v", err)
		}
		if sourceInfo.UUID == targetInfo.UUID {
			return errors.New("source and target must be different volumes")
		}
		if duplicate := targetUUIDs[targetInfo.UUID]; duplicate != "" {
			return fmt.Errorf("invalid target: %q is the same as %q", t, duplicate)
		}
		targetUUIDs[targetInfo.UUID] = t
		if targetInfo.FileSystemType != "apfs" {
			return errors.New("invalid target volume: does not contain an APFS file system")
		}
		// `asr restore` will restore the target volume to the same file system
		// as source. To be safe, error here to prevent changing the file
		// system without the user knowing.
		if sourceInfo.FileSystem != targetInfo.FileSystem {
			return fmt.Errorf("invalid source + target combination: source is formatted as %s, but target is formatted as %s", sourceInfo.FileSystem, targetInfo.FileSystem)
		}
		if !targetInfo.Writable {
			return errors.New("invalid target volume: volume not writable")
		}

		targetSnaps, err := c.diskutil.ListSnapshots(targetInfo)
		if err != nil {
			return fmt.Errorf("error listing snapshots of target: %v", err)
		}
		if err := c.cloneable(sourceSnaps, targetSnaps); err != nil {
			return err
		}
	}
	return nil
}

func (c Cloner) cloneable(sourceSnaps, targetSnaps []diskutil.Snapshot) error {
	if !c.initTargets {
		_, err := latestCommonSnapshot(sourceSnaps, targetSnaps)
		return err
	}
	if len(targetSnaps) > 0 {
		return errors.New("invalid target: target has snapshots - erase the disk before using initialize")
	}
	return nil
}

// Clone the latest snapshot in source to target, from the most recent common
// snapshot present in both source and target.
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

	if c.initTargets {
		if err := c.destructiveClone(sourceInfo, targetInfo); err != nil {
			return err
		}
	} else {
		if err := c.clone(sourceInfo, targetInfo); err != nil {
			return err
		}
	}
	// ASR renames the volume to source's name after a restore. Change it
	// back.
	if err := c.diskutil.Rename(targetInfo, targetInfo.Name); err != nil {
		return fmt.Errorf("error renaming volume to original name: %v", err)
	}
	return nil
}

func (c Cloner) clone(source, target diskutil.VolumeInfo) error {
	sourceSnaps, err := c.diskutil.ListSnapshots(source)
	if err != nil {
		return fmt.Errorf("error listing snapshots of source: %v", err)
	}
	targetSnaps, err := c.diskutil.ListSnapshots(target)
	if err != nil {
		return fmt.Errorf("error listing snapshots of target: %v", err)
	}
	commonSnap, err := latestCommonSnapshot(sourceSnaps, targetSnaps)
	if err != nil {
		return fmt.Errorf("error finding latest snapshot in common between source and target: %v", err)
	}
	log.Printf("Found snapshot in common: %s", commonSnap)

	// TODO: document that this relies on the snapshots being in the right order.
	latestSourceSnap := sourceSnaps[0]
	log.Printf("Restoring to latest snapshot in source, %s, from common snapshot", latestSourceSnap)
	if err := c.asr.Restore(source, target, latestSourceSnap, commonSnap); err != nil {
		return fmt.Errorf("error restoring: %v", err)
	}

	if c.prune {
		log.Print("Pruning common snapshot from target...")
		if err := c.diskutil.DeleteSnapshot(target, commonSnap); err != nil {
			return fmt.Errorf("error deleting snapshot %q from target", commonSnap)
		}
	}
	return nil
}

func (c Cloner) destructiveClone(source, target diskutil.VolumeInfo) error {
	sourceSnaps, err := c.diskutil.ListSnapshots(source)
	if err != nil {
		return fmt.Errorf("error listing snapshots of source: %v", err)
	}
	if len(sourceSnaps) == 0 {
		return errors.New("source does not contain any snapshots")
	}
	targetSnaps, err := c.diskutil.ListSnapshots(target)
	if err != nil {
		return fmt.Errorf("error listing snapshots of target: %v", err)
	}
	if len(targetSnaps) > 0 {
		return errors.New("aborting because target contains snapshots that would be erased")
	}
	// TODO: document that this relies on the snapshots being in the right order.
	latestSourceSnap := sourceSnaps[0]
	log.Printf("Restoring to latest snapshot in source, %s", latestSourceSnap)
	if err := c.asr.DestructiveRestore(source, target, latestSourceSnap); err != nil {
		return fmt.Errorf("error restoring: %v", err)
	}
	return nil
}

// TODO: document that this relies on the snapshots being in the right order.
func latestCommonSnapshot(source, target []diskutil.Snapshot) (diskutil.Snapshot, error) {
	commonSourceI, commonTargetI, exists := latestCommonSnapshotIndices(source, target)
	if !exists {
		return diskutil.Snapshot{}, errors.New("source and target have no snapshots in common")
	}
	if commonSourceI == 0 && commonTargetI == 0 {
		return diskutil.Snapshot{}, errors.New("both source and target have the same latest snapshot")
	}
	// TODO: is this logic correct? Shouldn't it be `commonSourceI < commonTargetI`?
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
