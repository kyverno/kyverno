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

// OrphanedWebhookConfigSummary builds a human-readable, deterministic summary of the orphaned
// webhook configuration objects found, for use in the startup L0 log. Unlike a legacy policy
// custom resource, these objects have no replacement to migrate to -- they're dead leftovers
// from an old install -- so the guidance is simply to delete them, and the exact remediation
// command is included inline. ok is false when found is empty, in which case message is empty
// and callers should not log or emit anything.
func OrphanedWebhookConfigSummary(found []string) (message string, ok bool) {
	if len(found) == 0 {
		return "", false
	}
	sorted := append([]string(nil), found...)
	sort.Strings(sorted)
	return fmt.Sprintf(
		"orphaned webhook configuration(s) from an old Kyverno install found and can be safely deleted, "+
			"for example: kubectl delete validatingwebhookconfigurations,mutatingwebhookconfigurations %s; "+
			"these are not created or used by this version of Kyverno",
		strings.Join(sorted, " "),
	), true
}

// OrphanedWebhookConfigEventNote builds a short summary of the orphaned webhook configuration
// objects found, suitable for a Kubernetes Event Note. Like LegacyPolicyEventNote, it stays
// well under the 1024-byte Note limit even when every known orphaned name is present.
func OrphanedWebhookConfigEventNote(found []string) (message string, ok bool) {
	if len(found) == 0 {
		return "", false
	}
	sorted := append([]string(nil), found...)
	sort.Strings(sorted)
	return fmt.Sprintf(
		"orphaned webhook configuration(s) from an old Kyverno install found (%s); these can be safely "+
			"deleted, they are not created or used by this version of Kyverno",
		strings.Join(sorted, ", "),
	), true
}
