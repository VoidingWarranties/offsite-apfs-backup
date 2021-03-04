// Package fakecmd provides utilities for testing code that contains calls to
// exec.Command. For example, consider the following function to test:
//	var execCommand = exec.Command
//
//	func CountFiles(path string) (int, error) {
//		lsCmd := execCommand("ls", path)
//		lsStdout, err := lsCmd.StdoutPipe()
//		if err != nil {
//			return 0, err
//		}
//		if err := lsCmd.Start(); err != nil {
//			return 0, err
//		}
//
//		wcCmd := execCommand("wc", "-l")
//		wcCmd.Stdin = lsStdout
//		wcStdout, err := wcCmd.Output()
//		if err != nil {
//			return 0, fmt.Errorf("wc error: %w", err)
//		}
//		if err := lsCmd.Wait(); err != nil {
//			return 0, fmt.Errorf("ls error: %w", err)
//		}
//		return strconv.Atoi(strings.TrimSpace(wcStdout))
//	}
//
// This function can be tested using the fakecmd package like so:
//	func TestHelperProcess(t *testing.T) {
//		fakecmd.HelperProcess(t)
//	}
//
//	func TestCountFiles(t *testing.T) {
//		t.Cleanup(func() { execCommand = exec.Command })
//		execCommand = fakecmd.FakeCommand(t,
//			fakecmd.Stdout("ls", "example-ls-stdout"),
//			fakecmd.Stdout("wc", "     5"),
//			fakecmd.WantStdin("wc", "example-ls-stdout"),
//		})
//		got, err := CountFiles("/example/path")
//		if err := fakecmd.AsHelperProcessErr(err); err != nil {
//			t.Fatal(err)
//		}
//		if err != nil {
//			t.Fatalf("CountFiles returned unexpected error: %v, want: nil", err)
//		}
//		if got != 5 {
//			t.Errorf("CountFiles returned unexpected number of files: %d, want: 5", got)
//		}
//	}
package fakecmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
)

// config defines the behaviors of commands faked by FakeCommand. All keys in
// the top-level maps are command names.
type config struct {
	stdouts    map[string]string
	stderrs    map[string]string
	exitFails  map[string]bool
	wantStdins map[string]string
	wantArgs   map[string]map[string]bool // Map value is set of args.
}

// Option configures the behavior of a command faked by FakeCommand.
type Option func(*config)

// Stdout set the stdout that will be output by `name`.
func Stdout(name string, stdout string) Option {
	return func(conf *config) {
		conf.stdouts[name] = stdout
	}
}

// Stderr sets the stderr that will be output by `name`.
func Stderr(name string, stderr string) Option {
	return func(conf *config) {
		conf.stderrs[name] = stderr
	}
}

// ExitFail causes `name` to exit with exit code 1.
func ExitFail(name string) Option {
	return func(conf *config) {
		conf.exitFails[name] = true
	}
}

// WantStdin sets the expected value of `name`'s stdin. If a different value is
// received, the helper process exits in such a way that AsHelperProcessErr
// returns non-nil.
func WantStdin(name string, stdin string) Option {
	return func(conf *config) {
		conf.wantStdins[name] = stdin
	}
}

// WantArg adds `arg` to the set of `name`'s expected arguments. If `arg` is
// not present in the command, the execCommand function returned by FakeCommand
// will t.Error.
func WantArg(name string, arg string) Option {
	return func(conf *config) {
		wantArgs := conf.wantArgs[name]
		if wantArgs == nil {
			wantArgs = make(map[string]bool)
		}
		wantArgs[arg] = true
		conf.wantArgs[name] = wantArgs
	}
}

// FakeCommand returns a function suitable for replacing a call to
// exec.Command in tests. Inspired by the stdlib's exec_test. Modified to allow
// specifying different stdouts, stderrs, stdins, and exit codes per command.
func FakeCommand(t *testing.T, opts ...Option) func(string, ...string) *exec.Cmd {
	conf := config{
		stdouts:    make(map[string]string),
		stderrs:    make(map[string]string),
		exitFails:  make(map[string]bool),
		wantStdins: make(map[string]string),
		wantArgs:   make(map[string]map[string]bool),
	}
	for _, opt := range opts {
		opt(&conf)
	}
	return func(name string, args ...string) *exec.Cmd {
		validateArgs(t, name, conf.wantArgs[name], args)
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("GO_HELPER_PROCESS_STDOUT=%s", conf.stdouts[name]),
			fmt.Sprintf("GO_HELPER_PROCESS_STDERR=%s", conf.stderrs[name]),
		)
		if exitFail := conf.exitFails[name]; exitFail {
			cmd.Env = append(cmd.Env, "GO_HELPER_PROCESS_EXIT_FAIL=1")
		}
		if wantStdin, exists := conf.wantStdins[name]; exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("GO_HELPER_PROCESS_WANT_STDIN=%s", wantStdin))
		}
		return cmd
	}
}

func validateArgs(t *testing.T, name string, want map[string]bool, got []string) {
	gotArgSet := make(map[string]bool)
	for _, arg := range got {
		gotArgSet[arg] = true
	}
	for arg := range want {
		if !gotArgSet[arg] {
			t.Errorf("expected %q to be called with arg %q", name, arg)
		}
	}
}

// Magic number to indicate that the error is caused by an error in the
// TestHelperProcess function, rather than an intended "fake" error. Can be any
// number, as long as the number is not the same as an exit code chosen by a
// test case.
const helperProcessErrExitCode = 42

// HelperProcess writes the values of environment variables
// GO_HELPER_PROCESS_STDOUT and GO_HELPER_PROCESS_STDERR to standard out and
// standard error, respectively. It also validates that the standard input
// matches the value of environment variable GO_HELPER_PROCESS_WANT_STDIN.
//
// HelperProcess must be called, and only called, in a test function named
// TestHelperProcess that does nothing else.
func HelperProcess(t *testing.T) {
	if t.Name() != "TestHelperProcess" {
		panic("HelperProcess must be called (and only called) in a test function named TestHelperProcess")
	}
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
