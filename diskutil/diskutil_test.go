package diskutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func fakeCommand(stdouts, stderrs, wantStdins map[string]string, exitFails map[string]bool) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("GO_HELPER_PROCESS_STDOUT=%s", stdouts[name]),
			fmt.Sprintf("GO_HELPER_PROCESS_STDERR=%s", stderrs[name]),
		)
		if exitFail := exitFails[name]; exitFail {
			cmd.Env = append(cmd.Env, "GO_HELPER_PROCESS_EXIT_FAIL=1")
		}
		if wantStdin, exists := wantStdins[name]; exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("GO_HELPER_PROCESS_WANT_STDIN=%s", wantStdin))
		}
		return cmd
	}
}

// Magic number to indicate that the error is caused by an error in the
// TestHelperProcess function, rather than an intended "fake" error. Can be any
// number, as long as the number is not the same as an exit code chosen by a
// test case.
const helperProcessErrExitCode = 42

func TestHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if _, exists := os.LookupEnv("GO_HELPER_PROCESS_EXIT_FAIL"); exists {
		defer os.Exit(1)
	} else {
		defer os.Exit(0)
	}

	// Order is important here.
	// This order (output stdout, validate stdin, output stderr) is chosen
	// as it behaves correctly regardless of how the command was executed
	// (i.e. cmd.Run() vs cmd.Start() + process stdout + cmd.Wait()).
	//
	// For example, consider the decodePlist function.
	//   - If stdin is validated before outputing stdout and stdin is
	//     incorrect, decodePlist will return a JSON decode error because
	//     nothing was written to stdout.
	//   - If stdin is validated after outputing stdout and stderr, and
	//     stdin is incorrect, the test case's fake stderr will be included
	//     in the error message.
	fmt.Fprint(os.Stdout, os.Getenv("GO_HELPER_PROCESS_STDOUT"))
	gotStdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from STDIN: %v", err)
		os.Exit(helperProcessErrExitCode)
	}
	wantStdin := os.Getenv("GO_HELPER_PROCESS_WANT_STDIN")
	if wantStdin != string(gotStdin) {
		fmt.Fprintf(os.Stderr, "Received unexpected STDIN. want: %q, got: %q", wantStdin, string(gotStdin))
		os.Exit(helperProcessErrExitCode)
	}
	fmt.Fprint(os.Stderr, os.Getenv("GO_HELPER_PROCESS_STDERR"))
}

// asHelperProcessErr returns a non-nil error if any error in err's chain is an
// *os.ExitError with exit code equal to the magic number 42. Use it to
// determine if a (potentially wrapped) error from running a exec.Cmd was
// caused by an unintended error in the TestHelperProcess func.
//
// If err represents a helper process error and *os.ExitError.Stderr is not
// empty, an error containing just the stderr is returned. Otherwise, the
// original error is returned.
func asHelperProcessErr(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == helperProcessErrExitCode {
		if len(exitErr.Stderr) != 0 {
			return errors.New(string(exitErr.Stderr))
		}
		return err
	}
	return nil
}

func TestDecodePlist(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	type simpleStruct struct {
		Val string `json:"val"`
	}

	tests := []struct {
		name      string
		stdout    string
		stderr    string
		r         io.Reader
		want      simpleStruct
		wantStdin string
	}{
		{
			name:   "decodes JSON stdout",
			stdout: `{"val": "example"}`,
			want: simpleStruct{
				Val: "example",
			},
		},
		{
			name: "ignores unknown fields",
			stdout: `{"val": "example", "unknown": "foo"}`,
			want: simpleStruct{
				Val: "example",
			},
		},
		{
			name:   "ignores stderr (if exit code 0)",
			stdout: "{}",
			stderr: "example non-fatal error",
			want:   simpleStruct{},
		},
		{
			name:      "passes r to stdin",
			stdout:    "{}",
			r:         bytes.NewBufferString("example stdin"),
			want:      simpleStruct{},
			wantStdin: "example stdin",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdouts := map[string]string{"plutil": test.stdout}
			stderrs := map[string]string{"plutil": test.stderr}
			wantStdins := map[string]string{"plutil": test.wantStdin}
			execCommand = fakeCommand(stdouts, stderrs, wantStdins, nil)

			got := simpleStruct{}
			err := decodePlist(test.r, &got)
			if err := asHelperProcessErr(err); err != nil {
				// TODO: it would be nice if we could `t.Fatal(string(exitErr.Stderr))` instead, but exec.Cmd.Wait() does not populate this field. I don't see why it couldn't. Add it!
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("decodePlist returned unexpected error: %q, want: nil", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("decodePlist resulted in unexpected value. -want +got:\n%s", diff)
			}
		})
	}
}

func TestDecodePlist_Errors(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	type simpleStruct struct {
		Val string `json:"val"`
	}

	var exitErr *exec.ExitError
	var syntaxErr *json.SyntaxError

	tests := []struct{
		name      string
		stdout    string
		stderr    string
		exitFail  bool
		wantErrAs interface{}
	}{
		{
			name:      "non-0 exit code",
			stdout:    "{}",
			stderr:    "example stderr foobar",
			exitFail:  true,
			wantErrAs: &exitErr,
		},
		{
			name:      "invalid JSON returns decode error",
			stdout:    "not-json",
			wantErrAs: &syntaxErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdouts := map[string]string{"plutil": test.stdout}
			stderrs := map[string]string{"plutil": test.stderr}
			exitFails := map[string]bool{"plutil": test.exitFail}
			execCommand = fakeCommand(stdouts, stderrs, nil, exitFails)

			err := decodePlist(nil, &simpleStruct{})
			if err := asHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("decodePlist returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}

func TestInfo(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	tests := []struct{
		name       string
		stdouts    map[string]string
		stderrs    map[string]string
		wantStdins map[string]string
		want       VolumeInfo
	}{
		{
			name: "success",
			stdouts: map[string]string{
				"diskutil": "<plist diskutil output>",
				"plutil": `{
					"VolumeUUID": "foo-uuid",
					"VolumeName": "foo-name",
					"MountPoint": "/foo/mount/point"
				}`,
			},
			wantStdins: map[string]string{
				"plutil": "<plist diskutil output>",
			},
			want: VolumeInfo{
				UUID:       "foo-uuid",
				Name:       "foo-name",
				MountPoint: "/foo/mount/point",
			},
		},
		{
			name: "ignores stderr (if exit code 0)",
			stdouts: map[string]string{
				"diskutil": "<plist diskutil output>",
				"plutil": `{
					"VolumeUUID": "bar-uuid",
					"VolumeName": "bar-name",
					"MountPoint": "/bar/mount/point"
				}`,
			},
			stderrs: map[string]string{
				"diskutil": "diskutil-stderr",
				"plutil": "plutil-stderr",
			},
			wantStdins: map[string]string{
				"plutil": "<plist diskutil output>",
			},
			want: VolumeInfo{
				UUID:       "bar-uuid",
				Name:       "bar-name",
				MountPoint: "/bar/mount/point",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCommand = fakeCommand(test.stdouts, test.stderrs, test.wantStdins, nil)

			du := DiskUtil{}
			got, err := du.Info("/example/volume")
			if err := asHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("Info returned unexpected error: %q, want: nil", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Info returned unexpected VolumeInfo. -want +got:\n%s", diff)
			}
		})
	}
}

func TestInfo_Errors(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	var exitErr *exec.ExitError
	var plistErr plistError

	tests := []struct{
		name       string
		stdouts    map[string]string
		stderrs    map[string]string
		wantStdins map[string]string
		exitFails  map[string]bool
		wantErrAs interface{}
	}{
		{
			name: "diskutil exec errors",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil":   "{}",
			},
			stderrs:    map[string]string{"diskutil": "stderr"},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			exitFails:  map[string]bool{"diskutil": true},
			wantErrAs:  &exitErr,
		},
		{
			name: "plutil exec errors",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil":   "{}",
			},
			stderrs:    map[string]string{"plutil": "stderr"},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			exitFails:  map[string]bool{"plutil": true},
			wantErrAs:  &exitErr,
		},
		{
			name: "diskutil plist error output - returns plist error",
			stdouts: map[string]string{
				"diskutil": "diskutil-plist-err",
				"plutil":   `{"Error": true, "ErrorMessage": "diskutil err message"}`,
			},
			wantStdins: map[string]string{"plutil": "diskutil-plist-err"},
			exitFails:  map[string]bool{"diskutil": true},
			wantErrAs:  &plistErr,
		},
		{
			name: "diskutil plist error output - plist error wraps exec.ExitError",
			stdouts: map[string]string{
				"diskutil": "diskutil-plist-err",
				"plutil":   `{"Error": true, "ErrorMessage": "diskutil err message"}`,
			},
			wantStdins: map[string]string{"plutil": "diskutil-plist-err"},
			exitFails:  map[string]bool{"diskutil": true},
			wantErrAs:  &exitErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCommand = fakeCommand(test.stdouts, test.stderrs, test.wantStdins, test.exitFails)

			du := DiskUtil{}
			_, err := du.Info("/example/volume")
			if err := asHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("Info returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}

func TestListSnapshots(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	tests := []struct{
		name       string
		stdouts    map[string]string
		stderrs    map[string]string
		wantStdins map[string]string
		want       []Snapshot
	}{
		{
			name: "multiple snapshots",
			stdouts: map[string]string{
				"diskutil": "<plist diskutil output>",
				"plutil": `{
					"Snapshots": [
						{
							"SnapshotName": "foo-snapshot-name-2021-03-02-012345",
							"SnapshotUUID": "foo-snapshot-uuid"
						},
						{
							"SnapshotName": "bar.snapshot.name.2021-04-03-012345",
							"SnapshotUUID": "bar-snapshot-uuid"
						},
						{
							"SnapshotName": "baz_2021-05-04-012345_snapshot_name",
							"SnapshotUUID": "baz-snapshot-uuid"
						}
					]
				}`,
			},
			wantStdins: map[string]string{
				"plutil": "<plist diskutil output>",
			},
			want: []Snapshot{
				{
					Name:    "baz_2021-05-04-012345_snapshot_name",
					UUID:    "baz-snapshot-uuid",
					Created: time.Date(2021, 5, 4, 1, 23, 45, 0, time.UTC),
				},
				{
					Name:    "bar.snapshot.name.2021-04-03-012345",
					UUID:    "bar-snapshot-uuid",
					Created: time.Date(2021, 4, 3, 1, 23, 45, 0, time.UTC),
				},
				{
					Name:    "foo-snapshot-name-2021-03-02-012345",
					UUID:    "foo-snapshot-uuid",
					Created: time.Date(2021, 3, 2, 1, 23, 45, 0, time.UTC),
				},
			},
		},
		{
			name: "no snapshots",
			stdouts: map[string]string{
				"diskutil": "<plist diskutil output>",
				"plutil": `{
					"Snapshots": []
				}`,
			},
			wantStdins: map[string]string{
				"plutil": "<plist diskutil output>",
			},
			want: []Snapshot{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCommand = fakeCommand(test.stdouts, test.stderrs, test.wantStdins, nil)

			du := DiskUtil{}
			got, err := du.ListSnapshots("/example/volume")
			if err := asHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("ListSnapshots returned unexpected error: %q, want: nil", err)
			}
			cmpOpts := []cmp.Option{
				cmpopts.EquateEmpty(),
			}
			if diff := cmp.Diff(test.want, got, cmpOpts...); diff != "" {
				t.Errorf("ListSnapshots returned unexpected []Snapshot. -want +got:\n%s", diff)
			}
		})
	}
}

func TestListSnapshots_Errors(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })

	var exitErr *exec.ExitError
	var validationErr validationError

	tests := []struct{
		name       string
		stdouts    map[string]string
		stderrs    map[string]string
		wantStdins map[string]string
		exitFails  map[string]bool
		wantErrAs interface{}
	}{
		{
			name: "snapshots in unexpected order",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil": `{
					"Snapshots": [
						{
							"SnapshotName": "bar-snapshot-name-2021-04-03-012345",
							"SnapshotUUID": "bar-snapshot-uuid"
						},
						{
							"SnapshotName": "foo-snapshot-name-2021-03-02-012345",
							"SnapshotUUID": "foo-snapshot-uuid"
						}
					]
				}`,
			},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			wantErrAs: &validationErr,
		},
		{
			name: "no time in name",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil": `{
					"Snapshots": [
						{
							"SnapshotName": "foo-snapshot-name",
							"SnapshotUUID": "foo-snapshot-uuid"
						}
					]
				}`,
			},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			wantErrAs: &validationErr,
		},
		{
			name: "invalid time in name",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil": `{
					"Snapshots": [
						{
							"SnapshotName": "foo-snapshot-name-2021-13-01-000000",
							"SnapshotUUID": "foo-snapshot-uuid"
						}
					]
				}`,
			},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			wantErrAs: &validationErr,
		},
		{
			name: "diskutil exec errors",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil":   "{}",
			},
			stderrs:    map[string]string{"diskutil": "stderr"},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			exitFails:  map[string]bool{"diskutil": true},
			wantErrAs:  &exitErr,
		},
		{
			name: "plutil exec errors",
			stdouts: map[string]string{
				"diskutil": "foo-stdout",
				"plutil":   "{}",
			},
			stderrs:    map[string]string{"plutil": "stderr"},
			wantStdins: map[string]string{"plutil": "foo-stdout"},
			exitFails:  map[string]bool{"plutil": true},
			wantErrAs:  &exitErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCommand = fakeCommand(test.stdouts, test.stderrs, test.wantStdins, test.exitFails)

			du := DiskUtil{}
			_, err := du.ListSnapshots("/example/volume")
			if err := asHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("ListSnapshots returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}

func TestRename(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })
	execCommand = fakeCommand(nil, nil, nil, nil)

	du := DiskUtil{}
	err := du.Rename("/example/volume", "newname")
	if err := asHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Rename returned unexpected error: %v, want: nil", err)
	}
}

func TestRename_Errors(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })
	stderrs := map[string]string{"diskutil": "example stderr"}
	exitFails := map[string]bool{"diskutil": true}
	execCommand = fakeCommand(nil, stderrs, nil, exitFails)

	du := DiskUtil{}
	err := du.Rename("/example/volume", "newname")
	if err := asHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("Rename returned unexpected error: %v, want type: *exec.ExitError", err)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })
	execCommand = fakeCommand(nil, nil, nil, nil)

	du := DiskUtil{}
	err := du.DeleteSnapshot("/example/volume", Snapshot{
		Name: "example-snapshot",
		UUID: "example-snapshot-uuid",
	})
	if err := asHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
}

func TestDeleteSnapshot_Errors(t *testing.T) {
	t.Cleanup(func() { execCommand = exec.Command })
	stderrs := map[string]string{"diskutil": "example stderr"}
	exitFails := map[string]bool{"diskutil": true}
	execCommand = fakeCommand(nil, stderrs, nil, exitFails)

	du := DiskUtil{}
	err := du.DeleteSnapshot("/example/volume", Snapshot{
		Name: "example-snapshot",
		UUID: "example-snapshot-uuid",
	})
	if err := asHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("DeleteSnapshot returned unexpected error: %v, want type: *exec.ExitError", err)
	}
}
