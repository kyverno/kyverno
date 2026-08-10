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
	return &Config{
		Target: policiesv1beta1.Target{
			Group:    group,
			Version:  version,
			Resource: resource,
			// Best-effort: exact Kind casing isn't load-bearing yet since
			// webhook fine-grained matchConditions (the only current
			// consumer of Target.Kind) aren't wired for extraction targets
			// yet. Live discovery-based resolution (a follow-up pass) will
			// supply the real Kind.
			Kind: guessKind(resource),
		},
		ReplacementsRef: ExtractionReplacementsRef,
	}
}

func guessKind(resource string) string {
	trimmed := strings.TrimSuffix(resource, "s")
	if trimmed == "" {
		return trimmed
	}
	return strings.ToUpper(trimmed[:1]) + trimmed[1:]
}
