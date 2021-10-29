package webhooks

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// isResponseSuccessful return true if all responses are successful
func isResponseSuccessful(engineReponses []*response.EngineResponse) bool {
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			return false
		}
	}
	return true
}

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlockResource(engineReponses []*response.EngineResponse, log logr.Logger) bool {
	for _, er := range engineReponses {
		if !er.IsSuccessful() && er.PolicyResponse.ValidationFailureAction == common.Enforce {
			log.Info("spec.ValidationFailureAction set to enforce blocking resource request", "policy", er.PolicyResponse.Policy.Name)
			return true
		}
	}

	log.V(4).Info("spec.ValidationFailureAction set to audit for all applicable policies, won't block resource operation")
	return false
}

// getEnforceFailureErrorMsg gets the error messages for failed enforce policy
func getEnforceFailureErrorMsg(engineResponses []*response.EngineResponse) string {
	policyToRule := make(map[string]interface{})
	var resourceName string
	for _, er := range engineResponses {
		if !er.IsSuccessful() && er.PolicyResponse.ValidationFailureAction == common.Enforce {
			ruleToReason := make(map[string]string)
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					ruleToReason[rule.Name] = rule.Message
				}
			}

			resourceName = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			policyToRule[er.PolicyResponse.Policy.Name] = ruleToReason
		}
	}

	result, _ := yamlv2.Marshal(policyToRule)
	return "\n\nresource " + resourceName + " was blocked due to the following policies\n\n" + string(result)
}

// getErrorMsg gets all failed engine response message
func getErrorMsg(engineReponses []*response.EngineResponse) string {
	var str []string
	var resourceInfo string

	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			// resource in engineReponses is identical as this was called per admission request
			resourceInfo = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			str = append(str, fmt.Sprintf("failed policy %s:", er.PolicyResponse.Policy.Name))
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					str = append(str, rule.ToString())
				}
			}
		}
	}
	return fmt.Sprintf("Resource %s %s", resourceInfo, strings.Join(str, ";"))
}

//ArrayFlags to store filterkinds
type ArrayFlags []string

func (i *ArrayFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

//Set setter for array flags
func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func processResourceWithPatches(patch []byte, resource []byte, log logr.Logger) []byte {
	if patch == nil {
		return resource
	}

	resource, err := engineutils.ApplyPatchNew(resource, patch)
	if err != nil {
		log.Error(err, "failed to patch resource:", "patch", string(patch), "resource", string(resource))
		return nil
	}

	log.V(6).Info("", "patchedResource", string(resource))
	return resource
}

func containsRBACInfo(policies ...[]*kyverno.ClusterPolicy) bool {
	for _, policySlice := range policies {
		for _, policy := range policySlice {
			for _, rule := range policy.Spec.Rules {
				if len(rule.MatchResources.Roles) > 0 || len(rule.MatchResources.ClusterRoles) > 0 || len(rule.ExcludeResources.Roles) > 0 || len(rule.ExcludeResources.ClusterRoles) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// extracts the new and old resource as unstructured
func extractResources(newRaw []byte, request *v1beta1.AdmissionRequest) (unstructured.Unstructured, unstructured.Unstructured, error) {
	var emptyResource unstructured.Unstructured

	// New Resource
	if newRaw == nil {
		newRaw = request.Object.Raw
	}
	if newRaw == nil {
		return emptyResource, emptyResource, fmt.Errorf("new resource is not defined")
	}

	new, err := convertResource(newRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return emptyResource, emptyResource, fmt.Errorf("failed to convert new raw to unstructured: %v", err)
	}

	// Old Resource - Optional
	oldRaw := request.OldObject.Raw
	if oldRaw == nil {
		return new, emptyResource, nil
	}

	old, err := convertResource(oldRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return emptyResource, emptyResource, fmt.Errorf("failed to convert old raw to unstructured: %v", err)
	}
	return new, old, err
}

// convertResource converts raw bytes to an unstructured object
func convertResource(raw []byte, group, version, kind, namespace string) (unstructured.Unstructured, error) {
	obj, err := engineutils.ConvertToUnstructured(raw)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to convert raw to unstructured: %v", err)
	}

	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	obj.SetNamespace(namespace)
	return *obj, nil
}

func excludeKyvernoResources(kind string) bool {
	switch kind {
	case "ClusterPolicyReport":
		return true
	case "PolicyReport":
		return true
	case "ReportChangeRequest":
		return true
	case "GenerateRequest":
		return true
	case "ClusterReportChangeRequest":
		return true
	default:
		return false
	}
}
