package asr

import (
	"fmt"
	"os"

	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

type dryRun struct {
	config
}

func NewDryRun(opts ...Option) ASR {
	conf := config{
		stdout: os.Stdout,
	}
	for _, opt := range opts {
		opt(&conf)
	}
	return dryRun{
		config: conf,
	}
}

func (dry dryRun) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	fmt.Fprintln(dry.stdout, "Restore completed successfully.")
	return nil
}

func (dry dryRun) DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error {
	fmt.Fprintln(dry.stdout, "Restore completed successfully.")
	return nil
}
