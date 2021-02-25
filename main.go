package main

import (
	"log"

	"apfs-snapshot-diff-clone/snapshot/cloner"
)

func main() {
	source := "/Volumes/Backup - USB"
	target := "/Volumes/Backup - Thunderbolt NVME"
	c := cloner.New()
	err := c.Clone(source, target)
	if err != nil {
		log.Fatalf("failed to clone %q to %q: %v", source, target, err)
	}
}
