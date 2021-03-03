package fakecmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Options struct {
	Stdouts    map[string]string
	Stderrs    map[string]string
	WantStdins map[string]string
	ExitFails  map[string]bool
}

func FakeCommand(opt Options) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("GO_HELPER_PROCESS_STDOUT=%s", opt.Stdouts[name]),
			fmt.Sprintf("GO_HELPER_PROCESS_STDERR=%s", opt.Stderrs[name]),
		)
		if exitFail := opt.ExitFails[name]; exitFail {
			cmd.Env = append(cmd.Env, "GO_HELPER_PROCESS_EXIT_FAIL=1")
		}
		if wantStdin, exists := opt.WantStdins[name]; exists {
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

func HelperProcess() {
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

// AsHelperProcessErr returns a non-nil error if any error in err's chain is an
// *os.ExitError with exit code equal to the magic number 42. Use it to
// determine if a (potentially wrapped) error from running a exec.Cmd was
// caused by an unintended error in the TestHelperProcess func.
//
// If err represents a helper process error and *os.ExitError.Stderr is not
// empty, an error containing just the stderr is returned. Otherwise, the
// original error is returned.
func AsHelperProcessErr(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == helperProcessErrExitCode {
		if len(exitErr.Stderr) != 0 {
			return errors.New(string(exitErr.Stderr))
		}
		return err
	}
	return nil
}
