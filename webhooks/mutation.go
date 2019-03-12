package webhooks

import (
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

	var allPatches []PatchBytes
	for _, policy := range policies {
		patchingSets := getPolicyPatchingSets(policy)

		for ruleIdx, rule := range policy.Spec.Rules {
			err := rule.Validate()
			if err != nil {
				mw.logger.Printf("Invalid rule detected: #%d in policy %s", ruleIdx, policy.ObjectMeta.Name)
				continue
			}

			mw.logger.Printf("Applying policy %s, rule #%d", policy.ObjectMeta.Name, ruleIdx)
			rulePatches, err := mw.applyRule(request, rule, patchingSets)

			if err != nil {
				errStr := fmt.Sprintf("Unable to apply rule #%d: %s", ruleIdx, err)
				mw.logger.Printf("Denying the request because of error: %s", errStr)
				mw.controller.LogPolicyError(policy.Name, errStr)
				return errorToAdmissionResponse(err, true)
			}

			rulePatchesProcessed, err := ProcessPatches(rulePatches, request.Object.Raw, patchingSets)
			if rulePatches != nil {
				allPatches = append(allPatches, rulePatchesProcessed...)
				mw.logger.Printf("Prepared %d patches", len(rulePatchesProcessed))
			} else {
				mw.logger.Print("No patches prepared")
			}
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   JoinPatches(allPatches),
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func getPolicyPatchingSets(policy types.Policy) PatchingSets {
	// failurePolicy property is the only available way for now to define behavior on patching error.
	// TODO: define new failurePolicy values specific for patching and other policy features.
	if policy.Spec.FailurePolicy != nil && *policy.Spec.FailurePolicy == "continueOnError" {
		return PatchingSetsContinueAlways
	}
	return PatchingSetsDefault
}

// Applies all rule to the created object and returns list of JSON patches.
// May return nil patches if it is not necessary to create patches for requested object.
func (mw *MutationWebhook) applyRule(request *v1beta1.AdmissionRequest, rule types.PolicyRule, errorBehavior PatchingSets) ([]types.PolicyPatch, error) {
	if !IsRuleApplicableToRequest(rule.Resource, request) {
		return nil, nil
	}

	err := mw.applyRuleGenerators(request, rule)
	if err != nil && errorBehavior == PatchingSetsStopOnError {
		return nil, err
	} else {
		return rule.Patches, nil
	}
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

func errorToAdmissionResponse(err error, allowed bool) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
		Allowed: allowed,
	}
}
