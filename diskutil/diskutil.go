// Package diskutil implements reading metadata of, and some modifications to,
// local volumes using MacOS's diskutil.
package diskutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"time"

	"github.com/voidingwarranties/offsite-apfs-backup/plutil"
)

// DiskUtil reads and modifies metadata of local volumes.
type DiskUtil struct {
	execCommand func(string, ...string) *exec.Cmd
	pl          plutil.PLUtil
}

type option func(*DiskUtil)

func withExecCommand(f func(string, ...string) *exec.Cmd) option {
	return func(du *DiskUtil) {
		du.execCommand = f
	}
}

func withPLUtil(pl plutil.PLUtil) option {
	return func(du *DiskUtil) {
		du.pl = pl
	}
}

// New returns a new DiskUtil.
func New(opts ...option) DiskUtil {
	du := DiskUtil{
		execCommand: exec.Command,
		pl:          plutil.New(),
	}
	for _, opt := range opts {
		opt(&du)
	}
	return du
}

// VolumeInfo describes a local volume. Typically used to identify a volume by
// UUID, mount point, or device node.
type VolumeInfo struct {
	UUID string `json:"VolumeUUID"`
	Name string `json:"VolumeName"`
	// e.g. /Volumes/name
	MountPoint string `json:"MountPoint"`
	// e.g. /dev/disk1s2
	Device   string `json:"DeviceNode"`
	Writable bool   `json:"WritableVolume"`
	// e.g. apfs, hfs.
	FileSystemType string `json:"FilesystemType"`
	// e.g. APFS, Case-sensitive APFS.
	FileSystem string `json:"FilesystemName"`
}

// Info returns the VolumeInfo of volume. Volume may be a volume name, UUID,
// mount point, or device node.
func (du DiskUtil) Info(volume string) (VolumeInfo, error) {
	cmd := du.execCommand("diskutil", "info", "-plist", volume)
	var info VolumeInfo
	err := du.runAndDecodePlist(cmd, &info)
	return info, err
}

// Rename volume to name.
func (du DiskUtil) Rename(volume VolumeInfo, name string) error {
	cmd := du.execCommand("diskutil", "rename", volume.Device, name)
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr)
	}
	return nil
}

// Snapshot describes an APFS volume's snapshot.
type Snapshot struct {
	Name    string    `json:"SnapshotName"`
	UUID    string    `json:"SnapshotUUID"`
	Created time.Time `json:"-"`
}

func (s Snapshot) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.UUID)
}

// ListSnapshots returns a volume's APFS snapshots. The snapshots are returned
// in the order of most recent snapshot first. Note that this is the reverse of
// the order returned by 'diskutil apfs listsnapshots`.
func (du DiskUtil) ListSnapshots(volume VolumeInfo) ([]Snapshot, error) {
	cmd := du.execCommand("diskutil", "apfs", "listsnapshots", "-plist", volume.Device)
	var snapshotList struct {
		Snapshots []Snapshot `json:"Snapshots"`
	}
	err := du.runAndDecodePlist(cmd, &snapshotList)
	if err != nil {
		return nil, err
	}

	// TODO: document why we sort here.
	var snapshots []Snapshot
	for _, snap := range snapshotList.Snapshots {
		created, err := parseTimeFromSnapshotName(snap.Name)
		if err != nil {
			return nil, err
		}
		snap.Created = created
		snapshots = append(snapshots, snap)
	}
	isSorted := sort.SliceIsSorted(snapshots, func(i, ii int) bool {
		lhs := snapshots[i]
		rhs := snapshots[ii]
		return lhs.Created.Before(rhs.Created)
	})
	if !isSorted {
		return nil, validationError{
			fmt.Errorf("`%s` returned snapshots in an unexpected order", cmd),
		}
	}
	for i, ii := 0, len(snapshots)-1; i < ii; i, ii = i+1, ii-1 {
		snapshots[i], snapshots[ii] = snapshots[ii], snapshots[i]
	}
	return snapshots, nil
}

type validationError struct {
	error
}

func parseTimeFromSnapshotName(name string) (time.Time, error) {
	timeRegex := regexp.MustCompile(`\d{4}-\d{2}-\d{2}-\d{6}`)
	timeMatch := timeRegex.FindString(name)
	if len(timeMatch) == 0 {
		return time.Time{}, validationError{
			fmt.Errorf("snapshot name (%q) does not contain a timestamp of the form yyyy-mm-dd-hhmmss", name),
		}
	}
	created, err := time.Parse("2006-01-02-150405", string(timeMatch))
	if err != nil {
		return time.Time{}, validationError{
			fmt.Errorf("failed to parse time substring (%q) from snapshot name", timeMatch),
		}
	}
	return created, nil
}

// DeleteSnapshot removes the given snapshot from the given volume.
func (du DiskUtil) DeleteSnapshot(volume VolumeInfo, snap Snapshot) error {
	cmd := du.execCommand("diskutil", "apfs", "deletesnapshot", volume.Device, "-uuid", snap.UUID)
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr)
	}
	return nil
}

func (du DiskUtil) runAndDecodePlist(cmd *exec.Cmd, v interface{}) error {
	stdout, err := cmd.Output()
	if err != nil {
		var errMsg plistErrorMessage
		if perr := du.pl.Unmarshal(stdout, &errMsg); perr == nil && errMsg.IsError {
			plistErr := plistError{
				message: errMsg.Message,
				cmdErr:  err,
			}
			return fmt.Errorf("`%s` failed %w", cmd, plistErr)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, exitErr.Stderr)
		}
		return fmt.Errorf("`%s` failed (%w)", cmd, err)
	}
	if err := du.pl.Unmarshal(stdout, v); err != nil {
		return fmt.Errorf("error parsing plist: %w", err)
	}
	return nil
}

type plistError struct {
	message string
	cmdErr  error
}

func (err plistError) Error() string {
	return fmt.Sprintf("(%s) with error: %s", err.cmdErr, err.message)
}

func (err plistError) Unwrap() error {
	return err.cmdErr
}

type plistErrorMessage struct {
	IsError bool   `json:"Error"`
	Message string `json:"ErrorMessage"`
}
