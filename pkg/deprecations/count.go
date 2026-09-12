package deprecations

import (
	"errors"
	"sort"
)

// KindCounter returns the number of existing custom resources for one legacy
// kyverno.io kind. Implementations typically wrap a synced informer lister,
// for example:
//
//	func() (int, error) {
//		pols, err := cpolLister.List(labels.Everything())
//		return len(pols), err
//	}
type KindCounter func() (int, error)

// CountLegacyPolicies invokes each counter and returns a map of kind name to
// the number of existing custom resources of that kind. It is pure: it takes
// whatever counters the caller can build from its own listers and returns a
// plain map, with no logging, metrics, or event side effects.
//
// Counting continues for every kind even if one counter fails; failures are
// aggregated and returned alongside whatever counts were successfully
// collected, so callers can still act on the partial result.
func CountLegacyPolicies(counters map[string]KindCounter) (map[string]int, error) {
	counts := make(map[string]int, len(counters))
	var errs []error

	// iterate in a stable order so errors (when joined) are deterministic
	kinds := make([]string, 0, len(counters))
	for kind := range counters {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	for _, kind := range kinds {
		count, err := counters[kind]()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		counts[kind] = count
	}
	return counts, errors.Join(errs...)
}
