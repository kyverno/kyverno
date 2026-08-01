// Command task-manifest generates a machine-readable index of the
// project's `make` targets from the self-documenting `## ` comments
// already used by `make help`.
//
// It reuses the exact same convention `make help` relies on
// (a target line matching `^[a-zA-Z_-]+:.*## .*$`) so the two never
// drift apart, and additionally groups targets under the section
// header comment blocks already present in the Makefile
// (e.g. "# TOOLS #", "# CODEGEN #") so a caller can filter by area
// without having to parse the Makefile itself.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Task describes a single documented `make` target.
type Task struct {
	Target      string `json:"target"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

var (
	targetRe  = regexp.MustCompile(`^([a-zA-Z_-]+):.*?##\s?(.*)$`)
	hashRunRe = regexp.MustCompile(`^#+$`)
	// headerRe matches the free-form text between a leading and
	// trailing '#' (e.g. "# BUILD (LOCAL) #", "# CLI TESTS #").
	// Section names in this Makefile include punctuation such as
	// parentheses, so this intentionally does not allowlist
	// characters — it only requires the line to be hash-delimited.
	headerRe = regexp.MustCompile(`^#([^#]+)#$`)
)

func main() {
	makefilePath := flag.String("makefile", "Makefile", "path to the Makefile to parse")
	outPath := flag.String("out", "", "output file path (defaults to stdout)")
	flag.Parse()

	tasks, err := parseMakefile(*makefilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "task-manifest:", err)
		os.Exit(1)
	}

	out := os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "task-manifest:", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(tasks); err != nil {
		fmt.Fprintln(os.Stderr, "task-manifest:", err)
		os.Exit(1)
	}
}

// parseMakefile extracts every self-documented target from f, in the
// order they appear, tagged with the nearest preceding section header.
//
// Section headers are the existing three-line comment blocks used
// throughout the Makefile, e.g.:
//
//	#########
//	# TOOLS #
//	#########
//
// A target belongs to a header once that header has been seen and
// until the next one is; targets before the first header (there are
// none today, but the parser tolerates it) are tagged "UNCATEGORIZED".
func parseMakefile(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		tasks    []Task
		category = "UNCATEGORIZED"
		prevHash bool
	)

	scanner := bufio.NewScanner(f)
	// Makefile lines (recipe lines, long variable definitions) can
	// exceed the default 64KiB scanner buffer.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Three-line "#####\n# NAME #\n#####" section header block.
		if prevHash {
			if m := headerRe.FindStringSubmatch(line); m != nil {
				category = strings.TrimSpace(m[1])
			}
		}
		prevHash = hashRunRe.MatchString(line)

		if m := targetRe.FindStringSubmatch(line); m != nil {
			tasks = append(tasks, Task{
				Target:      m[1],
				Description: strings.TrimSpace(m[2]),
				Category:    category,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}
