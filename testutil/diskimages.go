// +build darwin

package testutil

import (
	"bytes"
	"testing"
	"time"
	"os/exec"
	"path/filepath"

	"apfs-snapshot-diff-clone/diskutil"
)

const (
	SourceImg        = "testdata/source.dmg"
	TargetImg        = "testdata/target.dmg"
)

var (
	SourceInfo = diskutil.VolumeInfo{
		Name: "source",
		UUID: "CA79DDFA-D75D-43F3-8099-3BEA2F7C1F33",
	}
	SourceSnaps = [...]diskutil.Snapshot{
		{
			Name:    "com.bombich.ccc.6AE4815C-1F9A-4D5E-86E1-19078BE01958.2021-03-01-203509",
			UUID:    "D1ABE254-5B1B-4FDF-8DB3-1B4B4B825E39",
			Created: time.Date(2021, 3, 1, 20, 35, 9, 0, time.UTC),
		},
		{
			Name:    "com.bombich.ccc.D7B2D286-3CE0-40B9-9797-EBF108ADAD30.2021-03-01-203433",
			UUID:    "A175CCCF-0C56-4A46-97FB-CA267A540C96",
			Created: time.Date(2021, 3, 1, 20, 34, 33, 0, time.UTC),
		},
	}

	TargetInfo = diskutil.VolumeInfo{
		Name: "target",
		UUID: "CD9BFC7A-63A9-480C-9918-E692CC147A60",
	}
	TargetSnaps = [...]diskutil.Snapshot{
		{
			Name:    "com.bombich.ccc.D7B2D286-3CE0-40B9-9797-EBF108ADAD30.2021-03-01-203433",
			UUID:    "A175CCCF-0C56-4A46-97FB-CA267A540C96",
			Created: time.Date(2021, 3, 1, 20, 34, 33, 0, time.UTC),
		},
	}
)

// MountRO mounts the disk image at `path` as a readonly volume and
// returns the mount point.
func MountRO(t *testing.T, path string) (mountpoint string) {
	t.Helper()

	mountpoint = t.TempDir()
	cmd := exec.Command(
		"hdiutil", "attach",
		"-readonly",
		"-mountpoint", mountpoint,
		path,
	)
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	err := cmd.Run()
	// TODO: it would be nice if *exec.ExitError.Error() included the stderr, if any.
	if err != nil {
		t.Fatalf("failed to mount %q (%v): %s", path, err, stderr)
	}
	t.Cleanup(func() {
		// TODO: force detach?
		cmd := exec.Command("hdiutil", "detach", mountpoint)
		err := cmd.Run()
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to unmount %q: %v: %s", mountpoint, err, exitErr.Stderr)
		}
		if err != nil {
			t.Fatalf("failed to unmount %q: %v", mountpoint, err)
		}
	})
	// t.TempDir can return a path that contains a symlink. Evaluate the
	// mount point, as `diskutil info` returns the non-symlink mount
	// points.
	mountpoint, err = filepath.EvalSymlinks(mountpoint)
	if err != nil {
		t.Fatalf("error evaluating symlinks: %v", err)
	}
	return mountpoint
}

// MountRW mounts the disk image at `path` as a read/write volume
// using a shadow file. All modifications to the volume are written to
// the shadow file rather than the disk image.
func MountRW(t *testing.T, path string) (mountpoint string) {
	t.Helper()

	mountpoint = t.TempDir()
	shadow := filepath.Join(t.TempDir(), "shadow")
	cmd := exec.Command(
		"hdiutil", "attach",
		"-shadow", shadow,
		"-mountpoint", mountpoint,
		path,
	)
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	err := cmd.Run()
	// TODO: it would be nice if *exec.ExitError.Error() included the stderr, if any.
	if err != nil {
		t.Fatalf("failed to mount %q (%v): %s", path, err, stderr)
	}
	t.Cleanup(func() {
		// TODO: force detach?
		cmd := exec.Command("hdiutil", "detach", mountpoint)
		err := cmd.Run()
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to unmount %q: %v: %s", mountpoint, err, exitErr.Stderr)
		}
		if err != nil {
			t.Fatalf("failed to unmount %q: %v", mountpoint, err)
		}
	})
	mountpoint, err = filepath.EvalSymlinks(mountpoint)
	if err != nil {
		t.Fatalf("error evaluating symlinks: %v", err)
	}
	return mountpoint
}
