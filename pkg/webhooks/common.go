package webhooks

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func isResponseSuccesful(engineReponses []response.EngineResponse) bool {
	for _, er := range engineReponses {
		if !er.IsSuccesful() {
			return false
		}
	}
	return true
}

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlockResource(engineReponses []response.EngineResponse) bool {
	for _, er := range engineReponses {
		if !er.IsSuccesful() && er.PolicyResponse.ValidationFailureAction == Enforce {
			glog.V(4).Infof("ValidationFailureAction set to enforce for policy %s , blocking resource request ", er.PolicyResponse.Policy)
			return true
		}
	}
	glog.V(4).Infoln("ValidationFailureAction set to audit, allowing resource request, reporting with policy violation")
	return false
}

func getErrorMsg(engineReponses []response.EngineResponse) string {
	var str []string
	var resourceInfo string

	for _, er := range engineReponses {
		if !er.IsSuccesful() {
			// resource in engineReponses is identical as this was called per admission request
			resourceInfo = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			str = append(str, fmt.Sprintf("failed policy %s:", er.PolicyResponse.Policy))
			for _, rule := range er.PolicyResponse.Rules {
				if !rule.Success {
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

// extract the kinds that the policy rules apply to
func getApplicableKindsForPolicy(p *kyverno.ClusterPolicy) []string {
	kinds := []string{}
	// iterate over the rules an identify all kinds
	// Matching
	for _, rule := range p.Spec.Rules {
		for _, k := range rule.MatchResources.Kinds {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// Policy Reporting Modes
const (
	Enforce = "enforce" // blocks the request on failure
	Audit   = "audit"   // dont block the request on failure, but report failiures as policy violations
)

func processResourceWithPatches(patch []byte, resource []byte) []byte {
	if patch == nil {
		return nil
	}

	resource, err := engineutils.ApplyPatchNew(resource, patch)
	if err != nil {
		glog.Errorf("failed to patch resource: %v", err)
		return nil
	}
	return resource
}

func containRBACinfo(policies []kyverno.ClusterPolicy) bool {
	for _, policy := range policies {
		for _, rule := range policy.Spec.Rules {
			if len(rule.MatchResources.Roles) > 0 || len(rule.MatchResources.ClusterRoles) > 0 {
				return true
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
