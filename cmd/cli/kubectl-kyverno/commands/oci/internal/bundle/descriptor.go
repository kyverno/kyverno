package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	yamlv2 "go.yaml.in/yaml/v2"
	"sigs.k8s.io/yaml"
)

// pathSeparator is the separator gobwas/glob stops "*" and "?" at; "**" still crosses it.
const pathSeparator = '/'

// LoadDescriptor reads root/kyverno-bundle.yaml and resolves it against documents, returning the
// normalized Descriptor Write needs. It returns (nil, nil) when root is a single file or the
// descriptor is absent: those are a push with no descriptor. documents must already have passed
// Validate, so their identities are unique.
func LoadDescriptor(root string, documents []Document) (*Descriptor, error) {
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("reading bundle root %s: %w", root, err)
	}
	if !fi.IsDir() {
		return nil, nil
	}

	path := filepath.Join(root, internal.DescriptorFilename)
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", internal.DescriptorFilename, err)
	}
	if info.IsDir() {
		// A directory literally named kyverno-bundle.yaml is ordinary archived content (Assemble
		// already walks into it), not a descriptor; treat it the same as "absent" to keep a tree
		// that pushed successfully before this change pushing the same way.
		return nil, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("descriptor %s is not a regular file (symlinks are not allowed in a bundle)", internal.DescriptorFilename)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", internal.DescriptorFilename, err)
	}

	pb, err := parseDescriptor(content)
	if err != nil {
		return nil, err
	}
	return resolveDescriptor(pb, documents)
}

// parseDescriptor strictly unmarshals exactly one non-empty document and checks apiVersion/kind.
func parseDescriptor(content []byte) (*v1alpha1.PolicyBundle, error) {
	documents, err := extyaml.SplitDocuments(content)
	if err != nil {
		return nil, fmt.Errorf("descriptor %s: splitting YAML documents: %w", internal.DescriptorFilename, err)
	}
	if len(documents) != 1 {
		return nil, fmt.Errorf("descriptor %s must contain exactly one document, found %d", internal.DescriptorFilename, len(documents))
	}

	var pb v1alpha1.PolicyBundle
	if err := yaml.UnmarshalStrict(documents[0], &pb); err != nil {
		return nil, fmt.Errorf("descriptor %s: %w", internal.DescriptorFilename, err)
	}
	const expectedAPIVersion = "cli.kyverno.io/v1alpha1"
	const expectedKind = "PolicyBundle"
	if pb.APIVersion != expectedAPIVersion || pb.Kind != expectedKind {
		return nil, fmt.Errorf("descriptor %s: expected %s %s, got %s %s", internal.DescriptorFilename, expectedAPIVersion, expectedKind, pb.APIVersion, pb.Kind)
	}
	if err := rejectCoercedScalars(documents[0]); err != nil {
		return nil, err
	}
	if err := rejectCoercedAnnotationKeys(documents[0]); err != nil {
		return nil, err
	}
	return &pb, nil
}

// rejectCoercedScalars rejects any non-string, non-null leaf under metadata.name,
// metadata.annotations, or spec. Every one of those is a string field (or a collection of
// strings) in PolicyBundle; sigs.k8s.io/yaml otherwise reformats an unquoted numeric or boolean
// scalar there silently (1.20 -> 1.2, yes -> true, 0123456 -> 42798) with no error.
//
// It converts doc with yaml.YAMLToJSON, the exact step UnmarshalStrict takes internally, instead
// of re-parsing with a different YAML decoder: sigs.k8s.io/yaml decodes through go.yaml.in/yaml/v2
// (a YAML 1.1 resolver), while go.yaml.in/yaml/v3's node API is YAML 1.2 and disagrees with it on
// booleans (yes/no/on/off) and octals; two parsers deciding this question independently is how the
// original gate missed those cases. Reusing YAMLToJSON also resolves aliases and << merge keys
// before this walk ever sees them, the same way the real decode does. It only covers values: JSON
// requires string keys, so by this point an annotation key that was itself unquoted has already
// been stringified and the coercion is invisible here; rejectCoercedAnnotationKeys covers that case
// separately, before this stringification happens.
func rejectCoercedScalars(doc []byte) error {
	jsonDoc, err := yaml.YAMLToJSON(doc)
	if err != nil {
		return fmt.Errorf("descriptor %s: %w", internal.DescriptorFilename, err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(jsonDoc, &root); err != nil {
		return fmt.Errorf("descriptor %s: %w", internal.DescriptorFilename, err)
	}

	if metadata, ok := root["metadata"].(map[string]interface{}); ok {
		if err := walkCoercedScalars("metadata.name", metadata["name"]); err != nil {
			return err
		}
		if err := walkCoercedScalars("metadata.annotations", metadata["annotations"]); err != nil {
			return err
		}
	}
	return walkCoercedScalars("spec", root["spec"])
}

// walkCoercedScalars recursively rejects any leaf under path that YAMLToJSON produced as a JSON
// number or boolean: that only happens when the author left a value unquoted that the string
// field it fills in expects as text, so YAML's implicit typing silently reformatted it.
func walkCoercedScalars(path string, v interface{}) error {
	switch t := v.(type) {
	case nil, string:
		return nil
	case map[string]interface{}:
		for k, vv := range t {
			if err := walkCoercedScalars(fmt.Sprintf("%s.%s", path, k), vv); err != nil {
				return err
			}
		}
		return nil
	case []interface{}:
		for i, vv := range t {
			if err := walkCoercedScalars(fmt.Sprintf("%s[%d]", path, i), vv); err != nil {
				return err
			}
		}
		return nil
	default:
		// Report only the field, not the already-coerced value: suggesting it back as "the quoted
		// form" would walk the author into publishing the mangled reading, not what they wrote.
		return fmt.Errorf("descriptor %s: %s is not a string; quote it so YAML doesn't read it as a number or boolean",
			internal.DescriptorFilename, path)
	}
}

// rejectCoercedAnnotationKeys rejects a non-string, non-null key anywhere in metadata.annotations
// or spec.annotations. Both are user-supplied key/value maps, so an unquoted key is as coercible as
// an unquoted value (metadata.annotations: {1.20: x} -> key "1.2"), but JSON requires string keys,
// so by the time rejectCoercedScalars sees the tree (via YAMLToJSON) a coerced key has already been
// stringified and is invisible. This decodes doc with go.yaml.in/yaml/v2 directly instead -- the
// same library sigs.k8s.io/yaml uses internally, just inspected one step earlier, before keys are
// forced to strings -- so this still isn't a second resolver deciding the same question.
func rejectCoercedAnnotationKeys(doc []byte) error {
	var root interface{}
	if err := yamlv2.Unmarshal(doc, &root); err != nil {
		return fmt.Errorf("descriptor %s: %w", internal.DescriptorFilename, err)
	}
	metadata := yamlMapValue(root, "metadata")
	if err := rejectNonStringKeys("metadata.annotations", yamlMapValue(metadata, "annotations")); err != nil {
		return err
	}
	spec := yamlMapValue(root, "spec")
	return rejectNonStringKeys("spec.annotations", yamlMapValue(spec, "annotations"))
}

// yamlMapValue returns the value for key in a go.yaml.in/yaml/v2-decoded mapping, or nil when node
// is nil, not a mapping, or has no such key. yaml.v2 decodes mappings into map[interface{}]interface{},
// so the lookup key is itself an interface{}; comparing it against a plain string key works because
// Go compares interface values by dynamic type and value.
func yamlMapValue(node interface{}, key string) interface{} {
	m, ok := node.(map[interface{}]interface{})
	if !ok {
		return nil
	}
	return m[key]
}

// rejectNonStringKeys rejects any key in node, a go.yaml.in/yaml/v2-decoded mapping, that isn't a
// string or null. It does not recurse: annotations is a flat map[string]string, so only its direct
// keys can be coerced.
func rejectNonStringKeys(path string, node interface{}) error {
	m, ok := node.(map[interface{}]interface{})
	if !ok {
		return nil
	}
	for k := range m {
		switch k.(type) {
		case nil, string:
			continue
		default:
			return fmt.Errorf("descriptor %s: %s has a key that is not a string; quote it so YAML doesn't read it as a number or boolean",
				internal.DescriptorFilename, path)
		}
	}
	return nil
}

// resolveDescriptor validates pb and builds the normalized Descriptor, including derived Sets.
func resolveDescriptor(pb *v1alpha1.PolicyBundle, documents []Document) (*Descriptor, error) {
	spec := pb.Spec

	if spec.FormatVersion != "" && spec.FormatVersion != internal.FormatVersion {
		return nil, fmt.Errorf("descriptor %s pins formatVersion %q, but this CLI writes only format version %s", internal.DescriptorFilename, spec.FormatVersion, internal.FormatVersion)
	}

	var created *string
	if spec.Created != "" {
		if _, err := time.Parse(time.RFC3339, spec.Created); err != nil {
			return nil, fmt.Errorf("descriptor %s: spec.created %q is not an RFC 3339 timestamp", internal.DescriptorFilename, spec.Created)
		}
		created = &spec.Created
	}

	var source *Source
	if spec.Source != nil && (spec.Source.URL != "" || spec.Source.Revision != "") {
		source = &Source{URL: spec.Source.URL, Revision: spec.Source.Revision}
	}

	sets, err := deriveSets(spec.Sets, documents)
	if err != nil {
		return nil, err
	}

	return &Descriptor{
		Name:           pb.Name,
		Version:        spec.Version,
		Description:    spec.Description,
		KyvernoVersion: spec.KyvernoVersion,
		Source:         source,
		Created:        created,
		Sets:           sets,
		Annotations:    spec.Annotations,
	}, nil
}

// compiledSet is one descriptor set with its patterns compiled and its reserved-name check
// already applied.
type compiledSet struct {
	name     string
	typ      string
	patterns []glob.Glob
	claims   int
}

// deriveSets applies the section-7 rules and returns one SetAssignment per claimed document.
func deriveSets(sets []v1alpha1.PolicyBundleSet, documents []Document) ([]SetAssignment, error) {
	if len(sets) == 0 {
		return nil, nil
	}

	seenNames := map[string]bool{}
	compiled := make([]*compiledSet, 0, len(sets))
	for _, s := range sets {
		if s.Name == "" {
			return nil, fmt.Errorf("descriptor %s: set has no name", internal.DescriptorFilename)
		}
		if seenNames[s.Name] {
			return nil, fmt.Errorf("descriptor %s: duplicate set name %q", internal.DescriptorFilename, s.Name)
		}
		seenNames[s.Name] = true

		if s.Type != "policies" && s.Type != "exceptions" {
			return nil, fmt.Errorf(`descriptor %s: set %q has type %q; must be "policies" or "exceptions"`, internal.DescriptorFilename, s.Name, s.Type)
		}
		if s.Name == "policies" && s.Type != "policies" {
			return nil, fmt.Errorf(`descriptor %s: set %q is a reserved default set name and must have type "policies", not %q`, internal.DescriptorFilename, s.Name, s.Type)
		}
		if s.Name == "exceptions" && s.Type != "exceptions" {
			return nil, fmt.Errorf(`descriptor %s: set %q is a reserved default set name and must have type "exceptions", not %q`, internal.DescriptorFilename, s.Name, s.Type)
		}
		if len(s.Paths) == 0 {
			return nil, fmt.Errorf("descriptor %s: set %q declares no paths", internal.DescriptorFilename, s.Name)
		}

		patterns := make([]glob.Glob, 0, len(s.Paths))
		for _, p := range s.Paths {
			if p == "" {
				return nil, fmt.Errorf("descriptor %s: set %q declares an empty path", internal.DescriptorFilename, s.Name)
			}
			if strings.HasPrefix(p, "/") {
				return nil, fmt.Errorf("descriptor %s: set %q path %q must be relative to the bundle root", internal.DescriptorFilename, s.Name, p)
			}
			if strings.HasPrefix(p, "./") {
				return nil, fmt.Errorf(`descriptor %s: set %q path %q must not start with "./"; paths are already relative to the bundle root`, internal.DescriptorFilename, s.Name, p)
			}
			if strings.HasSuffix(p, "/") {
				return nil, fmt.Errorf(`descriptor %s: set %q path %q must not end with "/"; use %q to match everything under a directory`, internal.DescriptorFilename, s.Name, p, p+"**")
			}
			g, err := glob.Compile(p, pathSeparator)
			if err != nil {
				return nil, fmt.Errorf("descriptor %s: set %q path %q is not a valid glob: %w", internal.DescriptorFilename, s.Name, p, err)
			}
			patterns = append(patterns, g)
		}

		compiled = append(compiled, &compiledSet{name: s.Name, typ: s.Type, patterns: patterns})
	}

	var assignments []SetAssignment
	for _, doc := range documents {
		var claiming []*compiledSet
		for _, cs := range compiled {
			matched := false
			for _, p := range cs.patterns {
				if p.Match(doc.Path) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			// Type is a per-document filter: a "policies" set claims every matched document
			// except PolicyException; an "exceptions" set claims only matched PolicyException
			// documents.
			fitsType := (cs.typ == "exceptions") == (doc.Kind == "PolicyException")
			if !fitsType {
				continue
			}
			claiming = append(claiming, cs)
		}

		switch len(claiming) {
		case 0:
			// No fitting claim: falls back to the by-kind default set in assignSets.
			continue
		case 1:
			claiming[0].claims++
			assignments = append(assignments, SetAssignment{
				Kind: doc.Kind, Namespace: doc.Namespace, Name: doc.Name,
				Set: claiming[0].name, Type: claiming[0].typ,
			})
		default:
			var names []string
			for _, cs := range claiming {
				names = append(names, cs.name)
			}
			return nil, fmt.Errorf("descriptor %s: resource %s %q at %s is claimed by more than one set: %s",
				internal.DescriptorFilename, doc.Kind, doc.Name, doc.Path, strings.Join(names, ", "))
		}
	}

	for _, cs := range compiled {
		if cs.claims == 0 {
			return nil, fmt.Errorf("descriptor %s: set %q (type %q) claims no indexed document; check its paths", internal.DescriptorFilename, cs.name, cs.typ)
		}
	}

	return assignments, nil
}
