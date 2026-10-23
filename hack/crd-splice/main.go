// crd-splice merges freshly generated kyverno.io CRDs into the checked-in
// config/crds/kyverno directory while keeping named files or named
// spec.versions[] entries byte-identical to the checked-in copy. Everything
// not named by --frozen-file or --freeze tracks the generated output, so a
// new kind or version is never silently frozen. See
// config/crds/kyverno/README.md for why this exists.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// versionsHeaderLine is the fixed controller-gen line that ends a CRD's
// header block and starts its spec.versions[] list.
const versionsHeaderLine = "  versions:"

// entryPrefix marks the start of a spec.versions[] list entry.
const entryPrefix = "  - "

// nameLinePrefix marks the "name: <version>" line inside a versions entry.
const nameLinePrefix = "    name: "

func main() {
	if len(os.Args) < 2 {
		failf("usage: crd-splice <splice|extract> [flags]")
	}
	switch os.Args[1] {
	case "splice":
		runSplice(os.Args[2:])
	case "extract":
		runExtract(os.Args[2:])
	default:
		failf("unknown mode %q: want splice or extract", os.Args[1])
	}
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "crd-splice: "+format+"\n", args...)
	os.Exit(1)
}

// stringList collects repeated occurrences of a flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runSplice(args []string) {
	fs := flag.NewFlagSet("splice", flag.ExitOnError)
	var generated, target string
	var frozenFileArgs, freezeArgs stringList
	fs.StringVar(&generated, "generated", "", "directory of freshly generated CRDs")
	fs.StringVar(&target, "target", "", "checked-in CRD directory to update")
	fs.Var(&frozenFileArgs, "frozen-file", "file name never regenerated (repeatable)")
	fs.Var(&freezeArgs, "freeze", "name=version entry frozen within a regenerated file; repeat to freeze several versions of the same file (same single-version syntax as the extract subcommand)")
	if err := fs.Parse(args); err != nil {
		failf("%v", err)
	}
	if generated == "" || target == "" {
		failf("splice requires --generated and --target")
	}

	frozenFiles := map[string]bool{}
	for _, f := range frozenFileArgs {
		frozenFiles[f] = true
	}

	freeze := map[string]map[string]bool{}
	for _, arg := range freezeArgs {
		name, version, ok := strings.Cut(arg, "=")
		if !ok || name == "" || version == "" {
			failf("invalid --freeze value %q: want name=version", arg)
		}
		if strings.Contains(version, ",") {
			failf("invalid --freeze value %q: one version per --freeze, repeat the flag for more", arg)
		}
		if frozenFiles[name] {
			failf("%s is both --frozen-file and --freeze: a whole frozen file cannot also freeze a version", name)
		}
		set := freeze[name]
		if set == nil {
			set = map[string]bool{}
			freeze[name] = set
		}
		set[version] = true
	}

	if err := doSplice(generated, target, frozenFiles, freeze); err != nil {
		failf("%v", err)
	}
}

// doSplice is the testable core of the splice mode: it returns an error
// instead of exiting so unit tests can assert on every failure path.
func doSplice(generated, target string, frozenFiles map[string]bool, freeze map[string]map[string]bool) error {
	// Step 1: every frozen file must already exist in target. Never read
	// or write it, even if controller-gen emitted a (partial) counterpart.
	for f := range frozenFiles {
		if _, err := os.Stat(filepath.Join(target, f)); err != nil {
			return fmt.Errorf("frozen file %s does not exist in target %s", f, target)
		}
	}

	// Step 2: every non-frozen target file must have generated output, or
	// its Go type vanished without anyone updating the frozen-file list.
	targetFiles, err := yamlFiles(target)
	if err != nil {
		return fmt.Errorf("reading target dir: %w", err)
	}
	for _, f := range targetFiles {
		if frozenFiles[f] {
			continue
		}
		if _, err := os.Stat(filepath.Join(generated, f)); err != nil {
			return fmt.Errorf("retained CRD %s has no generated output; its Go type was removed", f)
		}
	}

	generatedFiles, err := yamlFiles(generated)
	if err != nil {
		return fmt.Errorf("reading generated dir: %w", err)
	}

	// Step 3: reject unusable --freeze keys before writing anything, so a typo
	// or a stale entry cannot drop a frozen version on its way to failing.
	generatedSet := map[string]bool{}
	for _, f := range generatedFiles {
		generatedSet[f] = true
	}
	for f := range freeze {
		if frozenFiles[f] {
			return fmt.Errorf("--freeze names %s, which is also a --frozen-file; a file is either frozen whole or spliced per version", f)
		}
		if !generatedSet[f] {
			return fmt.Errorf("--freeze names %s, which has no generated counterpart in %s", f, generated)
		}
	}

	// Step 4: write every non-frozen generated file, splicing in frozen
	// versions where named. New kinds with no --freeze entry just copy.
	for _, f := range generatedFiles {
		if frozenFiles[f] {
			continue
		}
		frozenVersions := freeze[f]
		if len(frozenVersions) == 0 {
			if err := copyFile(filepath.Join(generated, f), filepath.Join(target, f)); err != nil {
				return fmt.Errorf("copying %s: %w", f, err)
			}
			continue
		}
		if err := spliceVersions(generated, target, f, frozenVersions); err != nil {
			return err
		}
	}
	return nil
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	var target, freeze string
	fs.StringVar(&target, "target", "", "checked-in CRD directory")
	fs.StringVar(&freeze, "freeze", "", "name=version to extract")
	if err := fs.Parse(args); err != nil {
		failf("%v", err)
	}
	if target == "" || freeze == "" {
		failf("extract requires --target and --freeze name=version")
	}
	name, version, ok := strings.Cut(freeze, "=")
	if !ok || name == "" || version == "" {
		failf("invalid --freeze value %q: want name=version", freeze)
	}
	out, err := doExtract(target, name, version)
	if err != nil {
		failf("%v", err)
	}
	fmt.Println(out)
}

// doExtract is the testable core of the extract mode.
func doExtract(target, name, version string) (string, error) {
	crd, err := parseCRD(filepath.Join(target, name))
	if err != nil {
		return "", err
	}
	entry, ok := crd.entries[version]
	if !ok {
		return "", fmt.Errorf("version %q not found in %s", version, name)
	}
	return strings.Join(entry, "\n"), nil
}

// crdFile is a CRD YAML file split into its fixed header (everything up to
// and including the "  versions:" line) and its spec.versions[] entries,
// keyed by version name.
type crdFile struct {
	header  []string
	entries map[string][]string
}

func parseCRD(path string) (*crdFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	lines := splitLines(string(data))

	headerEnd := -1
	for i, l := range lines {
		if l == versionsHeaderLine {
			headerEnd = i
			break
		}
	}
	if headerEnd == -1 {
		return nil, fmt.Errorf("%s: no %q line found", path, versionsHeaderLine)
	}
	header := lines[:headerEnd+1]
	rest := lines[headerEnd+1:]

	var starts []int
	for i, l := range rest {
		if strings.HasPrefix(l, entryPrefix) {
			starts = append(starts, i)
		}
	}

	entries := map[string][]string{}
	for i, start := range starts {
		end := len(rest)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		entry := rest[start:end]
		name, err := entryVersionName(entry)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, dup := entries[name]; dup {
			return nil, fmt.Errorf("%s: duplicate version %q", path, name)
		}
		entries[name] = entry
	}
	return &crdFile{header: header, entries: entries}, nil
}

// entryVersionName returns the single "name: <version>" value inside a
// versions entry; it errors if the entry has zero or more than one.
func entryVersionName(entry []string) (string, error) {
	var name string
	count := 0
	for _, l := range entry {
		if strings.HasPrefix(l, nameLinePrefix) {
			name = strings.TrimPrefix(l, nameLinePrefix)
			count++
		}
	}
	if count != 1 {
		return "", fmt.Errorf("versions entry has %d %q lines, want exactly 1", count, nameLinePrefix)
	}
	return name, nil
}

// spliceVersions writes target/name from generated/name, replacing the
// entries named in frozenVersions with the target's own bytes for those
// versions. Every other version comes from the generated file, including
// versions that do not exist in the target yet.
func spliceVersions(generatedDir, targetDir, name string, frozenVersions map[string]bool) error {
	gen, err := parseCRD(filepath.Join(generatedDir, name))
	if err != nil {
		return err
	}
	tgt, err := parseCRD(filepath.Join(targetDir, name))
	if err != nil {
		return err
	}

	result := map[string][]string{}
	for v, lines := range gen.entries {
		if frozenVersions[v] {
			continue // the target's bytes win for frozen versions
		}
		result[v] = lines
	}
	for v := range frozenVersions {
		lines, ok := tgt.entries[v]
		if !ok {
			return fmt.Errorf("%s: frozen version %q not found in target", name, v)
		}
		result[v] = lines
	}

	versions := make([]string, 0, len(result))
	for v := range result {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	out := append([]string{}, gen.header...)
	for _, v := range versions {
		out = append(out, result[v]...)
	}
	return writeCRD(filepath.Join(targetDir, name), []byte(strings.Join(out, "\n")+"\n"))
}

// writeCRD writes a generated manifest and forces mode 0644 to match the
// checked-in CRDs. os.WriteFile applies the umask when it creates a file, so
// under a restrictive umask the mode argument alone is not enough.
func writeCRD(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil { //nolint:gosec
		return err
	}
	return os.Chmod(path, 0o644)
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// src/dst are built from --generated/--target dir listings, not untrusted input.
	return writeCRD(dst, data)
}

// splitLines splits s on "\n" without producing a trailing empty element
// for a well-formed text file ending in a single newline.
func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
