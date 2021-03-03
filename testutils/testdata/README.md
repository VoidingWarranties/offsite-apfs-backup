1. `hdiutil create -size 31g -fs apfs -volname source original_source.dmg`
2. `hdiutil attach original_source.dmg`
3. `echo foo > /Volumes/source/foo.txt`
4. Take snapshot using Carbon Copy Cloner.
5. `echo bar > /Volumes/source/bar.txt`
6. Take snapshot using Carbon Copy Cloner.
7. `hdiutil create -size 10m -fs apfs -volname source source.dmg`
8. `hdiutil attach source.dmg`
9. `apfs restore --source /Volumes/source --target /Volumes/source\ 1 --toSnapshot <snap-1-uuid> --erase`
10. `apfs restore --source /Volumes/source --target /Volumes/source\ 1 --fromSnapshot <snap-1-uuid> --toSnapshot <snap-2-uuid> --erase`
11. `hdiutil detach /Volumes/source\ 1`
11. `hdiutil resize -size 8192b source.dmg`
12. `hdiutil convert -format UDRO -o source.dmg -ov source.dmg`
