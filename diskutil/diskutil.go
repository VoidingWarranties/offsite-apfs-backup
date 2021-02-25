package diskutil

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"time"

	"apfs-snapshot-diff-clone/snapshot"
)

type DiskUtil struct {}

func (d DiskUtil) Rename(volume string, name string) error {
	cmd := exec.Command("diskutil", "rename", volume, name)
	cmd.Stdout = os.Stdout
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`%s` failed (%v) with stderr: %s", cmd, err, stderr)
	}
	return nil
}

type VolumeInfo struct {
	UUID       string
	Name       string
	MountPoint string
}

func (d DiskUtil) Info(volume string) (VolumeInfo, error) {
	cmd := exec.Command("diskutil", "info", volume)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return VolumeInfo{}, fmt.Errorf("error creating stdout pipe: %v", err)
	}
	stdout := io.TeeReader(stdoutPipe, os.Stdout)
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Start(); err != nil {
		return VolumeInfo{}, fmt.Errorf("`%s` failed to start: %v", cmd, err)
	}
	output, err := io.ReadAll(stdout)
	if err != nil {
		return VolumeInfo{}, fmt.Errorf("error reading stdout: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		return VolumeInfo{}, fmt.Errorf("`%s` failed (%v) with stderr: %s", cmd, err, stderr)
	}

	return parseInfo(output)
}

func parseInfo(info []byte) (VolumeInfo, error) {
	re := regexp.MustCompile(
		// Match and capture volume mount point.
		`Volume Name: +(.+)` +

		// Match (but do not capture) any line. This makes the regex
		// pattern less brittle, as it will still match even if the
		// order of the fields after mount point are changed.
		`(?:\n.*)*` +

		// Match and capture volume mount point.
		`Mount Point: +(.+)` +

		`(?:\n.*)*` +

		// Match and capture volume UUID.
		`Volume UUID: +([A-Z0-9-]+)`)
	matches := re.FindSubmatch(info)
	if len(matches) == 0 {
		return VolumeInfo{}, fmt.Errorf("pattern %q did not match snapshot text:\n%q", re, info)
	}
	return VolumeInfo{
		Name:       string(matches[1]),
		MountPoint: string(matches[2]),
		UUID:       string(matches[3]),
	}, nil
}

func (d DiskUtil) ListSnapshots(volume string) ([]snapshot.Snapshot, error) {
	cmd := exec.Command("diskutil", "apfs", "listsnapshots", volume)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %v", err)
	}
	stdout := io.TeeReader(stdoutPipe, os.Stdout)
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	log.Printf("Running command:\n%s", cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("`%s` failed to start: %v", cmd, err)
	}
	output, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("error reading stdout: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("`%s` failed (%v) with stderr: %s", cmd, err, stderr)
	}

	snapshots, err := parseSnapshotList(output)
	if err != nil {
		return nil, err
	}
	isSorted := sort.SliceIsSorted(snapshots, func(i, ii int) bool {
		lhs := snapshots[i]
		rhs := snapshots[ii]
		return lhs.Created.Before(rhs.Created)
	})
	if !isSorted {
		return nil, fmt.Errorf("`%s` returned snapshots in an unexpected order", cmd)
	}
	for i, ii := 0, len(snapshots)-1; i < ii; i, ii = i+1, ii-1 {
		snapshots[i], snapshots[ii] = snapshots[ii], snapshots[i]
	}
	return snapshots, nil
}

func parseSnapshotList(stdout []byte) ([]snapshot.Snapshot, error) {
	headerRegex := regexp.MustCompile(`Snapshots? for [a-z0-9]+ \((\d+) found\)`)
	headerMatches := headerRegex.FindSubmatch(stdout)
	if len(headerMatches) == 0 {
		return nil, nil
	}
	numSnapshots, err := strconv.Atoi(string(headerMatches[1]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse header (%q): %v", headerMatches[0], err)
	}

	listRegex := regexp.MustCompile(`\+-- .*(?:\n\|? .*)+`)
	listMatches := listRegex.FindAll(stdout, -1)
	if len(listMatches) != numSnapshots {
		return nil, fmt.Errorf("found %d snapshots in list, but header claimed %d snapshots", len(listMatches), numSnapshots)
	}
	var snapshots []snapshot.Snapshot
	for _, match := range listMatches {
		snap, err := parseSnapshot(match)
		if err != nil {
			return nil, fmt.Errorf("failed to parse snapshot: %v", err)
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

func parseSnapshot(snap []byte) (snapshot.Snapshot, error) {
	snapshotRegex := regexp.MustCompile(
		// Match and capture UUID.
		`\+-- ([A-Z0-9-]+)` +

		// Match (but do not capture) any line. This makes the regex
		// pattern less brittle, as it will still match even if the
		// order of the fields after UUID are changed.
		`(?:\n.*)*` +

		// Match and capture Name.
		`\|? +Name: +([a-zA-Z0-9.-]+)`)
	snapshotMatches := snapshotRegex.FindSubmatch(snap)
	if len(snapshotMatches) == 0 {
		return snapshot.Snapshot{}, fmt.Errorf("pattern %q did not match snapshot text:\n%q", snapshotRegex, snap)
	}
	uuid := snapshotMatches[1]
	name := snapshotMatches[2]

	timeRegex := regexp.MustCompile(`\d{4}-\d{2}-\d{2}-\d{6}`)
	timeMatch := timeRegex.Find(name)
	if len(timeMatch) == 0 {
		return snapshot.Snapshot{}, fmt.Errorf("snapshot name (%q) does not contain a timestamp of the form yyyy-mm-dd-hhmmss", name)
	}
	created, err := time.Parse("2006-01-02-150405", string(timeMatch))
	if err != nil {
		return snapshot.Snapshot{}, fmt.Errorf("failed to parse time substring (%q) from snapshot name", timeMatch)
	}

	return snapshot.Snapshot{
		Name:    string(name),
		UUID:    string(uuid),
		Created: created,
	}, nil
}
