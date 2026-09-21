package deprecations

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// OrphanedValidatingWebhookConfigNames lists ValidatingWebhookConfiguration names that no
// current Kyverno code path creates or manages. A cluster can still have one of these if it
// was installed long enough ago and has been upgraded straight through without ever going
// through the (now-removed) cleanup that used to run in kyverno-init.
var OrphanedValidatingWebhookConfigNames = []string{
	// The name of what's now "kyverno-resource-validating-webhook-cfg", before it was renamed
	// in commit 1b417f42d ("changed validating webhook configuration names", Jan 2020).
	"kyverno-validating-webhook-cfg",
	// That old name's debug-mode variant.
	"kyverno-validating-webhook-cfg-debug",
	// Debug-mode variants of the current names: debug-mode webhook creation was removed
	// entirely (PR #4794, "remove error prone debug field"), so these are never created by
	// any code path today, regardless of how old the installation is.
	"kyverno-resource-validating-webhook-cfg-debug",
	"kyverno-policy-validating-webhook-cfg-debug",
}

// OrphanedMutatingWebhookConfigNames lists MutatingWebhookConfiguration names that no current
// Kyverno code path creates or manages -- see OrphanedValidatingWebhookConfigNames for why
// these can still linger in an old cluster.
var OrphanedMutatingWebhookConfigNames = []string{
	"kyverno-resource-mutating-webhook-cfg-debug",
	"kyverno-verify-mutating-webhook-cfg-debug",
	"kyverno-policy-mutating-webhook-cfg-debug",
}

// ErrOrphanedWebhookConfigsPresent is a sentinel error used to report, via an L0 error log,
// that orphaned webhook configuration objects from an old Kyverno install still exist in the
// cluster. It carries no dynamic information; the accompanying log message (built with
// OrphanedWebhookConfigSummary) carries the actual names found.
var ErrOrphanedWebhookConfigsPresent = errors.New("orphaned legacy kyverno webhook configurations present")

// WebhookExistenceChecker reports whether a specific named webhook configuration object
// currently exists in the cluster. Implementations typically wrap a Get call through a typed
// client, treating NotFound as (false, nil), for example:
//
//	func() (bool, error) {
//		_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
//		if apierrors.IsNotFound(err) {
//			return false, nil
//		}
//		return err == nil, err
//	}
type WebhookExistenceChecker func() (bool, error)

// FindOrphanedWebhookConfigs invokes each checker and returns the sorted list of names whose
// checker reported the object exists. It is pure: it takes whatever checkers the caller can
// build from its own client and returns a plain slice, with no logging, metrics, or event side
// effects.
//
// Checking continues for every name even if one checker fails; failures are aggregated and
// returned alongside whatever names were successfully confirmed, so callers can still act on
// the partial result.
func FindOrphanedWebhookConfigs(checkers map[string]WebhookExistenceChecker) ([]string, error) {
	var found []string
	var errs []error

	// iterate in a stable order so errors (when joined) are deterministic
	names := make([]string, 0, len(checkers))
	for name := range checkers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		exists, err := checkers[name]()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if exists {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	return found, errors.Join(errs...)
}

// qualifiedTargets renders each name in foundValidating/foundMutating as a kubectl
// resource/name target ("validatingwebhookconfiguration/<name>",
// "mutatingwebhookconfiguration/<name>"), sorted within each kind. A name's kind must be kept
// alongside it (never merged into one untyped list) -- a single object name only exists as one
// kind or the other, so a combined "kubectl delete validatingwebhookconfigurations,
// mutatingwebhookconfigurations <all names>" would apply every name to both resource types and
// fail with a spurious NotFound for whichever kind that particular name isn't.
func qualifiedTargets(foundValidating, foundMutating []string) []string {
	validating := append([]string(nil), foundValidating...)
	sort.Strings(validating)
	mutating := append([]string(nil), foundMutating...)
	sort.Strings(mutating)

	targets := make([]string, 0, len(validating)+len(mutating))
	for _, name := range validating {
		targets = append(targets, "validatingwebhookconfiguration/"+name)
	}
	for _, name := range mutating {
		targets = append(targets, "mutatingwebhookconfiguration/"+name)
	}
	return targets
}

// OrphanedWebhookConfigSummary builds a human-readable, deterministic summary of the orphaned
// webhook configuration objects found, for use in the startup L0 log. Unlike a legacy policy
// custom resource, these objects have no replacement to migrate to -- they're dead leftovers
// from an old install -- so the guidance is simply to delete them, and the exact remediation
// command is included inline, with each name qualified by its actual kind (kubectl's
// resource/name syntax) so the command works regardless of which kind(s) were found. ok is
// false when both foundValidating and foundMutating are empty, in which case message is empty
// and callers should not log or emit anything.
func OrphanedWebhookConfigSummary(foundValidating, foundMutating []string) (message string, ok bool) {
	targets := qualifiedTargets(foundValidating, foundMutating)
	if len(targets) == 0 {
		return "", false
	}
	return fmt.Sprintf(
		"orphaned webhook configuration(s) from an old Kyverno install found and can be safely deleted, "+
			"for example: kubectl delete %s; these are not created or used by this version of Kyverno",
		strings.Join(targets, " "),
	), true
}

// OrphanedWebhookConfigEventNote builds a short summary of the orphaned webhook configuration
// objects found, suitable for a Kubernetes Event Note. Like LegacyPolicyEventNote, it stays
// well under the 1024-byte Note limit even when every known orphaned name is present. Each name
// is qualified by its kind, for the same reason as OrphanedWebhookConfigSummary.
func OrphanedWebhookConfigEventNote(foundValidating, foundMutating []string) (message string, ok bool) {
	targets := qualifiedTargets(foundValidating, foundMutating)
	if len(targets) == 0 {
		return "", false
	}
	return fmt.Sprintf(
		"orphaned webhook configuration(s) from an old Kyverno install found (%s); these can be safely "+
			"deleted, they are not created or used by this version of Kyverno",
		strings.Join(targets, ", "),
	), true
}
