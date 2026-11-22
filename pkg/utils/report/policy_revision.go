package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EffectivePolicyRevision tracks evaluation-relevant CEL policy state.
// Metadata.resourceVersion changes on condition and annotation updates too;
// these must not invalidate reports or cause a cluster-wide background rescan.
// The revision fits in a Kubernetes label value (63 characters).
func EffectivePolicyRevision(policy metav1.Object) string {
	state := struct {
		Generation int64  `json:"generation"`
		UID        string `json:"uid,omitempty"`
		Excluded   bool   `json:"excluded"`
		Category   string `json:"category,omitempty"`
		Severity   string `json:"severity,omitempty"`
		Scored     string `json:"scored,omitempty"`
		Generated  bool   `json:"generated,omitempty"`
		Autogen    any    `json:"autogen,omitempty"`
	}{
		Generation: policy.GetGeneration(),
		UID:        string(policy.GetUID()),
	}
	_, state.Excluded = policy.GetLabels()[kyverno.LabelExcludeReporting]
	annotations := policy.GetAnnotations()
	state.Category = annotations["policies.kyverno.io/category"]
	state.Severity = annotations["policies.kyverno.io/severity"]
	state.Scored = annotations["policies.kyverno.io/scored"]

	// Generated-state transitions and autogen targets change which evaluation
	// runs, even when Kubernetes generation itself remains unchanged.
	switch p := policy.(type) {
	case *policiesv1beta1.ValidatingPolicy:
		state.Generated = p.Status.Generated
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	case *policiesv1beta1.NamespacedValidatingPolicy:
		state.Generated = p.Status.Generated
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	case *policiesv1beta1.MutatingPolicy:
		state.Generated = p.Status.Generated
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	case *policiesv1beta1.NamespacedMutatingPolicy:
		state.Generated = p.Status.Generated
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	case *policiesv1beta1.ImageValidatingPolicy:
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	case *policiesv1beta1.NamespacedImageValidatingPolicy:
		if len(p.Status.Autogen.Configs) != 0 {
			state.Autogen = p.Status.Autogen.Configs
		}
	default:
		// Leave native VAP/MAP invalidation unchanged.
		return policy.GetResourceVersion()
	}

	data, err := json.Marshal(state)
	if err != nil {
		// Never skip a real policy update if the fingerprint cannot be built.
		return policy.GetResourceVersion()
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("e1-%d-%x", policy.GetGeneration(), sum[:12])
}

// PolicyRevision is the value persisted on EphemeralReport policy labels.
// It must match the informer update predicate and partial-scan comparator.
func PolicyRevision(policy engineapi.GenericPolicy) string {
	if obj, ok := policy.AsObject().(metav1.Object); ok {
		return EffectivePolicyRevision(obj)
	}
	return policy.GetResourceVersion()
}
