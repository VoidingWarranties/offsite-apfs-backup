package asr

import (
	"io"
	"os"
	"testing"

	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/testutils/fakecmd"
)

func TestHelperProcess(t *testing.T) {
	fakecmd.HelperProcess(t)
}

func TestRestore_WritesOutputToStdout(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()

	a := New()
	a.execCommand = fakecmd.FakeCommand(t, fakecmd.Options{
		Stdouts: map[string]string{
			"asr": "want stdout",
		},
	})
	a.osStdout = pw

	dummyVolume := diskutil.VolumeInfo{}
	dummySnap := diskutil.Snapshot{}
	err = a.Restore(dummyVolume, dummyVolume, dummySnap, dummySnap)
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Restore returned unexpected error: %v, want: nil", err)
	}

	pw.Close()
	gotStdout, err := io.ReadAll(pr)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotStdout) != "want stdout" {
		t.Errorf("Restore wrote unexpected value to stdout: %q, want: %q", gotStdout, "want stdout")
	}
}

// Test that Restore:
//   1. IDs volumes by MountPoint.
//   2. IDs snapshots by UUID.
func TestRestore_CmdArgs(t *testing.T) {
	source := diskutil.VolumeInfo{
		UUID:       "source-volume-uuid",
		Name:       "source-volume-name",
		MountPoint: "/source/mount/point",
		Device:     "/dev/source-device",
	}
	target := diskutil.VolumeInfo{
		UUID:       "target-volume-uuid",
		Name:       "target-volume-name",
		MountPoint: "/target/mount/point",
		Device:     "/dev/target-device",
	}
	to := diskutil.Snapshot{
		UUID: "to-snapshot-uuid",
		Name: "to-snapshot-name",
	}
	from := diskutil.Snapshot{
		UUID: "from-snapshot-uuid",
		Name: "from-snapshot-name",
	}

	a := New()
	a.execCommand = fakecmd.FakeCommand(t, fakecmd.Options{
		WantArgs: map[string]map[string]bool{
			"asr": map[string]bool{
				source.MountPoint: true,
				target.MountPoint: true,
				to.UUID:           true,
				from.UUID:         true,
			},
		},
	})
	err := a.Restore(source, target, to, from)
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Restore returned unexpected error: %v, want: nil", err)
	}
}
