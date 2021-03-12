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
type ASR interface {
	Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error
	DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error
}

type asr struct {
	config
}

// config contains fields shared between asr and dryRunASR.
type config struct {
	execCommand func(string, ...string) *exec.Cmd
	stdout      io.Writer
}

// Option configures the behavior of ASR.
type Option func(*config)

// Stdout returns an Option that sets the stdout to the given io.Writer.
func Stdout(w io.Writer) Option {
	return func(conf *config) {
		conf.stdout = w
	}
}

func withExecCmd(f func(string, ...string) *exec.Cmd) Option {
	return func(conf *config) {
		conf.execCommand = f
	}
}

// New returns a new ASR.
func New(opts ...Option) ASR {
	conf := config{
		execCommand: exec.Command,
		stdout:      os.Stdout,
	}
	for _, opt := range opts {
		opt(&conf)
	}
	return asr{
		config: conf,
	}
}

// Restore the target volume to the source volume's `to` snapshot, from the
// target volume's `from` snapshot. Both to and from must exist in source. From
// must also exist in target.
func (a asr) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	cmd := a.execCommand(
		"asr", "restore",
		"--source", source.Device,
		"--target", target.Device,
		"--toSnapshot", to.UUID,
		"--fromSnapshot", from.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = a.stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}

// DestructiveRestore restores the target volume to the source volume's `to`
// snapshot. `to` must exist in source. target's previous data and snapshots
// will be lost. Use with caution!
func (a asr) DestructiveRestore(source, target diskutil.VolumeInfo, to diskutil.Snapshot) error {
	cmd := a.execCommand(
		"asr", "restore",
		"--source", source.Device,
		"--target", target.Device,
		"--toSnapshot", to.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = a.stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}
