package webhooks

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"

	kubeclient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	policylister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/policyengine"
	mutation "github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	v1beta1 "k8s.io/api/admission/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	rest "k8s.io/client-go/rest"
)

// MutationWebhook is a data type that represents
// business logic for resource mutation
type MutationWebhook struct {
	kubeclient   *kubeclient.KubeClient
	policyLister policylister.PolicyLister
	registration *MutationWebhookRegistration
	eventBuilder event.Generator
	logger       *log.Logger
}

// Registers mutation webhook in cluster and creates object for this webhook
func CreateMutationWebhook(
	clientConfig *rest.Config,
	kubeclient *kubeclient.KubeClient,
	policyLister policylister.PolicyLister,
	eventController event.Generator,
	logger *log.Logger) (*MutationWebhook, error) {
	if clientConfig == nil || kubeclient == nil {
		return nil, errors.New("Some parameters are not set")
	}

	registration, err := NewMutationWebhookRegistration(clientConfig, kubeclient)
	if err != nil {
		return nil, err
	}

	err = registration.Register()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = log.New(os.Stdout, "Mutation WebHook: ", log.LstdFlags|log.Lshortfile)
	}

	return &MutationWebhook{
		kubeclient:   kubeclient,
		policyLister: policyLister,
		registration: registration,
		eventBuilder: eventController,
		logger:       logger,
	}, nil
}

func (mw *MutationWebhook) getPolicies() ([]types.Policy, error) {
	selector := labels.NewSelector()
	cachedPolicies, err := mw.policyLister.List(selector)
	if err != nil {
		mw.logger.Printf("Error: %v", err)
		return nil, err
	}

	var policies []types.Policy
	for _, elem := range cachedPolicies {
		policies = append(policies, *elem.DeepCopy())
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreationTimestamp.Time.Before(policies[j].CreationTimestamp.Time)
	})
	return policies, nil

}

// Mutate applies admission to request
func (mw *MutationWebhook) Mutate(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	mw.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	policies, err := mw.getPolicies()
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	if len(policies) == 0 {
		return nil
	}

	var allPatches []mutation.PatchBytes
	for _, policy := range policies {
		mw.logger.Printf("Applying policy %s with %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches, err := mw.applyPolicyRules(request, policy)
		if err != nil {
			//TODO Log Policy Error

			errStr := fmt.Sprintf("Unable to apply policy %s: %v", policy.Name, err)
			mw.logger.Printf("Denying the request because of error: %s", errStr)
			return mw.denyResourceCreation(errStr)
		}

		if len(policyPatches) > 0 {
			namespace := mutation.ParseNamespaceFromObject(request.Object.Raw)
			name := mutation.ParseNameFromObject(request.Object.Raw)
			//TODO Log Policy Info
			mw.logger.Printf("%s applied to %s %s/%s", policy.Name, request.Kind.Kind, namespace, name)

			allPatches = append(allPatches, policyPatches...)
		}
	}

	patchType := v1beta1.PatchTypeJSONPatch
	return &v1beta1.AdmissionResponse{
		Allowed:   true,
		Patch:     mutation.JoinPatches(allPatches),
		PatchType: &patchType,
	}
}

// Applies all policy rules to the created object and returns list of processed JSON patches.
// May return nil patches if it is not necessary to create patches for requested object.
// Returns error ONLY in case when creation of resource should be denied.
func (mw *MutationWebhook) applyPolicyRules(request *v1beta1.AdmissionRequest, policy types.Policy) ([]mutation.PatchBytes, error) {
	return policyengine.Mutation(mw.logger, policy, request.Object.Raw)
}

// kind is the type of object being manipulated, e.g. request.Kind.kind
func (mw *MutationWebhook) applyPolicyRulesOnResource(kind string, rawResource []byte, policy types.Policy) ([]mutation.PatchBytes, error) {
	patchingSets := mutation.GetPolicyPatchingSets(policy)
	var policyPatches []mutation.PatchBytes

	for ruleIdx, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			mw.logger.Printf("Invalid rule detected: #%d in policy %s, err: %v\n", ruleIdx, policy.ObjectMeta.Name, err)
			continue
		}

		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			mw.logger.Printf("Rule %d of policy %s is not applicable to the request", ruleIdx, policy.Name)
			return nil, err
		}

		// configMapGenerator and secretGenerator can be applied only to namespaces
		if kind == "Namespace" {
			err = mw.applyRuleGenerators(rawResource, rule)
			if err != nil && patchingSets == mutation.PatchingSetsStopOnError {
				return nil, fmt.Errorf("Failed to apply generators from rule #%d: %s", ruleIdx, err)
			}
		}

		rulePatchesProcessed, err := mutation.ProcessPatches(rule.Patches, rawResource, patchingSets)
		if err != nil {
			return nil, fmt.Errorf("Failed to process patches from rule #%d: %s", ruleIdx, err)
		}

		if rulePatchesProcessed != nil {
			policyPatches = append(policyPatches, rulePatchesProcessed...)
			mw.logger.Printf("Rule %d: prepared %d patches", ruleIdx, len(rulePatchesProcessed))
		} else {
			mw.logger.Printf("Rule %d: no patches prepared", ruleIdx)
		}
	}

	// empty patch, return error to deny resource creation
	if policyPatches == nil {
		return nil, fmt.Errorf("no patches prepared")
	}

	return policyPatches, nil
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
func (mw *MutationWebhook) applyRuleGenerators(rawResource []byte, rule types.PolicyRule) error {
	namespaceName := mutation.ParseNameFromObject(rawResource)

	err := mw.applyConfigGenerator(rule.ConfigMapGenerator, namespaceName, "ConfigMap")
	if err == nil {
		err = mw.applyConfigGenerator(rule.SecretGenerator, namespaceName, "Secret")
	}
	return err
}

// Creates resourceKind (ConfigMap or Secret) with parameters specified in generator in cluster specified in request.
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

// Forms AdmissionResponse with denial of resource creation and error message
func (mw *MutationWebhook) denyResourceCreation(errStr string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: errStr,
		},
		Allowed: false,
	}
}
