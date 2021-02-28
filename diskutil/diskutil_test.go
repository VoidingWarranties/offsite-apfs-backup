package diskutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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

// isHelperProcessErr returns true if any error in err's chain is an
// *os.ExitError with exit code equal to the magic number 42. Use it to
// determine if a (potentially wrapped) error from running a exec.Cmd was
// caused by an unintended error in the TestHelperProcess func.
func isHelperProcessErr(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == helperProcessErrExitCode {
		return true
	}
	return false
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
			if isHelperProcessErr(err) {
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

	tests := []struct{
		name     string
		stdout   string
		stderr   string
		exitFail bool
	}{
		{
			name:       "non-0 exit code",
			stdout:     "{}",
			stderr:     "example stderr foobar",
			exitFail:   true,
		},
		{
			name:   "empty stdout returns JSON decode error",
			stdout: "",
		},
		{
			name:   "invalid JSON returns decode error",
			stdout: "not-json",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdouts := map[string]string{"plutil": test.stdout}
			stderrs := map[string]string{"plutil": test.stderr}
			exitFails := map[string]bool{"plutil": test.exitFail}
			execCommand = fakeCommand(stdouts, stderrs, nil, exitFails)

			err := decodePlist(nil, &simpleStruct{})
			if err == nil {
				t.Fatal("decodePlist returned nil error, want non-nil")
			}
			if isHelperProcessErr(err) {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.stderr) {
				t.Errorf("decodePlist returned error (%q) that does not contain stderr (%q)", err, test.stderr)
			}
		})
	}
}
