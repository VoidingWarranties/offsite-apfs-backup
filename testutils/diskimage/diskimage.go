// +build darwin

// Package diskimage provides utilities for mounting disk images in tests, as
// well as volume and snapshot metadata for the disk images in testdata/.
// Images are mounted in a temporary directory, and automatically unmounted
// during test cleanup.
//
// All VolumeInfo and Snapshot metadata are constructed independently, and
// therefore suitable for testing the diskutil package.
package diskimage

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
	"github.com/voidingwarranties/offsite-apfs-backup/plutil"
)

// Testdata disk images:
const (
	// SourceImg is an example source volume that has multiple snapshots.
	SourceImg DiskImage = "testdata/source.dmg"
	// TargetImg is an example target volume that has a snapshot in common
	// with SourceImg.
	TargetImg DiskImage = "testdata/target.dmg"
	// UninitializedTargetImg is an example target volume that has no
	// snapshots.
	UninitializedTargetImg DiskImage = "testdata/uninitialized_target.dmg"

	// Non-APFS file systems.
	CaseSensitiveAPFSImg DiskImage = "testdata/case_sensitive_apfs.dmg"
	HFSImg               DiskImage = "testdata/hfs.dmg"
)

// Expected metadata for each example image.
var (
	infos = map[DiskImage]diskutil.VolumeInfo{
		SourceImg: diskutil.VolumeInfo{
			Name:           "source",
			UUID:           "CA79DDFA-D75D-43F3-8099-3BEA2F7C1F33",
			FileSystemType: "apfs",
			FileSystem:     "APFS",
		},
		TargetImg: diskutil.VolumeInfo{
			Name:           "target",
			UUID:           "21CF5985-FA46-42AF-9872-52CDE74B04DE",
			FileSystemType: "apfs",
			FileSystem:     "APFS",
		},
		UninitializedTargetImg: diskutil.VolumeInfo{
			Name:           "uninitialized-target",
			UUID:           "E4317DE2-9CA1-4044-8B63-01FC118EB880",
			FileSystemType: "apfs",
			FileSystem:     "APFS",
		},
		HFSImg: diskutil.VolumeInfo{
			Name:           "hfs",
			UUID:           "4F5B6053-AB58-3899-A263-F00D22575F69",
			FileSystemType: "hfs",
			FileSystem:     "HFS+",
		},
		CaseSensitiveAPFSImg: diskutil.VolumeInfo{
			Name:           "case-sensitive-apfs",
			UUID:           "8969062E-E518-4591-9DEE-1DB5B16AB9DE",
			FileSystemType: "apfs",
			FileSystem:     "Case-sensitive APFS",
		},
	}
	snapshots = map[DiskImage][]diskutil.Snapshot{
		SourceImg: []diskutil.Snapshot{
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
		},
		TargetImg: []diskutil.Snapshot{
			{
				Name:    "com.bombich.ccc.D7B2D286-3CE0-40B9-9797-EBF108ADAD30.2021-03-01-203433",
				UUID:    "A175CCCF-0C56-4A46-97FB-CA267A540C96",
				Created: time.Date(2021, 3, 1, 20, 34, 33, 0, time.UTC),
			},
		},
	}
)

// Mounter mounts testdata disk images by constructing their path from the
// relative path to the diskimage package, Relpath.
type Mounter struct {
	// Relpath should be the relative path to this package from the test
	// being executed. For example, a test in the cloner package would set
	// Relpath to "../testutils/diskimage".
	Relpath string
}

// MountRO is similar to the top-level function, MountRO, but it a)
// automatically constructs the path to the given image using the relative
// path, Mounter.Relpath, to the diskimage package, and b) returns the full
// VolumeInfo, rather than just mount point and device node.
func (m Mounter) MountRO(t *testing.T, img DiskImage) diskutil.VolumeInfo {
	t.Helper()
	return img.mountRO(t, m.Relpath)
}

// MountRW is similar to the top-level function, MountRW, but it a)
// automatically constructs the path to the given image using the relative
// path, Mounter.Relpath, to the diskimage package, and b) returns the full
// VolumeInfo, rather than just mount point and device node.
func (m Mounter) MountRW(t *testing.T, img DiskImage) diskutil.VolumeInfo {
	t.Helper()
	return img.mountRW(t, m.Relpath)
}

// DiskImage is a testdata disk image. The value is a relative path from this
// package to the disk image.
type DiskImage string

func (img DiskImage) mountRO(t *testing.T, relpath string) diskutil.VolumeInfo {
	t.Helper()
	info, exists := infos[img]
	if !exists {
		t.Fatalf("unknown disk image %q", img)
	}
	path := filepath.Join(relpath, string(img))
	info.MountPoint, info.Device = MountRO(t, path)
	info.Writable = false
	return info
}

func (img DiskImage) mountRW(t *testing.T, relpath string) diskutil.VolumeInfo {
	t.Helper()
	info, exists := infos[img]
	if !exists {
		t.Fatalf("unknown disk image %q", img)
	}
	path := filepath.Join(relpath, string(img))
	info.MountPoint, info.Device = MountRW(t, path)
	info.Writable = true
	return info
}

// UUID returns the volume UUID of the disk image.
func (img DiskImage) UUID(t *testing.T) string {
	t.Helper()
	info, exists := infos[img]
	if !exists {
		t.Fatalf("unknown disk image %q", img)
	}
	return info.UUID
}

// Snapshots returns the APFS snapshots of the disk image.
func (img DiskImage) Snapshots(t *testing.T) []diskutil.Snapshot {
	snaps, exists := snapshots[img]
	if !exists {
		t.Fatalf("no snapshot information for disk image %q", img)
	}
	return snaps
}

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
		if err := detach(device); err != nil {
			t.Fatal(err)
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
		if err := detach(device); err != nil {
			t.Fatal(err)
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

// detach the device, retrying up to 2 additional times (with a small
// increasing delay) if there are errors. The retry is necessary because
// sometimes `hdiutil detach` complains that the device is busy and cannot
// detach. Not the best solution, but better than flaky tests.
func detach(device string) error {
	const maxAttempts = 3
	const initialDelay = time.Second
	var err error
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(time.Duration(i) * initialDelay)
		cmd := exec.Command("hdiutil", "detach", "-force", device)
		err = cmd.Run()
		if err == nil {
			return nil
		}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("failed to unmount %q after %d tries: %v: %s", device, maxAttempts, err, exitErr.Stderr)
	}
	return fmt.Errorf("failed to unmount %q after %d tries: %v", device, maxAttempts, err)
}
