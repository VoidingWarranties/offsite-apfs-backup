package diskutil

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"time"

	"apfs-snapshot-diff-clone/plutil"
)

type DiskUtil struct {
	execCommand func(string, ...string) *exec.Cmd
	pl          plutil.PLUtil
}

type Option func(*DiskUtil)

func WithExecCommand(f func(string, ...string) *exec.Cmd) Option {
	return func(du *DiskUtil) {
		du.execCommand = f
	}
}

func WithPLUtil(pl plutil.PLUtil) Option {
	return func(du *DiskUtil) {
		du.pl = pl
	}
}

func New(opts ...Option) DiskUtil {
	du := DiskUtil{}
	defaultOpts := []Option{
		WithExecCommand(exec.Command),
		WithPLUtil(plutil.New(plutil.WithExecCommand(exec.Command))),
	}
	opts = append(defaultOpts, opts...)
	for _, opt := range opts {
		opt(&du)
	}
	return du
}

func (du DiskUtil) Rename(volume string, name string) error {
	cmd := du.execCommand("diskutil", "rename", volume, name)
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr)
	}
	return nil
}

type VolumeInfo struct {
	UUID       string `json:"VolumeUUID"`
	Name       string `json:"VolumeName"`
	MountPoint string `json:"MountPoint"`
	Device     string `json:"DeviceNode"`
}

func (du DiskUtil) Info(volume string) (VolumeInfo, error) {
	cmd := du.execCommand("diskutil", "info", "-plist", volume)
	var info VolumeInfo
	err := du.runAndDecodePlist(cmd, &info)
	return info, err
}

type Snapshot struct {
	Name    string    `json:"SnapshotName"`
	UUID    string    `json:"SnapshotUUID"`
	Created time.Time `json:"-"`
}

func (s Snapshot) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.UUID)
}

func (du DiskUtil) ListSnapshots(volume string) ([]Snapshot, error) {
	cmd := du.execCommand("diskutil", "apfs", "listsnapshots", "-plist", volume)
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

func (du DiskUtil) DeleteSnapshot(volume string, snap Snapshot) error {
	cmd := du.execCommand("diskutil", "apfs", "deletesnapshot", volume, "-uuid", snap.UUID)
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, stderr)
	}
	return nil
}

func (du DiskUtil) runAndDecodePlist(cmd *exec.Cmd, v interface{}) error {
	log.Printf("Running command:\n%s", cmd)
	stdout, err := cmd.Output()
	r := bytes.NewReader(stdout)
	if err != nil {
		var errMsg plistErrorMessage
		if perr := du.pl.DecodePlist(r, &errMsg); perr == nil && errMsg.IsError {
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
	if err := du.pl.DecodePlist(r, v); err != nil {
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
