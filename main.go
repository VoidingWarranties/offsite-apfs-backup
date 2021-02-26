package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"apfs-snapshot-diff-clone/snapshot/cloner"
)

var (
	source = flag.String("source", "", "source APFS volume to clone - may be a mount point, /dev/ path, or volume UUID")
	targets targetsFlag
)

func init() {
	flag.Var(&targets, "target", "target APFS volume to clone to - may be specified multiple times - may be a mount point, /dev/ path, or volume UUID")
}

type targetsFlag []string

func (f *targetsFlag) String() string {
	return fmt.Sprint(*f)
}

func (f *targetsFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	flag.Parse()
	if err := validateFlags(); err != nil {
		log.Fatal(err)
	}

	for _, target := range targets {
		c := cloner.New()
		if err := c.Clone(*source, target); err != nil {
			log.Fatalf("failed to clone %q to %q: %v", *source, target, err)
		}
	}
}

func validateFlags() error {
	if *source == "" {
		return errors.New("-source is required")
	}
	if len(targets) == 0 {
		return errors.New("at least one -target is required")
	}
	return nil
}
