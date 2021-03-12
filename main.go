package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/voidingwarranties/offsite-apfs-backup/asr"
	"github.com/voidingwarranties/offsite-apfs-backup/cloner"
	"github.com/voidingwarranties/offsite-apfs-backup/diskutil"
)

var (
	prune = flag.Bool("prune", false, `If true, prune from target the latest snapshot that source and target had in common before the clone.
If false (default), no snapshots are removed from target.
Incompatible with -initialize.`)
	initialize = flag.Bool("initialize", false, `If true, initialize targets to the latest snapshot in source. All data on targets will be lost.
Set -initialize to true when first setting up an off-site backup volume.
If false (default), nondestructively clone the latest APFS snapshot in source to targets using the latest snapshot in common.
Incompatible with -prune.`)
	dryrun = flag.Bool("dryrun", false, `If true, only print the changes that would have been made to targets.
Does not modify targets in any way.`)
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s [-prune] [-initialize] [-dryrun] [--] <source volume> <target volume> [<target volume>...]

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
	if err := validateFlags(targets); err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "Error:", err)
		flag.Usage()
		os.Exit(1)
	}

	// Indent the stdout of cloner, diskutil, and asr with a single tab, to
	// help separate different clones to different targets.
	stdout := newPrefixWriter([]byte("\t"), os.Stdout)
	du := diskutil.New()
	var r asr.ASR = asr.New(asr.Stdout(stdout))
	if *dryrun {
		du = diskutil.NewDryRun(du)
		r = asr.NewDryRun(asr.Stdout(stdout))
	}
	c := cloner.New(
		du, r,
		cloner.Prune(*prune),
		cloner.InitializeTargets(*initialize),
		cloner.Stdout(stdout),
	)
	if err := c.Cloneable(source, targets...); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if err := confirm(source, targets); err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "Error:", err)
		os.Exit(1)
	}

	errs := make(map[string]error) // Map of target volume to clone error.
	for _, target := range targets {
		fmt.Printf("Cloning %q to %q...\n", source, target)
		if err := c.Clone(source, target); err != nil {
			errs[target] = err
			fmt.Fprintf(os.Stderr, "failed to clone %q to %q: %v\n", source, target, err)
		}
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "failed to clone to %d/%d targets\n", len(errs), len(targets))
		os.Exit(1)
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

func confirm(source string, targets []string) error {
	if *initialize {
		fmt.Printf("This will delete all data on the following volumes before restoring them to %s's most recent snapshot.\n", source)
	} else {
		fmt.Println("This will keep existing snapshots but delete any data written to the following volume's after their most recent snapshot.")
	}
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

type prefixWriter struct {
	output          io.Writer
	prefix          []byte
	prefixNextWrite bool
}

func newPrefixWriter(prefix []byte, w io.Writer) *prefixWriter {
	return &prefixWriter{
		output: w,
		prefix: prefix,
		// If true, write prefix before writing any other data to
		// output on the next call to Write.
		prefixNextWrite: true,
	}
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	lines := bytes.SplitAfter(p, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		// If p ends in a newline, remove the last element so we don't write a tab
		// after the last newline.
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		if w.prefixNextWrite || i != 0 {
			if _, err := w.output.Write(w.prefix); err != nil {
				// Count the number of bytes of p we've successfully written so far
				// (excluding the current line, which we have yet to write).
				n := 0
				for ii := 0; ii < i; ii++ {
					n += len(lines[ii])
				}
				return n, err
			}
		}
		if n, err := w.output.Write(line); err != nil {
			// Count the number of bytes of p we've successfully written so far,
			// including the number of bytes of the current line that were successfully
			// written.
			for ii := 0; ii < i; ii++ {
				n += len(lines[ii])
			}
			return n, err
		}
	}
	// Only prefix the next call to Write with prefix if p ends in a
	// newline.
	w.prefixNextWrite = p[len(p)-1] == '\n'
	return len(p), nil
}
