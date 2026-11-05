package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// legacyStatusTimeout bounds the list and status patches below, for the same reason as
// legacyPolicyCheckTimeout: kyverno-init gates the main container and must never hang on a slow
// API server.
const legacyStatusTimeout = 60 * time.Second

const (
	legacyReadyConditionType   = "Ready"
	legacyReadyConditionStatus = metav1.ConditionFalse
	legacyReadyConditionReason = "Unsupported"
)

// legacyReadyConditionMessage replaces the stale "Ready" message the pre-1.20 controllers left on
// legacy policies: nothing has enforced these objects since the legacy engine was removed, and
// nothing else writes their status any more.
var legacyReadyConditionMessage = fmt.Sprintf(
	"policy is not enforced: legacy kyverno.io policies are no longer supported, migrate to policies.kyverno.io (see %s)",
	deprecations.MigrationGuideURL,
)

// legacyStatusKinds are the legacy kinds whose Ready condition is shown in kubectl's READY and
// MESSAGE printer columns.
var legacyStatusKinds = []string{"ClusterPolicy", "Policy"}

// markLegacyPoliciesUnsupported rewrites the status of every legacy ClusterPolicy and Policy so
// its conditions are exactly one Ready=False/Unsupported condition
func markLegacyPoliciesUnsupported(ctx context.Context, setup internal.SetupResult) {
	if setup.DynamicClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, legacyStatusTimeout)
	defer cancel()
	if err := markLegacyStatuses(ctx, setup.DynamicClient, time.Now()); err != nil {
		// A stale status message is cosmetic, so this must not block kyverno-init (and therefore
		// the admission controller it gates): log and carry on, the next startup retries.
		logging.WithName("kyvernopre/legacy-status").Error(err, "failed to mark legacy kyverno.io policies as unsupported")
	}
}

// markLegacyStatuses is the core of markLegacyPoliciesUnsupported. It keeps going past
// per-object failures and returns all of them joined. A missing CRD or an object deleted since the
// list is not an error.
func markLegacyStatuses(ctx context.Context, client dynamic.Interface, now time.Time) error {
	patch := legacyStatusPatch(now)
	var errs []error
	for _, kind := range legacyStatusKinds {
		gvr := deprecations.LegacyPolicyResources[kind]
		list, err := client.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("listing %s: %w", gvr.Resource, err))
			}
			continue
		}
		for i := range list.Items {
			item := &list.Items[i]
			if hasLegacyUnsupportedStatus(item) {
				continue
			}
			_, err := client.Resource(gvr).Namespace(item.GetNamespace()).Patch(
				ctx, item.GetName(), types.MergePatchType, patch, metav1.PatchOptions{}, "status")
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("patching %s %s/%s status: %w", kind, item.GetNamespace(), item.GetName(), err))
			}
		}
	}
	return errors.Join(errs...)
}

// legacyStatusPatch builds the merge patch. status.conditions is a list, so a merge patch
// replaces it entirely with the single condition below.
func legacyStatusPatch(now time.Time) []byte {
	// marshalling a map of strings cannot fail
	patch, _ := json.Marshal(map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":               legacyReadyConditionType,
					"status":             string(legacyReadyConditionStatus),
					"reason":             legacyReadyConditionReason,
					"message":            legacyReadyConditionMessage,
					"lastTransitionTime": now.UTC().Format(time.RFC3339),
				},
			},
		},
	})
	return patch
}

func hasLegacyUnsupportedStatus(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found || len(conditions) != 1 {
		return false
	}
	condition, ok := conditions[0].(map[string]any)
	if !ok {
		return false
	}
	return condition["type"] == legacyReadyConditionType &&
		condition["status"] == string(legacyReadyConditionStatus) &&
		condition["reason"] == legacyReadyConditionReason &&
		condition["message"] == legacyReadyConditionMessage
}
