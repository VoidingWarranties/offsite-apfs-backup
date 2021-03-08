# Offsite APFS Backup

A non-destructive incremental backup utility for cloning APFS snapshots. The
purpose is to automate the process of cloning APFS volumes to off-site devices
when those devices are brought on-site.

## How to use it

1. Initialize target volumes:

   `sudo go run main.go -initialize /Volumes/source /Volumes/target`

2. At a later date when source has new data, incrementally clone the changes
   from source to targets:

   `sudo go run main.go /Volumes/source /Volumes/target`

## How it works

In short, it automates the process of calling `diskutil apfs listsnapshots` and
`asr restore`.

1. List snapshots on the source volume and each target volume.
2. For each target, find the most recent snapshot common to both source and
   target.
3. For each target, restore the volume to the source's most recent snapshot by
   applying the diff between source's most recent snapshot and the most recent
   common snapshot.

## Caveats

This utility does not create new snapshots. A snapshot must already exist on
the source volume for it to be restored to the target volume. This is
challenging, as the there are limited methods for creating APFS snapshots on
MacOS, each with their own caveats.

* Snapshots created with `tmutil snapshot` are frequently garbage collected.
* MacOS's `fs_snapshot_create` syscall requires an entitlement
  (com.apple.developer.vfs.snapshot).
* Snapshots created by other backup utilities, which have the
  com.apple.developer.vfs.snapshot entitlement, are subject to that backup
  utility's snapshot retention policy.
