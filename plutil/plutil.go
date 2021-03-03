package plutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

var (
	execCommand = exec.Command
)

type PLUtil struct {
	execCommand func(string, ...string) *exec.Cmd
}

type Option func(*PLUtil)

func WithExecCommand(f func(string, ...string) *exec.Cmd) Option {
	return func(pl *PLUtil) {
		pl.execCommand = f
	}
}

func New(opts ...Option) PLUtil {
	pl := PLUtil{}
	defaultOpts := []Option{
		WithExecCommand(exec.Command),
	}
	opts = append(defaultOpts, opts...)
	for _, opt := range opts {
		opt(&pl)
	}
	return pl
}

func (pl PLUtil) DecodePlist(r io.Reader, v interface{}) error {
	cmd := pl.execCommand(
		"plutil",
		"-convert", "json",
		// Read from stdin.
		"-",
		// Output to stdout.
		"-o", "-")
	cmd.Stdin = r
	stdout, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("`%s` failed (%w) with stderr: %s", cmd, err, exitErr.Stderr)
	}
	if err != nil {
		return fmt.Errorf("`%s` failed (%w)", cmd, err)
	}
	if err := json.Unmarshal(stdout, v); err != nil {
		return fmt.Errorf("failed to parse json: %w", err)
	}
	return nil
}
