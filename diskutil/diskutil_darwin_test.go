// +build darwin

package diskutil_test

import (
	"testing"
	"path/filepath"

	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/testutils/diskimage"

	"github.com/google/go-cmp/cmp"
)

var (
	img      = filepath.Join("../testutils/", diskimage.SourceImg)
	imgInfo  = diskimage.SourceInfo
	imgSnaps = diskimage.SourceSnaps
)

func TestInfo(t *testing.T) {
	mountpoint, device := diskimage.MountRO(t, img)
	du := diskutil.New()
	got, err := du.Info(mountpoint)
	if err != nil {
		t.Fatalf("Info returned unexpected error: %v, want: nil", err)
	}
	want := imgInfo
	want.MountPoint = mountpoint
	want.Device = device
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Info returned unexpected volume info. -want +got:\n%s", diff)
	}
}

func TestInfo_Errors(t *testing.T) {
	du := diskutil.New()
	_, err := du.Info(t.TempDir())
	if err == nil {
		t.Fatal("Info returned unexpected error: nil, want: non-nil", err)
	}
}

func TestListSnapshots(t *testing.T) {
	mountpoint, _ := diskimage.MountRO(t, img)
	du := diskutil.New()
	got, err := du.ListSnapshots(mountpoint)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want := imgSnaps[:]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListSnapshots returned unexpected snapshots: -want +got:\n%s", diff)
	}
}

func TestListSnapshots_Error(t *testing.T) {
	du := diskutil.New()
	_, err := du.ListSnapshots(t.TempDir())
	if err == nil {
		t.Fatal("ListSnapshots returned unexpected error: nil, want: non-nil", err)
	}
}

func TestRename(t *testing.T) {
	mountpoint, device := diskimage.MountRW(t, img)
	du := diskutil.New()
	if err := du.Rename(device, "newname"); err != nil {
		t.Fatalf("Rename returned unexpected error: %v, want: nil", err)
	}
	got, err := du.Info(device)
	if err != nil {
		t.Fatalf("Info returned unexpected error: %v, want: nil", err)
	}
	want := imgInfo
	want.Name = "newname"
	want.MountPoint = mountpoint
	want.Device = device
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Rename resulted in unexpected results. -want +got:\n%s", diff)
	}
}

func TestRename_Errors(t *testing.T) {
	du := diskutil.New()
	err := du.Rename(t.TempDir(), "newname")
	if err == nil {
		t.Fatal("Rename returned unexpected error: nil, want: non-nil")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	mountpoint, _ := diskimage.MountRW(t, img)
	du := diskutil.New()
	err := du.DeleteSnapshot(mountpoint, imgSnaps[1])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err := du.ListSnapshots(mountpoint)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want := imgSnaps[:1]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeleteSnapshot resulted in unexpected snapshots. -want +got:\n%s", diff)
	}

	err = du.DeleteSnapshot(mountpoint, imgSnaps[0])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err = du.ListSnapshots(mountpoint)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want = nil
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeleteSnapshot resulted in unexpected snapshots. -want +got:\n%s", diff)
	}
}

func TestDeleteSnapshot_Errors(t *testing.T) {
	tests := []struct{
		name  string
		setup func(*testing.T) (mountpoint string)
		snap  diskutil.Snapshot
	}{
		{
			name: "volume not found",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			snap: imgSnaps[1],
		},
		{
			name: "snapshot not found",
			setup: func(t *testing.T) string {
				mountpoint, _ := diskimage.MountRW(t, img)
				return mountpoint
			},
			snap: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-snapshot-uuid",
			},
		},
		{
			name: "readonly volume",
			setup: func(t *testing.T) string {
				mountpoint, _ := diskimage.MountRO(t, img)
				return mountpoint
			},
			snap: imgSnaps[1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mountpoint := test.setup(t)
			du := diskutil.New()
			err := du.DeleteSnapshot(mountpoint, test.snap)
			if err == nil {
				t.Fatal("DeleteSnapshot returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
