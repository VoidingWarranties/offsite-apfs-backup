package asr

import (
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

type DryRunASR struct {}

func NewDryRun() DryRunASR {
	return DryRunASR{}
}

func (dry DryRunASR) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	return nil
}

func (dry DryRunASR) DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error {
	return nil
}
