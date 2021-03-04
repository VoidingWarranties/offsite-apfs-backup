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

func newWithFakeCmd(t *testing.T, opts fakecmd.Options) DiskUtil {
	execCmd := fakecmd.FakeCommand(t, opts)
	pl := plutil.New(plutil.WithExecCommand(execCmd))
	return New(
		WithExecCommand(execCmd),
		WithPLUtil(pl),
	)
}

func TestInfo(t *testing.T) {
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
					"MountPoint": "/foo/mount/point",
					"DeviceNode": "/dev/disk1s2"
				}`,
			},
			wantStdins: map[string]string{
				"plutil": "<plist diskutil output>",
			},
			want: VolumeInfo{
				UUID:       "foo-uuid",
				Name:       "foo-name",
				MountPoint: "/foo/mount/point",
				Device:     "/dev/disk1s2",
			},
		},
		{
			name: "ignores stderr (if exit code 0)",
			stdouts: map[string]string{
				"diskutil": "<plist diskutil output>",
				"plutil": `{
					"VolumeUUID": "bar-uuid",
					"VolumeName": "bar-name",
					"MountPoint": "/bar/mount/point",
					"DeviceNode": "/dev/disk3s4"
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
				Device:     "/dev/disk3s4",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeCmdOpts := fakecmd.Options{
				Stdouts:    test.stdouts,
				Stderrs:    test.stderrs,
				WantStdins: test.wantStdins,
			}
			du := newWithFakeCmd(t, fakeCmdOpts)
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
			fakeCmdOpts := fakecmd.Options{
				Stdouts:    test.stdouts,
				Stderrs:    test.stderrs,
				WantStdins: test.wantStdins,
				ExitFails:  test.exitFails,
			}
			du := newWithFakeCmd(t, fakeCmdOpts)
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
	}
)

func TestListSnapshots(t *testing.T) {
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
			fakeCmdOpts := fakecmd.Options{
				Stdouts:    test.stdouts,
				Stderrs:    test.stderrs,
				WantStdins: test.wantStdins,
			}
			du := newWithFakeCmd(t, fakeCmdOpts)
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

func TestListSnapshots_Errors(t *testing.T) {
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
			fakeCmdOpts := fakecmd.Options{
				Stdouts:    test.stdouts,
				Stderrs:    test.stderrs,
				WantStdins: test.wantStdins,
				ExitFails:  test.exitFails,
			}
			du := newWithFakeCmd(t, fakeCmdOpts)
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
	du := newWithFakeCmd(t, fakecmd.Options{})
	err := du.Rename(exampleVolumeInfo, "newname")
	if err := fakecmd.AsHelperProcessErr(err); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("Rename returned unexpected error: %v, want: nil", err)
	}
}

func TestRename_Errors(t *testing.T) {
	fakeCmdOpts := fakecmd.Options{
		Stderrs:   map[string]string{"diskutil": "example stderr"},
		ExitFails: map[string]bool{"diskutil": true},
	}
	du := newWithFakeCmd(t, fakeCmdOpts)
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
	du := newWithFakeCmd(t, fakecmd.Options{})
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

func TestDeleteSnapshot_Errors(t *testing.T) {
	fakeCmdOpts := fakecmd.Options{
		Stderrs:   map[string]string{"diskutil": "example stderr"},
		ExitFails: map[string]bool{"diskutil": true},
	}
	du := newWithFakeCmd(t, fakeCmdOpts)
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
