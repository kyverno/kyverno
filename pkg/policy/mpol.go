package policy

import (
	"context"
	"encoding/json"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// newCELMutateUR creates a bare CELMutate UpdateRequest for a MutatingPolicy background scan.
// There is no admission request context – the processor will fetch all matching resources itself.
func newCELMutateUR(mpol *policiesv1beta1.MutatingPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.CELMutate,
		Policy: mpol.GetName(),
	}
	return ur
}

// newCELMutateURFromNamespacedPolicy creates a bare CELMutate UpdateRequest for a
// NamespacedMutatingPolicy background scan. The policy key uses namespace/name format.
func newCELMutateURFromNamespacedPolicy(nmpol *policiesv1beta1.NamespacedMutatingPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.CELMutate,
		Policy: nmpol.GetNamespace() + "/" + nmpol.GetName(),
	}
	return ur
}

func (pc *policyController) newCELMutateURForTrigger(policyKey string, trigger *unstructured.Unstructured) (*kyvernov2.UpdateRequest, error) {
	mapping, err := pc.restMapper.RESTMapping(trigger.GroupVersionKind().GroupKind(), trigger.GroupVersionKind().Version)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(trigger.Object)
	if err != nil {
		return nil, err
	}
	ur := newUrMeta()
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.CELMutate,
		Policy: policyKey,
		Context: kyvernov2.UpdateRequestSpecContext{
			AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
				Operation: admissionv1.Update,
				AdmissionRequest: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Kind: metav1.GroupVersionKind{
						Group: trigger.GroupVersionKind().Group, Version: trigger.GroupVersionKind().Version, Kind: trigger.GetKind(),
					},
					Resource: metav1.GroupVersionResource{
						Group: mapping.Resource.Group, Version: mapping.Resource.Version, Resource: mapping.Resource.Resource,
					},
					Namespace: trigger.GetNamespace(),
					Name:      trigger.GetName(),
					Object:    runtime.RawExtension{Raw: raw},
				},
			},
		},
	}
	return ur, nil
}

func (pc *policyController) createTriggerURs(policyKey string, matchConstraints *admissionregistrationv1.MatchResources, namespace string) error {
	if matchConstraints == nil {
		return nil
	}
	triggers := filterTriggersByNamespace(pc.getGpolTriggers(matchConstraints), namespace)
	var errs []error
	for _, trigger := range triggers {
		ur, err := pc.newCELMutateURForTrigger(policyKey, trigger)
		if err == nil {
			err = pc.submitUR(context.TODO(), ur)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

// createURForMutatingPolicy creates a CELMutate UpdateRequest that causes the background
// controller to apply this policy to all currently matching resources.
func (pc *policyController) createURForMutatingPolicy(mpol *policiesv1beta1.MutatingPolicy) error {
	if mpol.GetTargetMatchConstraints().Expression != "" {
		return pc.createTriggerURs(mpol.GetName(), mpol.Spec.MatchConstraints, "")
	}
	return pc.submitUR(context.TODO(), newCELMutateUR(mpol))
}

// createURForNamespacedMutatingPolicy creates a CELMutate UpdateRequest for a
// NamespacedMutatingPolicy background scan.
func (pc *policyController) createURForNamespacedMutatingPolicy(nmpol *policiesv1beta1.NamespacedMutatingPolicy) error {
	if nmpol.GetTargetMatchConstraints().Expression != "" {
		return pc.createTriggerURs(nmpol.GetNamespace()+"/"+nmpol.GetName(), nmpol.Spec.MatchConstraints, nmpol.GetNamespace())
	}
	return pc.submitUR(context.TODO(), newCELMutateURFromNamespacedPolicy(nmpol))
}
