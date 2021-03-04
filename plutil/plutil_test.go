package plutil

import (
	"encoding/json"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"apfs-snapshot-diff-clone/testutils/fakecmd"

	"github.com/google/go-cmp/cmp"
)

func TestHelperProcess(t *testing.T) {
	fakecmd.HelperProcess(t)
}

type simpleStruct struct {
	Val string `json:"val"`
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		opts []fakecmd.Option
		data []byte
		want simpleStruct
	}{
		{
			name: "unmarshals JSON stdout",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", `{"val": "example"}`),
			},
			want: simpleStruct{
				Val: "example",
			},
		},
		{
			name: "ignores unknown fields",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", `{"val": "example", "unknown": "foo"}`),
			},
			want: simpleStruct{
				Val: "example",
			},
		},
		{
			name: "ignores stderr (if exit code 0)",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("plutil", "example non-fatal error"),
			},
			want: simpleStruct{},
		},
		{
			name: "passes r to stdin",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.WantStdin("plutil", "example stdin"),
			},
			data: []byte("example stdin"),
			want: simpleStruct{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCmd := fakecmd.FakeCommand(t, test.opts...)
			pl := New(WithExecCommand(execCmd))
			got := simpleStruct{}
			err := pl.Unmarshal(test.data, &got)
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				// TODO: it would be nice if we could `t.Fatal(string(exitErr.Stderr))` instead, but exec.Cmd.Wait() does not populate this field. I don't see why it couldn't. Add it!
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("Unmarshal returned unexpected error: %q, want: nil", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Unmarshal resulted in unexpected value. -want +got:\n%s", diff)
			}
		})
	}
}

func TestUnmarshal_Errors(t *testing.T) {
	var exitErr *exec.ExitError
	var syntaxErr *json.SyntaxError

	tests := []struct {
		name      string
		opts      []fakecmd.Option
		wantErrAs interface{}
	}{
		{
			name: "non-0 exit code",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("plutil", "example stderr foobar"),
				fakecmd.ExitFail("plutil"),
			},
			wantErrAs: &exitErr,
		},
		{
			name: "invalid JSON returns unmarshal error",
			opts: []fakecmd.Option{
				fakecmd.Stdout("plutil", "not-json"),
			},
			wantErrAs: &syntaxErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execCmd := fakecmd.FakeCommand(t, test.opts...)
			pl := New(WithExecCommand(execCmd))
			err := pl.Unmarshal(nil, &simpleStruct{})
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("Unmarshal returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}
