package diskutil

type dryRun struct {
	du DiskUtil
}

// NewDryRun returns a DiskUtil that cannot modify any volumes. All
// readonly methods (Info and ListSnapshots) are passed through to the
// underlying DiskUtil, du.
func NewDryRun(du DiskUtil) DiskUtil {
	return dryRun{
		du: du,
	}
}

func (dry dryRun) Info(volume string) (VolumeInfo, error) {
	return dry.du.Info(volume)
}

func (dry dryRun) Rename(volume VolumeInfo, name string) error {
	return nil
}

func (dry dryRun) ListSnapshots(volume VolumeInfo) ([]Snapshot, error) {
	return dry.du.ListSnapshots(volume)
}

func (dry dryRun) DeleteSnapshot(volume VolumeInfo, snap Snapshot) error {
	return nil
}
