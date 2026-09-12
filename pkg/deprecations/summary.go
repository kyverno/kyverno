package deprecations

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrLegacyPoliciesPresent is a sentinel error used to report, via an L0
// error log, that legacy kyverno.io policy custom resources still exist in
// the cluster. It carries no dynamic information; the accompanying log
// message (built with LegacyPolicySummary) carries the counts and kinds.
var ErrLegacyPoliciesPresent = errors.New("legacy kyverno.io policy resources present")

// knownNonZeroSortedKinds returns the kinds that both have a non-zero count
// and are recognized legacy kyverno.io kinds (i.e. BuildKindWarning has an
// entry for them), sorted for deterministic output. This is the single
// source of truth for "which kinds are worth reporting", shared by
// LegacyPolicySummary (the L0 log) and LegacyPolicyEventNote (the Warning
// Event), so the two can never disagree about which kinds to include -- for
// example if a caller's counters map ever includes a kind that predates or
// falls outside the deprecations table.
func knownNonZeroSortedKinds(counts map[string]int) []string {
	kinds := make([]string, 0, len(counts))
	for kind, count := range counts {
		if count <= 0 {
			continue
		}
		if _, ok := BuildKindWarning("kyverno.io", "", kind); !ok {
			continue
		}
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// LegacyPolicySummary builds a human-readable, deterministic summary of the
// legacy kyverno.io kinds that still have at least one existing custom
// resource, for use in the startup L0 log. It states the removal notice and
// the migration URL once in a preamble, then lists each kind as
// "Kind=count (→ replacement)", drawing the replacement names and the URL from
// the same deprecations table as the admission-time warnings (via
// BuildKindWarning and MigrationGuideURL) so the guidance can't drift. ok is
// false when none of the counted kinds has a non-zero count, in which case
// message is empty and callers should not log or emit anything.
func LegacyPolicySummary(counts map[string]int) (message string, ok bool) {
	kinds := knownNonZeroSortedKinds(counts)
	if len(kinds) == 0 {
		return "", false
	}

	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		// ok is guaranteed true here: kinds only contains kinds BuildKindWarning
		// already recognized, in knownNonZeroSortedKinds above. Use the bare
		// Replacement (not the full Message) so the removal notice and migration
		// URL appear once in the preamble below, not repeated per kind.
		warning, _ := BuildKindWarning("kyverno.io", "", kind)
		parts = append(parts, fmt.Sprintf("%s=%d (→ %s)", kind, counts[kind], warning.Replacement))
	}
	return fmt.Sprintf(
		"legacy kyverno.io policy resources present and should be migrated to policies.kyverno.io before support is removed, see %s; %s",
		MigrationGuideURL,
		strings.Join(parts, ", "),
	), true
}

// LegacyPolicyEventNote builds a short summary of the legacy kyverno.io kinds
// that still have at least one existing custom resource, suitable for a
// Kubernetes Event Note. Unlike LegacyPolicySummary, it stays well under the
// 1024-byte Note limit ("K8s events" API, see client-go's
// tools/record/util.ValidateEventType and the emitEvent truncation in
// pkg/event/controller.go) even when every legacy kind is present, by listing
// only kind=count pairs instead of the full per-kind deprecation message. It
// draws from the same knownNonZeroSortedKinds as LegacyPolicySummary, so the
// event can never list a kind the log omitted (or vice versa). ok is false
// when none of the counted kinds has a non-zero, recognized count.
func LegacyPolicyEventNote(counts map[string]int) (message string, ok bool) {
	kinds := knownNonZeroSortedKinds(counts)
	if len(kinds) == 0 {
		return "", false
	}

	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", kind, counts[kind]))
	}
	return fmt.Sprintf(
		"legacy kyverno.io policy resources found (%s); migrate to policies.kyverno.io before support for them is removed, see %s",
		strings.Join(parts, ", "), MigrationGuideURL,
	), true
}
