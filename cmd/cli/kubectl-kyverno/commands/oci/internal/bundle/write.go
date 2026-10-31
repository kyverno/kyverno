package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
)

// Write serializes a validated Bundle into the format 1.0 v1.Image: the deterministic tar+gzip
// content layer (bundle-spec.md section 5), the JSON config blob (section 6), and the manifest
// annotations (section 4). d is the normalized bundle descriptor; a nil d is a push with no
// descriptor, so Write applies the no-descriptor defaults from section 6: only formatVersion,
// resources[], and sets[] (assigned by kind) are populated, and name, version, description,
// kyvernoVersion, source, and created — and their matching annotations — are omitted. Write
// does not call Validate; the caller validates first.
func Write(b *Bundle, d *Descriptor) (v1.Image, error) {
	layer, err := buildContentLayer(b.Files)
	if err != nil {
		return nil, err
	}

	sets, assignment := assignSets(b.Documents, d)

	created, err := resolveCreated(d)
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceEntry, 0, len(b.Documents))
	for _, doc := range b.Documents {
		resources = append(resources, ResourceEntry{
			APIVersion:    doc.APIVersion,
			Kind:          doc.Kind,
			Namespace:     doc.Namespace,
			Name:          doc.Name,
			Path:          doc.Path,
			DocumentIndex: doc.Index,
			Digest:        doc.Digest,
			Set:           assignment[identityKey(doc.Kind, doc.Namespace, doc.Name)],
		})
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Path != resources[j].Path {
			return resources[i].Path < resources[j].Path
		}
		return resources[i].DocumentIndex < resources[j].DocumentIndex
	})

	cfg := Config{
		FormatVersion: internal.FormatVersion,
		Sets:          sets,
		Resources:     resources,
		Created:       created,
	}
	if d != nil {
		cfg.Name = d.Name
		cfg.Version = d.Version
		cfg.Description = d.Description
		cfg.KyvernoVersion = d.KyvernoVersion
		cfg.Source = d.Source
	}

	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	annotations := buildAnnotations(cfg, d)

	return newImage(layer, configBytes, annotations)
}

// identityKey is the Kind/namespace/name identity string used to key a set assignment.
func identityKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// assignSets assigns every document to a set. Every document first gets the by-kind default
// (bundle-spec.md section 6): PolicyException resources go to exceptions, every other supported
// kind goes to policies. A descriptor's per-resource overrides (d.Sets) are then applied on top,
// for any resource they actually name that is present in this bundle. This order — default
// first, override second, and an override that names a resource not in the bundle is skipped
// rather than creating a dangling assignment — is what guarantees every resources[] entry always
// has a non-empty `set` naming an entry in sets[] (section 6), even once #17663 starts passing a
// Descriptor whose Sets don't happen to cover every resource. A set with no member is omitted
// (section 7, "every sets[] entry SHOULD be named by at least one resources[] entry").
func assignSets(documents []Document, d *Descriptor) ([]ConfigSet, map[string]string) {
	assignment := make(map[string]string, len(documents))
	setType := map[string]string{}
	var setOrder []string
	addSet := func(name, typ string) {
		if _, ok := setType[name]; !ok {
			setType[name] = typ
			setOrder = append(setOrder, name)
		}
	}

	for _, doc := range documents {
		key := identityKey(doc.Kind, doc.Namespace, doc.Name)
		set, typ := "policies", "policies"
		if doc.Kind == "PolicyException" {
			set, typ = "exceptions", "exceptions"
		}
		assignment[key] = set
		addSet(set, typ)
	}

	if d != nil {
		for _, sa := range d.Sets {
			if sa.Set == "" {
				// An empty Set is not a valid override; every resources[] entry MUST have a
				// non-empty set (bundle-spec.md section 6). Skip rather than assign an empty
				// set name — #17663 owns producing a well-formed Descriptor, but this function
				// still must not emit an invalid config if it's ever handed a malformed one.
				continue
			}
			key := identityKey(sa.Kind, sa.Namespace, sa.Name)
			if _, ok := assignment[key]; !ok {
				// sa names a resource that isn't in this bundle; nothing to reassign.
				continue
			}
			assignment[key] = sa.Set
			addSet(sa.Set, sa.Type)
		}
	}

	used := make(map[string]bool, len(assignment))
	for _, s := range assignment {
		used[s] = true
	}
	var sets []ConfigSet
	for _, name := range setOrder {
		if used[name] {
			sets = append(sets, ConfigSet{Name: name, Type: setType[name]})
		}
	}
	return sets, assignment
}

// resolveCreated returns the config's created field, RFC 3339. It's omitted (empty string)
// unless the descriptor sets spec.created or the environment sets SOURCE_DATE_EPOCH, which is
// what keeps a default push's config digest reproducible (bundle-spec.md sections 4 and 6).
func resolveCreated(d *Descriptor) (string, error) {
	if d != nil && d.Created != nil {
		return *d.Created, nil
	}
	v, ok := os.LookupEnv("SOURCE_DATE_EPOCH")
	if !ok || v == "" {
		return "", nil
	}
	sec, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parsing SOURCE_DATE_EPOCH %q: %w", v, err)
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339), nil
}

// reservedAnnotationKeys are the io.kyverno.bundle.* and org.opencontainers.image.* keys a
// writer computes itself; a descriptor's spec.annotations MUST NOT override any of them (writer
// MUST 14). Every key buildAnnotations below can set belongs in this set: the strip in
// buildAnnotations must have one entry here per annotation buildAnnotations computes, or a new
// writer-computed key added there without a matching entry here would reopen this exact bug.
var reservedAnnotationKeys = map[string]bool{
	internal.AnnotationFormatVersion:  true,
	internal.AnnotationName:           true,
	internal.AnnotationVersion:        true,
	internal.AnnotationKyvernoVersion: true,
	internal.AnnotationOCISource:      true,
	internal.AnnotationOCIRevision:    true,
	internal.AnnotationOCICreated:     true,
	internal.AnnotationOCIVersion:     true,
	internal.AnnotationOCITitle:       true,
	internal.AnnotationOCIDescription: true,
}

// buildAnnotations builds the manifest annotations from the config: io.kyverno.bundle.* and the
// relevant org.opencontainers.image.* keys, kept in agreement with the config (bundle-spec.md
// section 4, writer MUST 15). A reserved key here is always the writer-computed value; a
// descriptor's spec.annotations is stripped of every reserved key before it's merged, so it
// never overrides one (writer MUST 14) — including when the config leaves that field empty and
// the writer-computed block below has nothing to overwrite it with. Stripping first, rather than
// only overwriting a reserved key when the config populates it, is what keeps a descriptor from
// forging, for example, io.kyverno.bundle.version on a bundle with no descriptor-supplied
// version at all: without the config to draw from, an overwrite-only guard has nothing to
// overwrite with, and the descriptor's spoofed value would otherwise survive untouched.
func buildAnnotations(cfg Config, d *Descriptor) map[string]string {
	ann := map[string]string{}
	if d != nil {
		for k, v := range d.Annotations {
			if reservedAnnotationKeys[k] {
				continue
			}
			ann[k] = v
		}
	}

	ann[internal.AnnotationFormatVersion] = cfg.FormatVersion
	if cfg.Name != "" {
		ann[internal.AnnotationName] = cfg.Name
		ann[internal.AnnotationOCITitle] = cfg.Name
	}
	if cfg.Version != "" {
		ann[internal.AnnotationVersion] = cfg.Version
		ann[internal.AnnotationOCIVersion] = cfg.Version
	}
	if cfg.Description != "" {
		ann[internal.AnnotationOCIDescription] = cfg.Description
	}
	if cfg.KyvernoVersion != "" {
		ann[internal.AnnotationKyvernoVersion] = cfg.KyvernoVersion
	}
	if cfg.Source != nil {
		if cfg.Source.URL != "" {
			ann[internal.AnnotationOCISource] = cfg.Source.URL
		}
		if cfg.Source.Revision != "" {
			ann[internal.AnnotationOCIRevision] = cfg.Source.Revision
		}
	}
	if cfg.Created != "" {
		ann[internal.AnnotationOCICreated] = cfg.Created
	}

	return ann
}
