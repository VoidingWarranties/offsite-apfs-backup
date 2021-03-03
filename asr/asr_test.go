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
	a.execCommand = fakecmd.FakeCommand(fakecmd.Options{
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
