package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	controller "github.com/nirmata/kube-policy/controller"
	kubeclient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MutationWebhook is a data type that represents
// buisness logic for resource mutation
type MutationWebhook struct {
	kubeclient *kubeclient.KubeClient
	controller *controller.PolicyController
	logger     *log.Logger
}

// NewMutationWebhook is a method that returns new instance
// of MutationWebhook struct
func NewMutationWebhook(kubeclient *kubeclient.KubeClient, controller *controller.PolicyController, logger *log.Logger) (*MutationWebhook, error) {
	if kubeclient == nil || controller == nil || logger == nil {
		return nil, errors.New("Some parameters are not set")
	}
	return &MutationWebhook{kubeclient: kubeclient, controller: controller, logger: logger}, nil
}

// Mutate applies admission to request
func (mw *MutationWebhook) Mutate(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	mw.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	policies := mw.controller.GetPolicies()
	if len(policies) == 0 {
		return nil
	}

	var allPatches []types.PolicyPatch
	for _, policy := range policies {
		stopOnError := true
		if policy.Spec.FailurePolicy != nil && *policy.Spec.FailurePolicy == "continueOnError" {
			stopOnError = false
		}

		for ruleIdx, rule := range policy.Spec.Rules {
			err := rule.Validate()
			if err != nil {
				mw.logger.Printf("Invalid rule detected: #%d in policy %s", ruleIdx, policy.ObjectMeta.Name)
				continue
			}

			if IsRuleApplicableToRequest(rule.Resource, request) {
				mw.logger.Printf("Applying policy %s, rule #%d", policy.ObjectMeta.Name, ruleIdx)
				rulePatches, err := mw.applyRule(request, rule, stopOnError)
				// If at least one error is detected in the rule, the entire rule will not be applied:
				// it can be changed in the future by varying the policy.Spec.FailurePolicy values.
				if err != nil {
					errStr := fmt.Sprintf("Unable to apply rule #%d: %s", ruleIdx, err)
					mw.logger.Print(errStr)
					mw.controller.LogPolicyError(policy.Name, errStr)
					if stopOnError {
						mw.logger.Printf("/!\\ Denying the request according to FailurePolicy spec /!\\")
					}
					return errorToAdmissionResponse(err, !stopOnError)
				}
				if rulePatches != nil {
					allPatches = append(allPatches, rulePatches...)
				}
			}
		}
	}

	patchesBytes, err := SerializePatches(allPatches)
	if err != nil {
		mw.logger.Printf("Error occerred while serializing JSONPathch: %v", err)
		return errorToAdmissionResponse(err, true)
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

// Applies all rule to the created object and returns list of JSON patches.
// May return nil patches if it is not necessary to create patches for requested object.
func (mw *MutationWebhook) applyRule(request *v1beta1.AdmissionRequest, rule types.PolicyRule, stopOnError bool) ([]types.PolicyPatch, error) {
	rulePatches, err := mw.applyRulePatches(request, rule)
	if err != nil {
		mw.logger.Printf("Error occurred while applying patches according to the policy: %v", err)
	} else {
		mw.logger.Printf("Prepared %v patches", len(rulePatches))
	}

	if err == nil || !stopOnError {
		err = mw.applyRuleGenerators(request, rule)
	}

	return rulePatches, err
}

// Gets patches from "patch" section in PolicyRule
func (mw *MutationWebhook) applyRulePatches(request *v1beta1.AdmissionRequest, rule types.PolicyRule) ([]types.PolicyPatch, error) {
	var patches []types.PolicyPatch
	patches = append(patches, rule.Patches...)
	return patches, nil
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
func (mw *MutationWebhook) applyRuleGenerators(request *v1beta1.AdmissionRequest, rule types.PolicyRule) error {
	// configMapGenerator and secretGenerator can be applied only to namespaces
	if request.Kind.Kind == "Namespace" {
		meta := parseMetadataFromObject(request.Object.Raw)
		namespaceName := parseNameFromMetadata(meta)

		err := mw.applyConfigGenerator(rule.ConfigMapGenerator, namespaceName, "ConfigMap")
		if err == nil {
			err = mw.applyConfigGenerator(rule.SecretGenerator, namespaceName, "Secret")
		}
		return err
	}
	return nil
}

// Creates resourceKind (ConfigMap or Secret) with parameters specified in generator in cluster specified in request
func (mw *MutationWebhook) applyConfigGenerator(generator *types.PolicyConfigGenerator, namespace string, configKind string) error {
	if generator == nil {
		return nil
	}

	err := generator.Validate()
	if err != nil {
		return errors.New(fmt.Sprintf("Generator for '%s' is invalid: %s", configKind, err))
	}

	switch configKind {
	case "ConfigMap":
		err = mw.kubeclient.GenerateConfigMap(*generator, namespace)
	case "Secret":
		err = mw.kubeclient.GenerateSecret(*generator, namespace)
	default:
		err = errors.New(fmt.Sprintf("Unsupported config Kind '%s'", configKind))
	}

	if err != nil {
		return errors.New(fmt.Sprintf("Unable to apply generator for %s '%s/%s' : %s", configKind, namespace, generator.Name, err))
	}

	return nil
}

// Converts JSON patches to byte array
func SerializePatches(patches []types.PolicyPatch) ([]byte, error) {
	var result []byte
	if len(patches) == 0 {
		return result, nil
	}

	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		patchBytes, err := serializePatch(patch)
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

func serializePatch(patch types.PolicyPatch) ([]byte, error) {
	err := patch.Validate()
	if err != nil {
		return nil, err
	}
	return json.Marshal(patch)
}

func errorToAdmissionResponse(err error, allowed bool) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
		Allowed: allowed,
	}
}
