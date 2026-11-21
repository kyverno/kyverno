package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// legacyPolicyCheckTimeout bounds the metadata-list count plus the Deployment
// lookup and Event creation below, so a slow or unreachable API server can't
// block kyverno-init -- and therefore the main container it gates -- forever.
// A timeout is treated like any other failure here: logged, never fatal.
const legacyPolicyCheckTimeout = 30 * time.Second

// checkLegacyPolicies counts the legacy (non policies.kyverno.io) policy
// custom resources still present in the cluster -- ClusterPolicy, Policy,
// CleanupPolicy, ClusterCleanupPolicy, and legacy PolicyException -- using a
// metadata-only list through the metadata client, since kyverno-init runs
// once at startup, before the 30 s refresher used by the long-running
// controllers exists. When any of them are still present, it emits a single
// L0 error log and a single aggregate Warning Event attached to the
// admission-controller Deployment, with migration guidance.
//
// This is purely observational: it fires whenever legacy CRs exist,
// independent of any policy cleanup outcome or write-path enforcement
// toggle. It never fails startup: every error, including a timeout, is
// logged and the function returns.
func checkLegacyPolicies(ctx context.Context, setup internal.SetupResult) {
	logger := logging.WithName("kyvernopre/legacy-policies")

	ctx, cancel := context.WithTimeout(ctx, legacyPolicyCheckTimeout)
	defer cancel()

	counts, err := countLegacyPolicies(ctx, setup)
	if err != nil {
		// The counts are partial when a counter fails, so they are not presented as
		// authoritative: skip the aggregate summary log and Warning Event below,
		// which would otherwise under-report legacy policy resources still present.
		logger.Error(err, "failed to count legacy kyverno.io policy resources (possibly a timeout, will retry on next startup)")
		return
	}

	message, ok := deprecations.LegacyPolicySummary(counts)
	if !ok {
		return
	}

	// always-on L0 error, regardless of verbosity
	logger.Error(deprecations.ErrLegacyPoliciesPresent, message, "migrationGuide", deprecations.MigrationGuideURL)

	// The Event Note has its own, much shorter, size limit than a log message, so it
	// gets its own terse summary rather than reusing the verbose L0 log message.
	if note, ok := deprecations.LegacyPolicyEventNote(counts); ok {
		emitDeprecationEvent(ctx, logger, setup, event.LegacyPolicyPresent, note)
	}
}

// countLegacyPolicies is a thin adapter from the metadata client to
// deprecations.CountLegacyPolicies, the shared pure counting core also used
// by the admission-controller and cleanup-controller gauge callbacks.
func countLegacyPolicies(ctx context.Context, setup internal.SetupResult) (map[string]int, error) {
	return deprecations.CountLegacyPolicies(ctx, deprecations.MetadataKindCounters(
		setup.MetadataClient, "ClusterPolicy", "Policy", "CleanupPolicy", "ClusterCleanupPolicy", "PolicyException"))
}

// emitDeprecationEvent creates a single Warning Event of the given reason, with the given note,
// attached to the admission-controller Deployment. Shared by every kyverno-init startup
// diagnostic that needs to surface an observational-only finding as a Kubernetes Event (legacy
// policy custom resources, orphaned webhook configurations, ...) -- kyverno-init does not run
// the event generator/queue used by the long-running controllers, so the Event is created
// directly through the events client.
func emitDeprecationEvent(ctx context.Context, logger logr.Logger, setup internal.SetupResult, reason event.Reason, note string) {
	if setup.EventsClient == nil {
		return
	}

	namespace := config.KyvernoNamespace()
	deploymentName := config.KyvernoDeploymentName()
	deployment, err := setup.KubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "admission-controller Deployment not found, skipping event", "namespace", namespace, "name", deploymentName, "reason", reason)
			return
		}
		logger.Error(err, "failed to get admission-controller Deployment, skipping event", "namespace", namespace, "name", deploymentName, "reason", reason)
		return
	}

	hostname, _ := os.Hostname()
	reportingController := string(event.KyvernoInit)
	now := time.Now()

	// defensive: matches the Note truncation in pkg/event/controller.go's emitEvent, in case a
	// future caller passes a note without going through its own already-bounded builder.
	if len(note) > 1024 {
		note = note[0:1021] + "..."
	}

	ev := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%x", deployment.Name, now.UnixNano()),
			Namespace: namespace,
		},
		EventTime:           metav1.MicroTime{Time: now},
		ReportingController: reportingController,
		ReportingInstance:   reportingController + "-" + hostname,
		Action:              string(event.None),
		Reason:              string(reason),
		Regarding: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
			Namespace:  deployment.Namespace,
			UID:        deployment.UID,
		},
		Note: note,
		Type: corev1.EventTypeWarning,
	}

	if _, err := setup.EventsClient.Events(namespace).Create(ctx, ev, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "failed to create event", "reason", reason)
	}
}
