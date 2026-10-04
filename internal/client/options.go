package client

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// Names of the statistics the client can print, in the order of printing.
const (
	SectionLoad       = "load"
	SectionCPU        = "cpu"
	SectionDiskLoad   = "diskload"
	SectionDiskUsage  = "diskusage"
	SectionTopTalkers = "toptalkers"
	SectionNetStats   = "netstats"

	allSectionsName = "all"
)

var allSections = []string{
	SectionLoad, SectionCPU, SectionDiskLoad, SectionDiskUsage, SectionTopTalkers, SectionNetStats,
}

// Sections is a set of statistics to print.
type Sections map[string]bool

type Options struct {
	Addr     string
	N        int64
	M        int64
	Sections Sections
}

// ParseFlags parses the command line arguments (without the program name).
func ParseFlags(args []string, output io.Writer) (Options, error) {
	var opts Options
	var sections string

	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&opts.Addr, "addr", "localhost:8080", "address of the gRPC server")
	fs.Int64Var(&opts.N, "n", 5, "print a snapshot every n seconds")
	fs.Int64Var(&opts.M, "m", 15, "average the statistics over the last m seconds")
	fs.StringVar(&sections, "sections", allSectionsName,
		"comma separated statistics to print: "+strings.Join(allSections, ",")+" or all")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}

	parsed, err := parseSections(sections)
	if err != nil {
		return Options{}, err
	}
	opts.Sections = parsed

	return opts, nil
}

func parseSections(value string) (Sections, error) {
	result := make(Sections)

	for _, name := range strings.Split(value, ",") {
		name = strings.ToLower(strings.TrimSpace(name))

		switch {
		case name == "":
			continue
		case name == allSectionsName:
			return allSectionsSet(), nil
		case !isKnownSection(name):
			return nil, fmt.Errorf("unknown section %q, available: %s, %s",
				name, strings.Join(allSections, ", "), allSectionsName)
		}

		result[name] = true
	}

	if len(result) == 0 {
		return allSectionsSet(), nil
	}

	return result, nil
}

func allSectionsSet() Sections {
	result := make(Sections, len(allSections))
	for _, name := range allSections {
		result[name] = true
	}

	return result
}

func isKnownSection(name string) bool {
	for _, known := range allSections {
		if known == name {
			return true
		}
	}

	return false
}
