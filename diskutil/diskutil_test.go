package diskutil

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"apfs-snapshot-diff-clone/plutil"
	"apfs-snapshot-diff-clone/testutils/fakecmd"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestHelperProcess(t *testing.T) {
	fakecmd.HelperProcess(t)
}

func newWithFakeCmd(t *testing.T, opts ...fakecmd.Option) DiskUtil {
	execCmd := fakecmd.FakeCommand(t, opts...)
	pl := plutil.New(plutil.WithExecCommand(execCmd))
	return New(
		withExecCommand(execCmd),
		withPLUtil(pl),
	)
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name string
		opts []fakecmd.Option
		want VolumeInfo
	}{
		{
			name: "success",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "<plist diskutil output>"),
				fakecmd.Stdout("plutil", `{
					"VolumeUUID": "foo-uuid",
					"VolumeName": "foo-name",
					"MountPoint": "/foo/mount/point",
					"DeviceNode": "/dev/disk1s2",
					"WritableVolume": true,
					"FilesystemType": "apfs"
				}`),
				fakecmd.WantStdin("plutil", "<plist diskutil output>"),
			},
			want: VolumeInfo{
				UUID:       "foo-uuid",
				Name:       "foo-name",
				MountPoint: "/foo/mount/point",
				Device:     "/dev/disk1s2",
				Writable:   true,
				FileSystem: "apfs",
			},
		},
		{
			name: "ignores stderr (if exit code 0)",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "<plist diskutil output>"),
				fakecmd.Stdout("plutil", `{
					"VolumeUUID": "bar-uuid",
					"VolumeName": "bar-name",
					"MountPoint": "/bar/mount/point",
					"DeviceNode": "/dev/disk3s4",
					"WritableVolume": false,
					"FilesystemType": "hfs+"
				}`),
				fakecmd.Stderr("diskutil", "diskutil-stderr"),
				fakecmd.Stderr("plutil", "plutil-stderr"),
				fakecmd.WantStdin("plutil", "<plist diskutil output>"),
			},
			want: VolumeInfo{
				UUID:       "bar-uuid",
				Name:       "bar-name",
				MountPoint: "/bar/mount/point",
				Device:     "/dev/disk3s4",
				Writable:   false,
				FileSystem: "hfs+",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			du := newWithFakeCmd(t, test.opts...)
			got, err := du.Info("/example/volume")
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
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
	var exitErr *exec.ExitError
	var plistErr plistError

	tests := []struct {
		name      string
		opts      []fakecmd.Option
		wantErrAs interface{}
	}{
		{
			name: "diskutil exec errors",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("diskutil", "stderr"),
				fakecmd.WantStdin("plutil", "foo-stdout"),
				fakecmd.ExitFail("diskutil"),
			},
			wantErrAs: &exitErr,
		},
		{
			name: "plutil exec errors",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("plutil", "stderr"),
				fakecmd.WantStdin("plutil", "foo-stdout"),
				fakecmd.ExitFail("plutil"),
			},
			wantErrAs: &exitErr,
		},
		{
			name: "diskutil plist error output - returns plist error",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "diskutil-plist-err"),
				fakecmd.Stdout("plutil", `{"Error": true, "ErrorMessage": "diskutil err message"}`),
				fakecmd.WantStdin("plutil", "diskutil-plist-err"),
				fakecmd.ExitFail("diskutil"),
			},
			wantErrAs: &plistErr,
		},
		{
			name: "diskutil plist error output - plist error wraps exec.ExitError",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "diskutil-plist-err"),
				fakecmd.Stdout("plutil", `{"Error": true, "ErrorMessage": "diskutil err message"}`),
				fakecmd.WantStdin("plutil", "diskutil-plist-err"),
				fakecmd.ExitFail("diskutil"),
			},
			wantErrAs: &exitErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			du := newWithFakeCmd(t, test.opts...)
			_, err := du.Info("/example/volume")
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("Info returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}

var (
	exampleVolumeInfo = VolumeInfo{
		Name:       "Example Volume",
		UUID:       "example-volume-uuid",
		MountPoint: "/example/volume",
		Device:     "/dev/example-volume",
		Writable:   true,
		FileSystem: "apfs",
	}
)

func TestListSnapshots(t *testing.T) {
	tests := []struct {
		name string
		opts []fakecmd.Option
		want []Snapshot
	}{
		{
			name: "multiple snapshots",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "<plist diskutil output>"),
				fakecmd.Stdout("plutil", `{
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
				}`),
				fakecmd.WantStdin("plutil", "<plist diskutil output>"),
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
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "<plist diskutil output>"),
				fakecmd.Stdout("plutil", `{
					"Snapshots": []
				}`),
				fakecmd.WantStdin("plutil", "<plist diskutil output>"),
			},
			want: []Snapshot{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			du := newWithFakeCmd(t, test.opts...)
			got, err := du.ListSnapshots(exampleVolumeInfo)
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
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

func TestListSnapshots_IDsVolumesByUUID(t *testing.T) {
	du := newWithFakeCmd(t,
		fakecmd.Stdout("plutil", `{
			"Snapshots": []
		}`),
		fakecmd.WantArg("diskutil", exampleVolumeInfo.UUID),
	)
	_, err := du.ListSnapshots(exampleVolumeInfo)
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %q, want: nil", err)
	}
}

func TestListSnapshots_Errors(t *testing.T) {
	var exitErr *exec.ExitError
	var validationErr validationError

	tests := []struct {
		name      string
		opts      []fakecmd.Option
		wantErrAs interface{}
	}{
		{
			name: "snapshots in unexpected order",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", `{
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
				}`),
				fakecmd.WantStdin("plutil", "foo-stdout"),
			},
			wantErrAs: &validationErr,
		},
		{
			name: "no time in name",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", `{
					"Snapshots": [
						{
							"SnapshotName": "foo-snapshot-name",
							"SnapshotUUID": "foo-snapshot-uuid"
						}
					]
				}`),
				fakecmd.WantStdin("plutil", "foo-stdout"),
			},
			wantErrAs: &validationErr,
		},
		{
			name: "invalid time in name",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", `{
					"Snapshots": [
						{
							"SnapshotName": "foo-snapshot-name-2021-13-01-000000",
							"SnapshotUUID": "foo-snapshot-uuid"
						}
					]
				}`),
				fakecmd.WantStdin("plutil", "foo-stdout"),
			},
			wantErrAs: &validationErr,
		},
		{
			name: "diskutil exec errors",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("diskutil", "stderr"),
				fakecmd.WantStdin("plutil", "foo-stdout"),
				fakecmd.ExitFail("diskutil"),
			},
			wantErrAs: &exitErr,
		},
		{
			name: "plutil exec errors",
			opts: []fakecmd.Option{
				fakecmd.Stdout("diskutil", "foo-stdout"),
				fakecmd.Stdout("plutil", "{}"),
				fakecmd.Stderr("plutil", "stderr"),
				fakecmd.WantStdin("plutil", "foo-stdout"),
				fakecmd.ExitFail("plutil"),
			},
			wantErrAs: &exitErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			du := newWithFakeCmd(t, test.opts...)
			_, err := du.ListSnapshots(exampleVolumeInfo)
			if err := fakecmd.AsHelperProcessErr(err); err != nil {
				t.Fatal(err)
			}
			if !errors.As(err, test.wantErrAs) {
				t.Errorf("ListSnapshots returned unexpected error: %v, want type: %v", err, reflect.TypeOf(test.wantErrAs).Elem())
			}
		})
	}
}

func TestRename(t *testing.T) {
	du := newWithFakeCmd(t)
	err := du.Rename(exampleVolumeInfo, "newname")
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Rename returned unexpected error: %v, want: nil", err)
	}
}

func TestRename_IDsVolumesByUUID(t *testing.T) {
	du := newWithFakeCmd(t, fakecmd.WantArg("diskutil", exampleVolumeInfo.UUID))
	err := du.Rename(exampleVolumeInfo, "newname")
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Rename returned unexpected error: %q, want: nil", err)
	}
}

func TestRename_Errors(t *testing.T) {
	opts := []fakecmd.Option{
		fakecmd.Stderr("diskutil", "example stderr"),
		fakecmd.ExitFail("diskutil"),
	}
	du := newWithFakeCmd(t, opts...)
	err := du.Rename(exampleVolumeInfo, "newname")
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("Rename returned unexpected error: %v, want type: *exec.ExitError", err)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	du := newWithFakeCmd(t)
	err := du.DeleteSnapshot(exampleVolumeInfo, Snapshot{
		Name: "example-snapshot",
		UUID: "example-snapshot-uuid",
	})
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
}

func TestDeleteSnapshot_IDsVolumesByUUID(t *testing.T) {
	du := newWithFakeCmd(t, fakecmd.WantArg("diskutil", exampleVolumeInfo.UUID))
	err := du.DeleteSnapshot(exampleVolumeInfo, Snapshot{
		Name: "example-snapshot",
		UUID: "example-snapshot-uuid",
	})
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Rename returned unexpected error: %q, want: nil", err)
	}
}

func TestDeleteSnapshot_Errors(t *testing.T) {
	opts := []fakecmd.Option{
		fakecmd.Stderr("diskutil", "example stderr"),
		fakecmd.ExitFail("diskutil"),
	}
	du := newWithFakeCmd(t, opts...)
	err := du.DeleteSnapshot(exampleVolumeInfo, Snapshot{
		Name: "example-snapshot",
		UUID: "example-snapshot-uuid",
	})
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("DeleteSnapshot returned unexpected error: %v, want type: *exec.ExitError", err)
	}
}
