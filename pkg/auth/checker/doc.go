// Package checker provides the low-level authorization primitive used by
// Kyverno's policy validation: an AuthChecker that answers a single
// (identity, resolved-resource, verb) question by issuing one Kubernetes access
// review.
//
// Two implementations wrap the two review APIs:
//   - NewSubjectChecker issues SubjectAccessReviews for a given user and groups.
//   - NewSelfChecker issues SelfSubjectAccessReviews for the caller's own identity.
//
// NewCachedChecker / NewCachedSelfChecker decorate either implementation with a
// ResultCache so repeated questions within a single validation pass are answered
// once. See ResultCache for the (short-lived, single-pass) caching contract —
// the cache has no expiry and must never be promoted to a process- or
// controller-scoped lifetime, or it would serve stale RBAC decisions.
package checker
