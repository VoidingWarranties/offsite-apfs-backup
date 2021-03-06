// +build darwin

// Package diskimage provides utilities for mounting disk images in tests, as
// well as volume and snapshot metadata for the disk images in testdata/.
// Images are mounted in a temporary directory, and automatically unmounted
// during test cleanup.
package diskimage

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/plutil"
)

// Relative path from testutils to example images.
const (
	SourceImg = "testdata/source.dmg"
	TargetImg = "testdata/target.dmg"

	HFSImg               = "testdata/hfs.dmg"
	CaseSensitiveAPFSImg = "testdata/case_sensitive_apfs.dmg"
)

// Expected metadata for each example image.
var (
	SourceInfo = diskutil.VolumeInfo{
		Name:           "source",
		UUID:           "CA79DDFA-D75D-43F3-8099-3BEA2F7C1F33",
		FileSystemType: "apfs",
		FileSystem:     "APFS",
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
		Name:           "target",
		UUID:           "21CF5985-FA46-42AF-9872-52CDE74B04DE",
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}
	TargetSnaps = [...]diskutil.Snapshot{
		{
			Name:    "com.bombich.ccc.D7B2D286-3CE0-40B9-9797-EBF108ADAD30.2021-03-01-203433",
			UUID:    "A175CCCF-0C56-4A46-97FB-CA267A540C96",
			Created: time.Date(2021, 3, 1, 20, 34, 33, 0, time.UTC),
		},
	}

	HFSInfo = diskutil.VolumeInfo{
		Name:           "hfs",
		UUID:           "4F5B6053-AB58-3899-A263-F00D22575F69",
		FileSystemType: "hfs",
		FileSystem:     "HFS+",
	}
	CaseSensitiveAPFSInfo = diskutil.VolumeInfo{
		Name:           "case-sensitive-apfs",
		UUID:           "8969062E-E518-4591-9DEE-1DB5B16AB9DE",
		FileSystemType: "apfs",
		FileSystem:     "Case-sensitive APFS",
	}
)

// MountRO mounts the disk image at `path` as a readonly volume and
// returns the mount point and device node.
func MountRO(t *testing.T, path string) (mountpoint, device string) {
	t.Helper()

	mountpoint = t.TempDir()
	cmd := exec.Command(
		"hdiutil", "attach",
		// There's an odd bug in MacOS where repeatedly calling
		// `hdiutil attach` and `hdiutil detach` on an image and it's
		// volume will cause Finder to sometimes display multiple
		// Macintosh HD volumes. The -nobrowse flag seems to prevent
		// the visible symptoms of this bug, but this could also just
		// be hiding weirdness.
		"-nobrowse",
		"-plist",
		"-readonly",
		"-mountpoint", mountpoint,
		path,
	)
	stdout, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		// TODO: it would be nice if *exec.ExitError.Error() included the stderr, if any.
		t.Fatalf("failed to mount %q (%v) with stderr: %s", path, err, exitErr.Stderr)
	}
	if err != nil {
		t.Fatalf("failed to mount %q (%v)", path, err)
	}
	// Mount point may have changed by the time we cleanup (e.g. by `asr
	// restore`). Get the the device node to use during cleanup.
	device, err = parseHdiutilAttachOutput(stdout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd := exec.Command("hdiutil", "detach", "-force", device)
		err := cmd.Run()
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to unmount %q: %v: %s", device, err, exitErr.Stderr)
		}
		if err != nil {
			t.Fatalf("failed to unmount %q: %v", device, err)
		}
	})
	// t.TempDir can return a path that contains a symlink. Evaluate the
	// mount point, as `diskutil info` returns the non-symlink mount
	// points. We could return `info.MountPoint`, but then
	// diskutil_darwin_test's TestInfo wouldn't be truly testing the
	// MountPoint value.
	mountpoint, err = filepath.EvalSymlinks(mountpoint)
	if err != nil {
		t.Fatal(err)
	}
	return mountpoint, device
}

// MountRW mounts the disk image at `path` as a read/write volume
// using a shadow file. All modifications to the volume are written to
// the shadow file rather than the disk image.
func MountRW(t *testing.T, path string) (mountpoint, device string) {
	t.Helper()

	mountpoint = t.TempDir()
	shadow := filepath.Join(t.TempDir(), "shadow")
	cmd := exec.Command(
		"hdiutil", "attach",
		"-nobrowse",
		"-plist",
		"-shadow", shadow,
		"-mountpoint", mountpoint,
		path,
	)
	stdout, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		t.Fatalf("failed to mount %q (%v) with stderr: %s", path, err, exitErr.Stderr)
	}
	if err != nil {
		t.Fatalf("failed to mount %q (%v)", path, err)
	}
	device, err = parseHdiutilAttachOutput(stdout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd := exec.Command("hdiutil", "detach", "-force", device)
		err := cmd.Run()
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to unmount %q: %v: %s", device, err, exitErr.Stderr)
		}
		if err != nil {
			t.Fatalf("failed to unmount %q: %v", device, err)
		}
	})
	mountpoint, err = filepath.EvalSymlinks(mountpoint)
	if err != nil {
		t.Fatal(err)
	}
	return mountpoint, device
}

func parseHdiutilAttachOutput(stdout []byte) (device string, err error) {
	pl := plutil.New()
	var info struct {
		SystemEntities []struct {
			MountPoint string `json:"mount-point"`
			DevEntry   string `json:"dev-entry"`
		} `json:"system-entities"`
	}
	if err := pl.Unmarshal(stdout, &info); err != nil {
		return "", err
	}
	found := false
	for _, e := range info.SystemEntities {
		if e.MountPoint == "" {
			continue
		}
		if found {
			return "", errors.New("diskimage test utility only supports images with a single volume")
		}

		found = true
		device = e.DevEntry
	}
	return device, nil
}
