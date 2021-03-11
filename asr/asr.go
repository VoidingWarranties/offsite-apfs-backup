// Package asr implements restoring volumes to APFS snapshots using MacOS's
// Apple Software Restore utility.
package asr

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

// ASR restores a target volume to a source volume's APFS snapshot.
type ASR struct {
	execCommand func(string, ...string) *exec.Cmd
	osStdout    io.Writer
}

// New returns a new ASR.
func New() ASR {
	return ASR{
		execCommand: exec.Command,
		osStdout:    os.Stdout,
	}
}

// Restore the target volume to the source volume's `to` snapshot, from the
// target volume's `from` snapshot. Both to and from must exist in source. From
// must also exist in target.
func (a ASR) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	cmd := a.execCommand(
		"asr", "restore",
		"--source", source.Device,
		"--target", target.Device,
		"--toSnapshot", to.UUID,
		"--fromSnapshot", from.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = a.osStdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	fmt.Fprintf(a.osStdout, "Running command:\n%s\n", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}

// DestructiveRestore restores the target volume to the source volume's `to`
// snapshot. `to` must exist in source. target's previous data and snapshots
// will be lost. Use with caution!
func (a ASR) DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error {
	cmd := a.execCommand(
		"asr", "restore",
		"--source", source.Device,
		"--target", target.Device,
		"--toSnapshot", to.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = a.osStdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	fmt.Fprintf(a.osStdout, "Running command:\n%s\n", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}
