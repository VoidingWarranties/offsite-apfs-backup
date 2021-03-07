package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"apfs-snapshot-diff-clone/cloner"
)

var (
	prune = flag.Bool("prune", false, `If true, prune from target the latest snapshot that source and target had in common before the clone.
If false (default), no snapshots are removed from target.
Incompatible with -initialize.`)
	initialize = flag.Bool("initialize", false, `If true, initialize targets to the latest snapshot in source. All data on targets will be lost.
Set -initialize to true when first setting up an off-site backup volume.
If false (default), nondestructively clone the latest APFS snapshot in source to targets using the latest snapshot in common.
Incompatible with -prune.`)
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s [-prune] [-initialize] [--] <source volume> <target volume> [<target volume>...]

  <source volume>
    	Source APFS volume to clone.
    	May be a mount point, /dev/ path, or volume UUID.
  <target volume>
    	Target APFS volume(s) to clone to.
    	May be specified multiple times.
    	May be a mount point, /dev/ path, or volume UUID.
`, os.Args[0])
		flag.CommandLine.PrintDefaults()
	}
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
	source, targets, err := parseArguments()
	if err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "Error:", err)
		flag.Usage()
		os.Exit(1)
	}
	c := cloner.New(cloner.Prune(*prune), cloner.InitializeTargets(*initialize))
	if err := c.Cloneable(source, targets...); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if err := validateFlags(targets); err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "Error:", err)
		flag.Usage()
		os.Exit(1)
	}

	if *initialize {
		if err := confirmInitialize(source, targets); err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), "Error:", err)
			os.Exit(1)
		}
	}

	errs := make(map[string]error) // Map of target volume to clone error.
	for _, target := range targets {
		if err := c.Clone(source, target); err != nil {
			errs[target] = err
			log.Printf("failed to clone %q to %q: %v", source, target, err)
		}
	}
	if len(errs) > 0 {
		log.Fatalf("failed to clone to %d/%d targets", len(errs), len(targets))
	}
}

func parseArguments() (source string, targets []string, err error) {
	args := flag.Args()
	if len(args) < 1 {
		return "", nil, errors.New("<source volume> and <target volume> are required")
	}
	if len(args) < 2 {
		return "", nil, errors.New("at least one <target volume> is required")
	}
	source = args[0]
	if strings.HasPrefix(source, "-") {
		return "", nil, fmt.Errorf("%q is not a valid volume", source)
	}
	targets = args[1:]
	for _, t := range targets {
		if strings.HasPrefix(t, "-") {
			return "", nil, fmt.Errorf("%q is not a valid volume", t)
		}
	}
	return source, targets, nil
}

func validateFlags(targets []string) error {
	if *initialize && *prune {
		return errors.New("-initialize and -prune are incompatible")
	}
	return nil
}

func confirmInitialize(source string, targets []string) error {
	fmt.Printf("This will delete all data on the following volumes before restoring them to %s's most recent snapshot.\n", source)
	for _, t := range targets {
		fmt.Printf("  - %s\n", t)
	}
	fmt.Print("This cannot be undone. Are you sure? y/N: ")
	r := bufio.NewReader(os.Stdin)
	response, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y":
		return nil
	case "yes":
		return nil
	}
	return errors.New("-initialize confirmation rejected")
}
