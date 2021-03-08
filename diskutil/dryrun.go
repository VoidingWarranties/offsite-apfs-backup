package diskutil

type DryRunDiskUtil struct {
	du DiskUtil
}

func NewDryRun(opts ...option) DryRunDiskUtil {
	return DryRunDiskUtil{
		du: New(opts...),
	}
}

func (dry DryRunDiskUtil) Info(volume string) (VolumeInfo, error) {
	return dry.du.Info(volume)
}

func (dry DryRunDiskUtil) Rename(volume VolumeInfo, name string) error {
	return nil
}

func (dry DryRunDiskUtil) ListSnapshots(volume VolumeInfo) ([]Snapshot, error) {
	return dry.du.ListSnapshots(volume)
}

func (dry DryRunDiskUtil) DeleteSnapshot(volume VolumeInfo, snap Snapshot) error {
	return nil
}
