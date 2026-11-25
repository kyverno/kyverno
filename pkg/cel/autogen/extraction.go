package autogen

import (
	"slices"
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
// Discovery matches resource names case-insensitively, so a Kind name such as
// "Deployment" resolves there too. Such a name is not a resource name: copying
// it into the target verbatim produces a rule that never matches (see #18833),
// so only the canonical plural resource name is accepted and anything else
// returns nil.
func RESTMapperBareNameResolver(mapper meta.RESTMapper) func(name string) *Config {
	return func(name string) *Config {
		gvr := schema.GroupVersionResource{Resource: name}
		gvk, err := mapper.KindFor(gvr)
		if err != nil {
			return nil
		}
		canonical, err := mapper.ResourceFor(gvr)
		if err != nil || canonical.Resource != name {
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
// where a cluster is available. Admission does not validate controller names,
// so an already-stored value, or one passed by a caller that does not run
// admission validation, can reach this function unchecked; step (3) is what
// lets such a bare name still resolve for an already-stored policy. A name
// that matches none of these steps is silently ignored and generates no rule,
// so a Kind name such as "Deployment" is admitted and leaves the policy with
// no controller rule for it (see #18833).
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

// ResolveMutatingConfig resolves a controller name for MutatingPolicy autogen.
// It is ResolveConfig, except that an explicit canonical
// "<resource>.<version>.<group>" form naming a built-in controller (e.g.
// "deployments.v1.apps") resolves to that controller's ConfigsMap entry, the
// same one its bare name resolves to. Extraction-mode mutation is not
// implemented and the MutatingPolicy engine skips it outright, so leaving such
// a name on the extraction path would silently never mutate the built-in
// workload. ValidatingPolicy and ImageValidatingPolicy do support extraction
// targets and keep using ResolveConfig unchanged. MutatingPolicy autogen can
// still keep a remapped target on the extraction path when a mutation cannot be
// rewritten for the pod template yet (JSONPatch, or an expression shape it does
// not parse), so that the workload is skipped rather than mutated at the wrong
// path - see pkg/cel/policies/mpol/autogen.
func ResolveMutatingConfig(name string) *Config {
	config := ResolveConfig(name)
	if config == nil || config.ReplacementsRef != ExtractionReplacementsRef {
		return config
	}
	if builtin := ConfigsMap[config.Target.Resource]; builtin != nil &&
		builtin.Target.Group == config.Target.Group &&
		builtin.Target.Version == config.Target.Version {
		return builtin
	}
	return config
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
	if !isCanonicalResourceForVersion(resource, version, group) {
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

// isCanonicalResource reports whether resource is the canonical plural resource
// name for group. This is an offline check rather than a mapper lookup, so it
// works without a cluster: the resource must be all lowercase, and
// when group is a built-in group it must be one of that group's canonical
// plurals from ControllerNames. "Deployment.v1.apps" and "deployment.v1.apps"
// both name a resource that does not exist, and a rule generated for either can
// never match. For a group that is not built in, lowercase is the only check
// possible without discovery: a singular custom resource such as
// "jobset.v1alpha2.jobset.x-k8s.io" cannot be distinguished from a plural
// offline, so it is accepted here.
func isCanonicalResource(resource, group string) bool {
	if resource != strings.ToLower(resource) {
		return false
	}
	builtinGroup := false
	for _, name := range ControllerNames() {
		if ConfigsMap[name].Target.Group != group {
			continue
		}
		builtinGroup = true
		if resource == name {
			return true
		}
	}
	return !builtinGroup
}

// builtInHistoricalVersions lists, per built-in controller, the API versions its
// group served before the version ConfigsMap targets today. There is no offline
// API source to consult, so without this list an explicit name such as
// "cronjobs.v1beta1.batch" or "deployments.*.apps" is rejected as nonexistent
// and silently loses a controller rule that resolved, to its extraction target,
// before the version check was added. Source: the Kubernetes API versions these
// resources shipped in - apps/v1beta1 and apps/v1beta2 for the apps group, and
// batch/v1beta1 for CronJob - which is what the offline CLI matched before. A
// version that is neither the configured one, nor "*", nor listed here is still
// rejected: Jobs only ever existed in batch/v1, so "jobs.v1beta1.batch"
// generates no rule.
var builtInHistoricalVersions = map[string][]string{
	"cronjobs":     {"v1beta1"},
	"daemonsets":   {"v1beta2"},
	"deployments":  {"v1beta1", "v1beta2"},
	"replicasets":  {"v1beta2"},
	"statefulsets": {"v1beta1", "v1beta2"},
}

// isCanonicalResourceForVersion reports whether resource is canonical for the
// group and, when the group is built in, whether version is one the resource
// actually served: the version ConfigsMap configures, the "*" wildcard
// (apiVersions: ["*"]), or a historical version from builtInHistoricalVersions.
// Custom groups have no offline version source, so they retain the
// lowercase-only check from isCanonicalResource. ResolveConfig uses this
// stricter check, so an explicit form naming a built-in controller at a version
// it never had (e.g. "deployments.v2.apps") generates no rule instead of one
// that cannot match.
func isCanonicalResourceForVersion(resource, version, group string) bool {
	if !isCanonicalResource(resource, group) {
		return false
	}
	if config := ConfigsMap[resource]; config != nil && config.Target.Group == group {
		return version == "*" || version == config.Target.Version || slices.Contains(builtInHistoricalVersions[resource], version)
	}
	return true
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
