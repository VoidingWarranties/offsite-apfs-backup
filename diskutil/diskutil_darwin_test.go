// +build darwin

package diskutil_test

import (
	"path/filepath"
	"testing"

	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/testutils/diskimage"

	"github.com/google/go-cmp/cmp"
)

var (
	img      = filepath.Join("../testutils/diskimage", diskimage.SourceImg)
	imgInfo  = diskimage.SourceInfo
	imgSnaps = diskimage.SourceSnaps

	hfsImg  = filepath.Join("../testutils/diskimage", diskimage.HFSImg)
	hfsInfo = diskimage.HFSInfo

	caseSensitiveAPFSImg  = filepath.Join("../testutils/diskimage", diskimage.CaseSensitiveAPFSImg)
	caseSensitiveAPFSInfo = diskimage.CaseSensitiveAPFSInfo

	nonexistentVolume = diskutil.VolumeInfo{
		Name:       "Not A Volume",
		UUID:       "not-a-volume-uuid",
		MountPoint: "/not/a/volume",
		Device:     "/dev/not-a-volume-device",
		Writable:   true,
		FileSystem: "apfs",
	}
)

func TestInfo(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T) diskutil.VolumeInfo
		volume string
	}{
		{
			name: "readonly APFS volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				mountpoint, device := diskimage.MountRO(t, img)
				want := imgInfo
				want.MountPoint = mountpoint
				want.Device = device
				want.Writable = false
				return want
			},
			volume: imgInfo.UUID,
		},
		{
			name: "case sensitive APFS volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				mountpoint, device := diskimage.MountRW(t, caseSensitiveAPFSImg)
				want := caseSensitiveAPFSInfo
				want.MountPoint = mountpoint
				want.Device = device
				want.Writable = true
				return want
			},
			volume: caseSensitiveAPFSInfo.UUID,
		},
		{
			name: "readwrite HFS+ volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				mountpoint, device := diskimage.MountRW(t, hfsImg)
				want := hfsInfo
				want.MountPoint = mountpoint
				want.Device = device
				want.Writable = true
				return want
			},
			volume: hfsInfo.UUID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := test.setup(t)
			du := diskutil.New()
			got, err := du.Info(test.volume)
			if err != nil {
				t.Fatalf("Info returned unexpected error: %v, want: nil", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Info returned unexpected volume info. -want +got:\n%s", diff)
			}
		})
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
	diskimage.MountRO(t, img)
	du := diskutil.New()
	got, err := du.ListSnapshots(imgInfo)
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
	_, err := du.ListSnapshots(nonexistentVolume)
	if err == nil {
		t.Fatal("ListSnapshots returned unexpected error: nil, want: non-nil", err)
	}
}

func TestRename(t *testing.T) {
	mountpoint, device := diskimage.MountRW(t, img)
	du := diskutil.New()
	if err := du.Rename(imgInfo, "newname"); err != nil {
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
	want.Writable = true
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Rename resulted in unexpected results. -want +got:\n%s", diff)
	}
}

func TestRename_Errors(t *testing.T) {
	du := diskutil.New()
	err := du.Rename(nonexistentVolume, "newname")
	if err == nil {
		t.Fatal("Rename returned unexpected error: nil, want: non-nil")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	diskimage.MountRW(t, img)
	du := diskutil.New()
	err := du.DeleteSnapshot(imgInfo, imgSnaps[1])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err := du.ListSnapshots(imgInfo)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want := imgSnaps[:1]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeleteSnapshot resulted in unexpected snapshots. -want +got:\n%s", diff)
	}

	err = du.DeleteSnapshot(imgInfo, imgSnaps[0])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err = du.ListSnapshots(imgInfo)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want = nil
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeleteSnapshot resulted in unexpected snapshots. -want +got:\n%s", diff)
	}
}

func TestDeleteSnapshot_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T)
		info  diskutil.VolumeInfo
		snap  diskutil.Snapshot
	}{
		{
			name: "volume not found",
			info: nonexistentVolume,
			snap: imgSnaps[1],
		},
		{
			name: "snapshot not found",
			setup: func(t *testing.T) {
				diskimage.MountRW(t, img)
			},
			info: imgInfo,
			snap: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-snapshot-uuid",
			},
		},
		{
			name: "readonly volume",
			setup: func(t *testing.T) {
				diskimage.MountRO(t, img)
			},
			info: imgInfo,
			snap: imgSnaps[1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setup != nil {
				test.setup(t)
			}
			du := diskutil.New()
			err := du.DeleteSnapshot(test.info, test.snap)
			if err == nil {
				t.Fatal("DeleteSnapshot returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
