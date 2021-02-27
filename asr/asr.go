package asr

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"

	"apfs-snapshot-diff-clone/diskutil"
)

type ASR struct {}

func (a ASR) Restore(source, target diskutil.VolumeInfo, to, from diskutil.Snapshot) error {
	cmd := exec.Command(
		"asr", "restore",
		"--source", source.MountPoint,
		"--target", target.MountPoint,
		"--toSnapshot", to.UUID,
		"--fromSnapshot", from.UUID,
		"--erase", "--noprompt")
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%v) with stderr: %s", cmd, err, stderr.String())
	}
	return nil
}
