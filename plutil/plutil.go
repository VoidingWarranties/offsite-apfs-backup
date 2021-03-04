// Package plutil implements plist unmarshalling using MacOS's plutil.
//
//	data := `<?xml version="1.0" encoding="UTF-8"?>
//	<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
//	<plist version="1.0">
//	<dict>
//	        <key>Name</key>
//	        <string>Bob</string>
//	        <key>Age</key>
//	        <integer>42</integer>
//	</dict>
//	</plist>`
//	var person struct {
//		Name string
//		Age  int
//	}
//	pl := plutil.New()
//	if err := pl.Unmarshal(data, &person); err != nil {
//		log.Fatal(err)
//	}
package plutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// PLUtil parses and unmarshals plist-encoded data.
type PLUtil struct {
	execCommand func(string, ...string) *exec.Cmd
}

// Option configures the behavior of PLUtil.
type Option func(*PLUtil)

// WithExecCommand FOR USE IN TESTS ONLY replaces all uses of exec.Command with
// f. It's used in tests to avoid calling the real plutil.
func WithExecCommand(f func(string, ...string) *exec.Cmd) Option {
	return func(pl *PLUtil) {
		pl.execCommand = f
	}
}

// New returns a new PLUtil with the given options.
func New(opts ...Option) PLUtil {
	pl := PLUtil{
		execCommand: exec.Command,
	}
	for _, opt := range opts {
		opt(&pl)
	}
	return pl
}

// Unmarshal parses the plist-encoded data and stores the result in the value
// pointed to by v.
//
// Internally, Unmarshal first converts the data to JSON before unmarshalling
// the data using the encoding/json package. Therefore v must be unmarshallable
// by json.Unmarshal, and the names of the fields of v must match the names of
// the keys of the plist-encoded data, or have `json:"name"` tags.
func (pl PLUtil) Unmarshal(data []byte, v interface{}) error {
	cmd := pl.execCommand(
		"plutil",
		"-convert", "json",
		// Read from stdin.
		"-",
		// Output to stdout.
		"-o", "-")
	cmd.Stdin = bytes.NewReader(data)
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
