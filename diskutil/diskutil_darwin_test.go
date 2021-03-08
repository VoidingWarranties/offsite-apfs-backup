// +build darwin

package diskutil_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
	"github.com/voidingwarranties/offsite-apfs-backup/testutils/diskimage"
)

var (
	nonexistentVolume = diskutil.VolumeInfo{
		Name:           "Not A Volume",
		UUID:           "not-a-volume-uuid",
		MountPoint:     "/not/a/volume",
		Device:         "/dev/not-a-volume-device",
		Writable:       true,
		FileSystemType: "apfs",
		FileSystem:     "APFS",
	}

	mounter = diskimage.Mounter{
		Relpath: "../testutils/diskimage",
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
				return mounter.MountRO(t, diskimage.SourceImg)
			},
			volume: diskimage.SourceImg.UUID(t),
		},
		{
			name: "case sensitive APFS volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				return mounter.MountRW(t, diskimage.CaseSensitiveAPFSImg)
			},
			volume: diskimage.CaseSensitiveAPFSImg.UUID(t),
		},
		{
			name: "readwrite HFS+ volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				return mounter.MountRW(t, diskimage.HFSImg)
			},
			volume: diskimage.HFSImg.UUID(t),
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
	info := mounter.MountRO(t, diskimage.SourceImg)
	du := diskutil.New()
	got, err := du.ListSnapshots(info)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want := diskimage.SourceImg.Snapshots(t)[:]
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
	info := mounter.MountRW(t, diskimage.SourceImg)
	du := diskutil.New()
	if err := du.Rename(info, "newname"); err != nil {
		t.Fatalf("Rename returned unexpected error: %v, want: nil", err)
	}
	got, err := du.Info(info.Device)
	if err != nil {
		t.Fatalf("Info returned unexpected error: %v, want: nil", err)
	}
	want := info
	want.Name = "newname"
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
	info := mounter.MountRW(t, diskimage.SourceImg)
	du := diskutil.New()
	err := du.DeleteSnapshot(info, diskimage.SourceImg.Snapshots(t)[1])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err := du.ListSnapshots(info)
	if err != nil {
		t.Fatalf("ListSnapshots returned unexpected error: %v, want: nil", err)
	}
	want := diskimage.SourceImg.Snapshots(t)[:1]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeleteSnapshot resulted in unexpected snapshots. -want +got:\n%s", diff)
	}

	err = du.DeleteSnapshot(info, diskimage.SourceImg.Snapshots(t)[0])
	if err != nil {
		t.Fatalf("DeleteSnapshot returned unexpected error: %v, want: nil", err)
	}
	got, err = du.ListSnapshots(info)
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
		setup func(*testing.T) diskutil.VolumeInfo
		snap  diskutil.Snapshot
	}{
		{
			name: "volume not found",
			setup: func(*testing.T) diskutil.VolumeInfo {
				return nonexistentVolume
			},
			snap: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "snapshot not found",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				return mounter.MountRW(t, diskimage.SourceImg)
			},
			snap: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-snapshot-uuid",
			},
		},
		{
			name: "readonly volume",
			setup: func(t *testing.T) diskutil.VolumeInfo {
				return mounter.MountRO(t, diskimage.SourceImg)
			},
			snap: diskimage.SourceImg.Snapshots(t)[1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			volume := test.setup(t)
			du := diskutil.New()
			err := du.DeleteSnapshot(volume, test.snap)
			if err == nil {
				t.Fatal("DeleteSnapshot returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
