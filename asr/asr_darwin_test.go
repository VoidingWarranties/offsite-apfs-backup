// +build darwin

package asr_test

import (
	"path/filepath"
	"testing"

	"github.com/voidingwarranties/offsite-apfs-backup/asr"
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
	"github.com/voidingwarranties/offsite-apfs-backup/testutils/diskimage"

	"github.com/google/go-cmp/cmp"
)

var mounter = diskimage.Mounter{
	Relpath: "../testutils/diskimage",
}

func TestRestore(t *testing.T) {
	source := mounter.MountRO(t, diskimage.SourceImg)
	target := mounter.MountRW(t, diskimage.TargetImg)
	to := diskimage.SourceImg.Snapshots(t)[0]
	from := diskimage.SourceImg.Snapshots(t)[1]

	r := asr.New()
	if err := r.Restore(source, target, to, from); err != nil {
		t.Fatalf("Restore returned unexpected error: %v, want: nil", err)
	}

	du := diskutil.New()
	got, err := du.ListSnapshots(target)
	if err != nil {
		t.Fatal(err)
	}
	want := diskimage.SourceImg.Snapshots(t)[:]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Restore resulted in unexpected snapshots in target. -want +got:\n%s", diff)
	}
}

func TestRestore_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (source, target diskutil.VolumeInfo)
		to    diskutil.Snapshot
		from  diskutil.Snapshot
	}{
		{
			name: "source not found",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				badMountPoint := t.TempDir()
				badDevice := filepath.Join(badMountPoint, "notadevice")
				source = diskutil.VolumeInfo{
					Name:       "not-a-volume",
					UUID:       "not-a-uuid",
					MountPoint: badMountPoint,
					Device:     badDevice,
				}
				target = mounter.MountRW(t, diskimage.TargetImg)
				return source, target
			},
			to:   diskimage.SourceImg.Snapshots(t)[0],
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "target not found",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = mounter.MountRO(t, diskimage.SourceImg)
				badMountPoint := t.TempDir()
				badDevice := filepath.Join(badMountPoint, "notadevice")
				target = diskutil.VolumeInfo{
					Name:       "not-a-volume",
					UUID:       "not-a-uuid",
					MountPoint: badMountPoint,
					Device:     badDevice,
				}
				return source, target
			},
			to:   diskimage.SourceImg.Snapshots(t)[0],
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "to snapshot not found on source",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = mounter.MountRO(t, diskimage.SourceImg)
				target = mounter.MountRW(t, diskimage.TargetImg)
				return source, target
			},
			to: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-uuid",
			},
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "to and from are same",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = mounter.MountRO(t, diskimage.SourceImg)
				target = mounter.MountRW(t, diskimage.TargetImg)
				return source, target
			},
			to:   diskimage.SourceImg.Snapshots(t)[1],
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "from snapshot not found on source or target",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = mounter.MountRO(t, diskimage.SourceImg)
				target = mounter.MountRW(t, diskimage.TargetImg)
				return source, target
			},
			to: diskimage.SourceImg.Snapshots(t)[0],
			from: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-uuid",
			},
		},
		{
			name: "source and target are same",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				info := mounter.MountRW(t, diskimage.SourceImg)
				return info, info
			},
			to:   diskimage.SourceImg.Snapshots(t)[0],
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
		{
			name: "target readonly",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = mounter.MountRO(t, diskimage.SourceImg)
				target = mounter.MountRO(t, diskimage.TargetImg)
				return source, target
			},
			to:   diskimage.SourceImg.Snapshots(t)[0],
			from: diskimage.SourceImg.Snapshots(t)[1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, target := test.setup(t)
			r := asr.New()
			err := r.Restore(source, target, test.to, test.from)
			if err == nil {
				t.Fatal("Restore returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
