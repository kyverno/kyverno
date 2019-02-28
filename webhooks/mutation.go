package webhooks

import (
	"encoding/json"
	"errors"
	"log"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MutationWebhook struct {
	logger *log.Logger
}

func NewMutationWebhook(logger *log.Logger) (*MutationWebhook, error) {
	if logger == nil {
		return nil, errors.New("Logger must be set for the mutation webhook")
	}
	return &MutationWebhook{logger: logger}, nil
}

func (mw *MutationWebhook) Mutate(request *v1beta1.AdmissionRequest, policies []types.Policy) *v1beta1.AdmissionResponse {
	mw.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	if len(policies) == 0 {
		return nil
	}

	var allPatches []types.PolicyPatch
	for _, policy := range policies {
		var stopOnError bool = true
		if policy.Spec.FailurePolicy != nil && *policy.Spec.FailurePolicy == "continueOnError" {
			stopOnError = false
		}

		for ruleIdx, rule := range policy.Spec.Rules {
			if IsRuleApplicableToRequest(rule.Resource, request) {
				mw.logger.Printf("Applying policy %v, rule index = %v", policy.ObjectMeta.Name, ruleIdx)
				rulePatches, err := mw.applyPolicyRule(request, rule)
				/*
				 * If at least one error is detected in the rule, the entire rule will not be applied.
				 * This may be changed in the future by varying the policy.Spec.FailurePolicy values.
				 */
				if err != nil {
					mw.logger.Printf("Error occurred while applying the policy: %v", err)
					if stopOnError {
						mw.logger.Printf("/!\\ Denying the request according to FailurePolicy spec /!\\")
						return errorToResponse(err, false)
					}
				} else {
					mw.logger.Printf("Prepared %v patches", len(rulePatches))
					allPatches = append(allPatches, rulePatches...)
				}
			}
		}
	}

	patchesBytes, err := SerializePatches(allPatches)
	if err != nil {
		mw.logger.Printf("Error occerred while serializing JSONPathch: %v", err)
		return errorToResponse(err, true)
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchesBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// Applies all possible patches in a rule
func (mw *MutationWebhook) applyPolicyRule(request *v1beta1.AdmissionRequest, rule types.PolicyRule) ([]types.PolicyPatch, error) {
	var allPatches []types.PolicyPatch
	if rule.Patches == nil && rule.ConfigMapGenerator == nil && rule.SecretGenerator == nil {
		return nil, errors.New("The rule is empty!")
	}

	allPatches = append(allPatches, rule.Patches...)

	if rule.ConfigMapGenerator != nil {
		// TODO: Make patches from configMapGenerator and add them to returned array
	}

	if rule.SecretGenerator != nil {
		// TODO: Make patches from secretGenerator and add them to returned array
	}

	return allPatches, nil
}

func SerializePatches(patches []types.PolicyPatch) ([]byte, error) {
	var result []byte
	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		if patch.Operation == "" || patch.Path == "" {
			return nil, errors.New("JSONPatch doesn't contain mandatory fields 'path' or 'op'")
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return nil, err
		}

		result = append(result, patchBytes...)
		if index != (len(patches) - 1) {
			result = append(result, []byte(",\n")...)
		}
	}
	result = append(result, []byte("\n]")...)
	return result, nil
}

func errorToResponse(err error, allowed bool) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
		Allowed: allowed,
	}
}
