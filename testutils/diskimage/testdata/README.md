How to create disk images for tests:

1. `hdiutil create -size 10m -fs apfs -volname name name.dmg && hdiutil attach name.dmg`
2. Write data to image, either by copying data and taking a snapshot, or by
   restoring image to another image's snapshots using:
   `asr restore --source /Volumes/source --target /Volumes/name --toSnapshot <snap-1-uuid> --erase`
3. `hdiutil detach /Volumes/name`
4. `hdiutil convert name.dmg -format ULMO -o name.dmg -ov`

Notes:

- Smallest possible APFS volume is 1.1 MB, but taking a snapshot requires the
  volume be at least 4.2 MB in size.
- Using `asr restore` seems to require a target volume size > 4.2 MB (I chose
  10 MB).
- If using a third party backup utility to create APFS snapshots (e.g. Carbon
  Copy Cloner), the initial size of the image may need to be much larger. For
  example, Carbon Copy Cloner will delete old snapshots if the free space is
  less than 30 GB. Images can be resized afterwards using `hdiutil resize`.
