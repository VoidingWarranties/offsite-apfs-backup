package asr

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"apfs-snapshot-diff-clone/diskutil"
)

type ASR struct {
	execCommand func(string, ...string) *exec.Cmd
	osStdout    io.Writer
}

func New() ASR {
	return ASR{
		execCommand: exec.Command,
		osStdout:    os.Stdout,
	}
}

func (a ASR) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	// TODO: ID volumes by Device instead of MountPoint.
	cmd := a.execCommand(
		"asr", "restore",
		"--source", source.MountPoint,
		"--target", target.MountPoint,
		"--toSnapshot", to.UUID,
		"--fromSnapshot", from.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = a.osStdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}
