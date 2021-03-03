package plutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"reflect"
	"testing"

	"apfs-snapshot-diff-clone/testutils/fakecmd"

	"github.com/google/go-cmp/cmp"
)

func TestHelperProcess(*testing.T) {
	fakecmd.HelperProcess()
}

type simpleStruct struct {
	Val string `json:"val"`
}

func TestDecodePlist(t *testing.T) {
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
			fakeCmdOpts := fakecmd.Options{
				Stdouts:    map[string]string{"plutil": test.stdout},
				Stderrs:    map[string]string{"plutil": test.stderr},
				WantStdins: map[string]string{"plutil": test.wantStdin},
			}
			execCmd := fakecmd.FakeCommand(fakeCmdOpts)
			pl := New(WithExecCommand(execCmd))
			got := simpleStruct{}
			err := pl.DecodePlist(test.r, &got)
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				// TODO: it would be nice if we could `t.Fatal(string(exitErr.Stderr))` instead, but exec.Cmd.Wait() does not populate this field. I don't see why it couldn't. Add it!
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("DecodePlist returned unexpected error: %q, want: nil", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("DecodePlist resulted in unexpected value. -want +got:\n%s", diff)
			}
		})
	}
}

func TestDecodePlist_Errors(t *testing.T) {
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
			fakeCmdOpts := fakecmd.Options{
				Stdouts:   map[string]string{"plutil": test.stdout},
				Stderrs:   map[string]string{"plutil": test.stderr},
				ExitFails: map[string]bool{"plutil": test.exitFail},
			}
			execCmd := fakecmd.FakeCommand(fakeCmdOpts)
			pl := New(WithExecCommand(execCmd))
			err := pl.DecodePlist(nil, &simpleStruct{})
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("DecodePlist returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}
