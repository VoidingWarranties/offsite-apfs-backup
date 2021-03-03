// +build darwin

package asr_test

import (
	"path/filepath"
	"testing"

	"apfs-snapshot-diff-clone/asr"
	"apfs-snapshot-diff-clone/diskutil"
	"apfs-snapshot-diff-clone/testutils/diskimage"

	"github.com/google/go-cmp/cmp"
)

var (
	sourceImg = filepath.Join("../testutils", diskimage.SourceImg)
	targetImg = filepath.Join("../testutils", diskimage.TargetImg)
)

func TestRestore(t *testing.T) {
	source := diskimage.SourceInfo
	target := diskimage.TargetInfo
	source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
	target.MountPoint, target.Device = diskimage.MountRW(t, targetImg)
	to := diskimage.SourceSnaps[0]
	from := diskimage.SourceSnaps[1]

	r := asr.ASR{}
	if err := r.Restore(source, target, to, from); err != nil {
		t.Fatalf("Restore returned unexpected error: %v, want: nil", err)
	}

	du := diskutil.New()
	got, err := du.ListSnapshots(target.UUID)
	if err != nil {
		t.Fatal(err)
	}
	want := diskimage.SourceSnaps[:]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Restore resulted in unexpected snapshots in target. -want +got:\n%s", diff)
	}
}

func TestRestore_Errors(t *testing.T) {
	tests := []struct{
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
					Name: "not-a-volume",
					UUID: "not-a-uuid",
					MountPoint: badMountPoint,
					Device: badDevice,
				}
				target = diskimage.TargetInfo
				target.MountPoint, target.Device = diskimage.MountRW(t, targetImg)
				return source, target
			},
			to:   diskimage.SourceSnaps[0],
			from: diskimage.SourceSnaps[1],
		},
		{
			name: "target not found",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.TargetInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				badMountPoint := t.TempDir()
				badDevice := filepath.Join(badMountPoint, "notadevice")
				target = diskutil.VolumeInfo{
					Name: "not-a-volume",
					UUID: "not-a-uuid",
					MountPoint: badMountPoint,
					Device: badDevice,
				}
				return source, target
			},
			to:   diskimage.SourceSnaps[0],
			from: diskimage.SourceSnaps[1],
		},
		{
			name: "to snapshot not found on source",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.SourceInfo
				target = diskimage.TargetInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				target.MountPoint, target.Device = diskimage.MountRW(t, targetImg)
				return source, target
			},
			to: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-uuid",
			},
			from: diskimage.SourceSnaps[1],
		},
		{
			name: "to and from are same",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.SourceInfo
				target = diskimage.TargetInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				target.MountPoint, target.Device = diskimage.MountRW(t, targetImg)
				return source, target
			},
			to:   diskimage.SourceSnaps[1],
			from: diskimage.SourceSnaps[1],
		},
		{
			name: "from snapshot not found on source or target",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.SourceInfo
				target = diskimage.TargetInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				target.MountPoint, target.Device = diskimage.MountRW(t, targetImg)
				return source, target
			},
			to: diskimage.SourceSnaps[0],
			from: diskutil.Snapshot{
				Name: "not-a-snapshot",
				UUID: "not-a-uuid",
			},
		},
		{
			name: "source and target are same",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.SourceInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				return source, source
			},
			to:   diskimage.SourceSnaps[0],
			from: diskimage.SourceSnaps[1],
		},
		{
			name: "target readonly",
			setup: func(t *testing.T) (source, target diskutil.VolumeInfo) {
				source = diskimage.SourceInfo
				target = diskimage.TargetInfo
				source.MountPoint, source.Device = diskimage.MountRO(t, sourceImg)
				target.MountPoint, target.Device = diskimage.MountRO(t, targetImg)
				return source, target
			},
			to:   diskimage.SourceSnaps[0],
			from: diskimage.SourceSnaps[1],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, target := test.setup(t)
			r := asr.ASR{}
			err := r.Restore(source, target, test.to, test.from)
			if err == nil {
				t.Fatal("Restore returned unexpected error: nil, want: non-nil")
			}
		})
	}
}
