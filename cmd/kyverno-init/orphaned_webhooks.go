package main

import (
	"context"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// orphanedWebhookConfigCheckTimeout bounds the direct Get calls plus the Deployment lookup and
// Event creation below, so a slow or unreachable API server can't block kyverno-init -- and
// therefore the main container it gates -- forever. A timeout is treated like any other
// failure here: logged, never fatal.
const orphanedWebhookConfigCheckTimeout = 30 * time.Second

// checkOrphanedWebhookConfigs checks for ValidatingWebhookConfiguration/
// MutatingWebhookConfiguration objects left over from an old Kyverno install that no current
// code path creates or manages (see deprecations.OrphanedValidatingWebhookConfigNames/
// OrphanedMutatingWebhookConfigNames for the specific names and why each one is dead). When any
// are found, it emits a single L0 error log and a single aggregate Warning Event attached to
// the admission-controller Deployment, with remediation guidance.
//
// This is purely observational and non-destructive: it never deletes anything, matching
// "legacy CRDs/types remain served" for 1.20 -- these webhook configs, unlike legacy policy
// custom resources, have no replacement to migrate to and can simply be deleted by the
// operator once confirmed unneeded. It never fails startup: every error, including a timeout,
// is logged and the function returns.
func checkOrphanedWebhookConfigs(ctx context.Context, setup internal.SetupResult) {
	logger := logging.WithName("kyvernopre/orphaned-webhook-configs")

	ctx, cancel := context.WithTimeout(ctx, orphanedWebhookConfigCheckTimeout)
	defer cancel()

	found, err := findOrphanedWebhookConfigs(ctx, setup)
	if err != nil {
		// The result is partial when a checker fails, so it is not presented as
		// authoritative: skip the aggregate summary log and Warning Event below, which
		// would otherwise under-report orphaned webhook configurations still present.
		logger.Error(err, "failed to check for orphaned webhook configurations (possibly a timeout, will retry on next startup)")
		return
	}

	message, ok := deprecations.OrphanedWebhookConfigSummary(found)
	if !ok {
		return
	}

	// always-on L0 error, regardless of verbosity
	logger.Error(deprecations.ErrOrphanedWebhookConfigsPresent, message)

	// The Event Note has its own, much shorter, size limit than a log message, so it
	// gets its own terse summary rather than reusing the verbose L0 log message.
	if note, ok := deprecations.OrphanedWebhookConfigEventNote(found); ok {
		emitDeprecationEvent(ctx, logger, setup, event.OrphanedWebhookConfigPresent, note)
	}
}

// findOrphanedWebhookConfigs is a thin adapter from the typed client to
// deprecations.FindOrphanedWebhookConfigs, the shared pure decision core.
func findOrphanedWebhookConfigs(ctx context.Context, setup internal.SetupResult) ([]string, error) {
	checkers := map[string]deprecations.WebhookExistenceChecker{}
	for _, name := range deprecations.OrphanedValidatingWebhookConfigNames {
		checkers[name] = func() (bool, error) {
			_, err := setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return true, nil
		}
	}
	for _, name := range deprecations.OrphanedMutatingWebhookConfigNames {
		checkers[name] = func() (bool, error) {
			_, err := setup.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return true, nil
		}
	}
	return deprecations.FindOrphanedWebhookConfigs(checkers)
}
