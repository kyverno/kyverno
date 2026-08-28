package autogen

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BareNameResolver, when set, resolves a bare controller name (one that
// doesn't parse as "<resource>.<version>.<group>") to a Config via live
// cluster discovery, so a policy author can write just "jobsets" instead of
// "jobsets.v1alpha2.jobset.x-k8s.io". It's nil by default: the CLI's offline
// apply/test commands have no cluster to ask, and ResolveConfig falls back
// to requiring the explicit format when this is unset or returns nil.
// RESTMapperBareNameResolver builds one from a live cluster's RESTMapper.
var BareNameResolver func(name string) *Config

// RESTMapperBareNameResolver resolves a bare resource name to a Config using
// a live cluster's RESTMapper. Ambiguous names (the same plural resource
// name existing in more than one API group) fail closed: mapper.KindFor
// returns an error in that case rather than guessing, and this returns nil.
func RESTMapperBareNameResolver(mapper meta.RESTMapper) func(name string) *Config {
	return func(name string) *Config {
		gvk, err := mapper.KindFor(schema.GroupVersionResource{Resource: name})
		if err != nil {
			return nil
		}
		return &Config{
			Target: policiesv1beta1.Target{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: name,
				Kind:     gvk.Kind,
			},
			ReplacementsRef: ExtractionReplacementsRef,
		}
	}
}

// KindResolver, when set, resolves a fully-qualified GroupVersionResource to
// its real Kind via live cluster discovery. It corrects
// parseCustomControllerConfig's best-effort guessKind - which gets
// irregular plurals wrong (e.g. "jobsets" -> "Jobset", not "JobSet") - and
// Target.Kind is not cosmetic: CreateMatchConditions compiles it into a CEL
// string comparison against the admitted object's real kind, so a wrong
// guess makes that precondition never match. Nil by default (no cluster),
// in which case parseCustomControllerConfig falls back to guessKind.
// RESTMapperKindResolver builds one from a live cluster's RESTMapper.
var KindResolver func(gvr schema.GroupVersionResource) (kind string, ok bool)

// RESTMapperKindResolver resolves a GroupVersionResource to its Kind using a
// live cluster's RESTMapper.
func RESTMapperKindResolver(mapper meta.RESTMapper) func(schema.GroupVersionResource) (string, bool) {
	return func(gvr schema.GroupVersionResource) (string, bool) {
		gvk, err := mapper.KindFor(gvr)
		if err != nil {
			return "", false
		}
		return gvk.Kind, true
	}
}

// ExtractionReplacementsRef groups every autogen target whose pod template is
// discovered by structural extraction at evaluation time, rather than by the
// compile-time byte-rewrite mechanism the built-in kinds use. Its Spec is
// never rewritten (there's no fixed path to rewrite to), which is why it has
// no entry in ReplacementsMap: Apply is a no-op for an unknown key, so the
// spec ends up byte-identical to the original.
const ExtractionReplacementsRef = "extract"

// ResolveConfig turns a controller name from spec.autogen.podControllers.controllers
// into a Config, trying in order: (1) ConfigsMap, for the built-in kinds,
// unchanged; (2) the explicit "<resource>.<version>.<group>" format (e.g.
// "jobsets.v1alpha2.jobset.x-k8s.io"), which needs no live cluster and so
// works from the CLI's offline apply/test commands too; (3) BareNameResolver,
// if one is set, so a bare name like "jobsets" resolves via live discovery
// where a cluster is available. A name that matches none of these is
// silently ignored, preserving the historical behavior for genuine typos.
func ResolveConfig(name string) *Config {
	if config := ConfigsMap[name]; config != nil {
		return config
	}
	if config := parseCustomControllerConfig(name); config != nil {
		return config
	}
	if BareNameResolver != nil {
		return BareNameResolver(name)
	}
	return nil
}

func parseCustomControllerConfig(name string) *Config {
	parts := strings.SplitN(name, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	resource, version, group := parts[0], parts[1], parts[2]
	if resource == "" || version == "" || group == "" {
		return nil
	}
	kind := guessKind(resource)
	if KindResolver != nil {
		if resolved, ok := KindResolver(schema.GroupVersionResource{Group: group, Version: version, Resource: resource}); ok {
			kind = resolved
		}
	}
	return &Config{
		Target: policiesv1beta1.Target{
			Group:    group,
			Version:  version,
			Resource: resource,
			Kind:     kind,
		},
		ReplacementsRef: ExtractionReplacementsRef,
	}
}

// guessKind is a best-effort fallback for when no KindResolver is available
// (e.g. the CLI's offline apply/test commands). It gets irregular plurals
// wrong - "jobsets" guesses "Jobset", not the real "JobSet" - so it's only
// ever used when live discovery can't correct it.
func guessKind(resource string) string {
	trimmed := strings.TrimSuffix(resource, "s")
	if trimmed == "" {
		return trimmed
	}
	return strings.ToUpper(trimmed[:1]) + trimmed[1:]
}
