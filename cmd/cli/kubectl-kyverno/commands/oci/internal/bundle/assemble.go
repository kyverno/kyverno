package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"sigs.k8s.io/yaml"
)

// typeMeta is the subset of a document's fields Assemble reads directly, before any document is
// handed to a resource loader (bundle-spec.md section 5, "What a writer archives").
type typeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

func (t typeMeta) group() string {
	if i := strings.IndexByte(t.APIVersion, '/'); i >= 0 {
		return t.APIVersion[:i]
	}
	return ""
}

// Assemble walks root and builds a Bundle: every regular .yaml/.yml file's original bytes for
// the content layer, and every non-empty document's identity, path, documentIndex, and digest
// for the resource index. It applies the archive rules from bundle-spec.md section 5 (hidden
// files skipped, the reserved descriptor excluded at the root and rejected elsewhere,
// cli.kyverno.io documents and v1 List documents rejected everywhere) before any document
// reaches policy.Load, which Assemble still calls per file to produce the typed objects Validate
// and Write need.
func Assemble(root string) (*Bundle, error) {
	root = filepath.Clean(root)
	// Resolve a symlinked root (or a symlink in one of its parents) to its real path before
	// walking. filepath.WalkDir Lstats only the root argument itself: given a symlink pointing
	// at a directory, Lstat reports a symlink, not a directory, so WalkDir never descends into
	// it and Assemble would otherwise report the misleading "yields zero resources" for a root
	// that, once resolved, holds valid content. This does not weaken the symlink rule for
	// entries inside the tree: assembleFile still Lstats every candidate file itself and rejects
	// a non-regular one, so only the root's own symlink-ness is transparent, not any file under
	// it.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("reading bundle root %s: %w", root, err)
	}
	root = resolved
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("reading bundle root %s: %w", root, err)
	}

	b := &Bundle{
		Files:   map[string][]byte{},
		Results: &policy.LoaderResults{},
	}

	// Single-file input archives that one file at the root (bundle-spec.md section 5). It must
	// clear the same hidden-name and .yaml/.yml-extension gate the directory branch below
	// applies to every candidate it finds, and for the same reason: policy.Load's own directory
	// walker (policy/load.go's fsLoad) silently returns an empty, error-free LoaderResults for a
	// hidden name or a non-.yaml/.yml extension. Without this gate here, a single-file input
	// like ./policy.json or ./.policy.yaml would sail through assembleFile (which archives and
	// indexes it from its own raw document scan) while policy.Load contributed nothing — so
	// Validate would have no CEL compilation or kind/version check to run for it (writer MUST 3,
	// 5, 12), and the resulting bundle would still fail Read, because Read's own Assemble call
	// runs the directory branch on the extracted tree and would exclude the very entry that the
	// single-file branch had, wrongly, accepted.
	if !fi.IsDir() {
		name := filepath.Base(root)
		if strings.HasPrefix(name, ".") {
			return nil, fmt.Errorf("input file %s is hidden; only a non-hidden .yaml or .yml file is a supported single-file bundle input", root)
		}
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			return nil, fmt.Errorf("input file %s is not a .yaml or .yml file; only .yaml and .yml files are supported bundle input", root)
		}
		if err := assembleFile(root, name, true, b); err != nil {
			return nil, err
		}
		if len(b.Documents) == 0 {
			return nil, fmt.Errorf("input %s yields zero resources", root)
		}
		return b, nil
	}

	var relPaths []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()

		// Hidden entries are invisible regardless of symlink-ness, the same way a hidden regular
		// file or directory is: bundle-spec.md section 5 lists hidden paths as excluded, "not an
		// error", and that rule carries no exception for a hidden name that happens to be a
		// symlink. This must run before the symlink check below — a harmless ./.venv -> /opt/venv
		// is exactly as invisible as any other hidden directory, and old push (via policy.Load's
		// os.Stat-based hidden check, which keys off the given path's own name) tolerated it too,
		// so rejecting it here would be a new, gratuitous failure the format never asked for.
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			// A non-hidden symlink is rejected only when it could actually carry bundle content
			// that a naive skip would silently lose: either its own name has a .yaml/.yml
			// extension (a real file there would be indexed content, and writer MUST 8 requires
			// rejecting a symlink standing in for one), or it resolves to a directory (the
			// round-2 MAJOR: fs.DirEntry.Type() reports a symlink's own type bit, not the type
			// of whatever it points at, so d.IsDir() is false even for a symlink to a directory;
			// without this check, a symlinked directory whose own name has no recognized
			// extension, for example common -> ../shared, falls through both the directory
			// branch, never descended into, and the extension branch, no recognized extension,
			// and is silently dropped with no error — the bundle would simply be missing
			// everything under it). Anything else — a symlink to a regular non-YAML file, or a
			// broken symlink whose name isn't .yaml/.yml — carries no bundle content either way,
			// so it's excluded the same as any other non-YAML path (section 5's "not an error"
			// list), matching old push, which read straight through symlinks with no rejection
			// of its own for this case.
			ext := filepath.Ext(name)
			if ext == ".yaml" || ext == ".yml" {
				return fmt.Errorf("%s is a symlink; symlinks are not allowed in a bundle", rel)
			}
			if target, statErr := os.Stat(path); statErr == nil && target.IsDir() {
				return fmt.Errorf("%s is a symlink to a directory; symlinks are not allowed in a bundle", rel)
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			// Not a YAML file: invisible to the writer's document-loading path entirely.
			return nil
		}
		relPaths = append(relPaths, rel)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking bundle root %s: %w", root, err)
	}

	sort.Strings(relPaths)

	for _, rel := range relPaths {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		atRoot := !strings.Contains(rel, "/")
		if err := assembleFile(abs, rel, atRoot, b); err != nil {
			return nil, err
		}
	}

	if len(b.Documents) == 0 {
		return nil, fmt.Errorf("input tree %s yields zero resources", root)
	}

	return b, nil
}

// assembleFile validates and records one candidate file. rel is its bundle-root-relative,
// forward-slash path; atRoot reports whether it sits directly at the bundle root.
func assembleFile(abs, rel string, atRoot bool, b *Bundle) error {
	if filepath.Base(rel) == internal.DescriptorFilename {
		if atRoot {
			// The descriptor is a reserved, bundle-root-only input: excluded, not archived,
			// never handed to the resource loader.
			return nil
		}
		return fmt.Errorf("%s is a reserved bundle descriptor filename and must be at the bundle root, not %s", internal.DescriptorFilename, rel)
	}

	info, err := os.Lstat(abs)
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file (symlinks and non-regular entries such as device files are not allowed in a bundle)", rel)
	}

	content, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}

	documents, err := extyaml.SplitDocuments(content)
	if err != nil {
		return fmt.Errorf("splitting YAML documents in %s: %w", rel, err)
	}

	for i, doc := range documents {
		var tm typeMeta
		if err := yaml.Unmarshal(doc, &tm); err != nil {
			return fmt.Errorf("reading %s, document %d: %w", rel, i, err)
		}
		if tm.group() == "cli.kyverno.io" {
			return fmt.Errorf("%s, document %d: cli.kyverno.io resources (kind %q) are CLI configuration, not bundle content, and must not appear in a bundle", rel, i, tm.Kind)
		}
		if tm.APIVersion == "v1" && tm.Kind == "List" {
			return fmt.Errorf("%s, document %d: v1 List documents are not supported in a bundle", rel, i)
		}

		sum := sha256.Sum256(doc)
		b.Documents = append(b.Documents, Document{
			Path:       rel,
			Index:      i,
			Bytes:      doc,
			Digest:     "sha256:" + hex.EncodeToString(sum[:]),
			APIVersion: tm.APIVersion,
			Kind:       tm.Kind,
			Namespace:  tm.Metadata.Namespace,
			Name:       tm.Metadata.Name,
		})
	}

	// Every regular .yaml/.yml file is archived with its original bytes, even one whose only
	// documents are empty (blank lines or #-comment lines): bundle-spec.md section 5 says a
	// writer archives "every regular .yaml or .yml file ... with its original bytes", and lists
	// only hidden files, non-YAML files, and the root descriptor as exclusions. An all-empty
	// file contributes no resources[] entry (there's nothing to index), but it still round-trips
	// through the content layer like any other file.
	b.Files[rel] = content

	if len(documents) == 0 {
		return nil
	}

	results, err := policy.Load(nil, "", false, abs)
	if err != nil {
		return fmt.Errorf("loading %s: %w", rel, err)
	}
	// validateResults' checks (CEL compilation, kind and version rejection, exception-reference
	// resolution) only ever see what policy.Load merges here; they have no independent view of
	// documents. That's safe only as long as a file assembleFile just parsed len(documents) > 0
	// non-empty documents out of always makes policy.Load produce *something* for it — a typed
	// object, or at least a NonFatalError. The single-file gate above is what keeps a hidden or
	// non-.yaml/.yml path from reaching this point at all, but this check is the invariant itself,
	// not just its one known cause: any other input shape for which Assemble's own raw document
	// scan succeeds while policy.Load silently contributes nothing would otherwise let a
	// resource land in the index and the content layer without ever being validated.
	if loaderResultCount(results) == 0 {
		return fmt.Errorf("%s: found %d document(s) but policy.Load produced no results for it; only .yaml and .yml files reachable by the CLI's own loader are supported", rel, len(documents))
	}
	b.Results.Merge(results)

	return nil
}

// loaderResultCount sums every policy.LoaderResults field validateResults inspects (and
// NonFatalErrors, since that's itself a rejection validateResults acts on), so assembleFile can
// tell "policy.Load saw this file and rejected or accepted something in it" apart from "policy.Load
// silently produced nothing for it".
func loaderResultCount(r *policy.LoaderResults) int {
	if r == nil {
		return 0
	}
	return len(r.Policies) + len(r.PolicyExceptions) + len(r.PolicyCelExceptions) +
		len(r.VAPs) + len(r.VAPBindings) + len(r.MAPs) + len(r.MAPBindings) +
		len(r.ValidatingPolicies) + len(r.EnvoyPolicies) + len(r.HTTPPolicies) +
		len(r.ImageValidatingPolicies) + len(r.GeneratingPolicies) + len(r.DeletingPolicies) +
		len(r.CleanupPolicies) + len(r.MutatingPolicies) + len(r.NonFatalErrors)
}
