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

// legacyPolicyCheckTimeout bounds the direct-list count plus the Deployment
// lookup and Event creation below, so a slow or unreachable API server can't
// block kyverno-init -- and therefore the main container it gates -- forever.
// A timeout is treated like any other failure here: logged, never fatal.
const legacyPolicyCheckTimeout = 30 * time.Second

// checkLegacyPolicies counts the legacy (non policies.kyverno.io) policy
// custom resources still present in the cluster -- ClusterPolicy, Policy,
// CleanupPolicy, ClusterCleanupPolicy, and legacy PolicyException -- using a
// direct list through the typed client, since kyverno-init runs once at
// startup, before any controller has a synced cache. When any of them are
// still present, it emits a single L0 error log and a single aggregate
// Warning Event attached to the admission-controller Deployment, with
// migration guidance.
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
		emitLegacyPolicyEvent(ctx, logger, setup, note)
	}
}

// countLegacyPolicies is a thin adapter from the typed client to
// deprecations.CountLegacyPolicies, the shared pure counting core also used
// by the admission-controller and cleanup-controller gauge callbacks (which
// instead adapt from synced informer listers).
func countLegacyPolicies(ctx context.Context, setup internal.SetupResult) (map[string]int, error) {
	counters := map[string]deprecations.KindCounter{
		"ClusterPolicy": func() (int, error) {
			list, err := setup.KyvernoClient.KyvernoV1().ClusterPolicies().List(ctx, metav1.ListOptions{})
			if err != nil {
				// A missing legacy CRD (for example when PolicyException is not
				// installed) lists as NotFound; treat it as zero present so one
				// absent kind does not abort counting the others.
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		},
		"Policy": func() (int, error) {
			list, err := setup.KyvernoClient.KyvernoV1().Policies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				// A missing legacy CRD (for example when PolicyException is not
				// installed) lists as NotFound; treat it as zero present so one
				// absent kind does not abort counting the others.
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		},
		"CleanupPolicy": func() (int, error) {
			list, err := setup.KyvernoClient.KyvernoV2().CleanupPolicies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				// A missing legacy CRD (for example when PolicyException is not
				// installed) lists as NotFound; treat it as zero present so one
				// absent kind does not abort counting the others.
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		},
		"ClusterCleanupPolicy": func() (int, error) {
			list, err := setup.KyvernoClient.KyvernoV2().ClusterCleanupPolicies().List(ctx, metav1.ListOptions{})
			if err != nil {
				// A missing legacy CRD (for example when PolicyException is not
				// installed) lists as NotFound; treat it as zero present so one
				// absent kind does not abort counting the others.
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		},
		"PolicyException": func() (int, error) {
			list, err := setup.KyvernoClient.KyvernoV2().PolicyExceptions(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				// A missing legacy CRD (for example when PolicyException is not
				// installed) lists as NotFound; treat it as zero present so one
				// absent kind does not abort counting the others.
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		},
	}
	return deprecations.CountLegacyPolicies(counters)
}

// emitLegacyPolicyEvent creates a single aggregate Warning Event listing the
// legacy kinds still present, attached to the admission-controller
// Deployment. kyverno-init does not run the event generator/queue used by
// the long-running controllers, so the Event is created directly through the
// events client.
func emitLegacyPolicyEvent(ctx context.Context, logger logr.Logger, setup internal.SetupResult, note string) {
	if setup.EventsClient == nil {
		return
	}

	namespace := config.KyvernoNamespace()
	deploymentName := config.KyvernoDeploymentName()
	deployment, err := setup.KubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "admission-controller Deployment not found, skipping legacy policy event", "namespace", namespace, "name", deploymentName)
			return
		}
		logger.Error(err, "failed to get admission-controller Deployment, skipping legacy policy event", "namespace", namespace, "name", deploymentName)
		return
	}

	hostname, _ := os.Hostname()
	reportingController := string(event.KyvernoInit)
	now := time.Now()

	// defensive: matches the Note truncation in pkg/event/controller.go's emitEvent,
	// in case a future kind is added without updating deprecations.LegacyPolicyEventNote
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
		Reason:              string(event.LegacyPolicyPresent),
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
		logger.Error(err, "failed to create legacy policy event")
	}
}
