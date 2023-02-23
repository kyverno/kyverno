package generation

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
)

// stripNonPolicyFields - remove feilds which get updated with each request by kyverno and are non policy fields
func stripNonPolicyFields(obj, newRes map[string]interface{}, logger logr.Logger) (map[string]interface{}, map[string]interface{}) {
	if metadata, found := obj["metadata"]; found {
		requiredMetadataInObj := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			delete(annotations.(map[string]interface{}), "kubectl.kubernetes.io/last-applied-configuration")
			requiredMetadataInObj["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			delete(labels.(map[string]interface{}), generate.LabelClonePolicyName)
			requiredMetadataInObj["labels"] = labels
		}
		obj["metadata"] = requiredMetadataInObj
	}

	if metadata, found := newRes["metadata"]; found {
		requiredMetadataInNewRes := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			requiredMetadataInNewRes["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			requiredMetadataInNewRes["labels"] = labels
		}
		newRes["metadata"] = requiredMetadataInNewRes
	}

	delete(obj, "status")

	if _, found := obj["spec"]; found {
		delete(obj["spec"].(map[string]interface{}), "tolerations")
	}

	if dataMap, found := obj["data"]; found {
		keyInData := make([]string, 0)
		switch dataMap := dataMap.(type) {
		case map[string]interface{}:
			for k := range dataMap {
				keyInData = append(keyInData, k)
			}
		}

		if len(keyInData) > 0 {
			for _, dataKey := range keyInData {
				originalResourceData := dataMap.(map[string]interface{})[dataKey]
				replaceData := strings.Replace(originalResourceData.(string), "\n", "", -1)
				dataMap.(map[string]interface{})[dataKey] = replaceData

				newResourceData := newRes["data"].(map[string]interface{})[dataKey]
				replacenewResourceData := strings.Replace(newResourceData.(string), "\n", "", -1)
				newRes["data"].(map[string]interface{})[dataKey] = replacenewResourceData
			}
		} else {
			logger.V(4).Info("data is not of type map[string]interface{}")
		}
	}

	return obj, newRes
}

type updateRequestResponse struct {
	ur  kyvernov1beta1.UpdateRequestSpec
	err error
}

func applyUpdateRequest(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	ruleType kyvernov1beta1.RequestType,
	urGenerator updaterequest.Generator,
	userRequestInfo kyvernov1beta1.RequestInfo,
	action admissionv1.Operation,
	engineResponses ...*engineapi.EngineResponse,
) (failedUpdateRequest []updateRequestResponse) {
	admissionRequestInfo := kyvernov1beta1.AdmissionRequestInfoObject{
		AdmissionRequest: request,
		Operation:        action,
	}

	for _, er := range engineResponses {
		ur := transform(admissionRequestInfo, userRequestInfo, er, ruleType)
		if err := urGenerator.Apply(ctx, ur, action); err != nil {
			failedUpdateRequest = append(failedUpdateRequest, updateRequestResponse{ur: ur, err: err})
		}
	}

	return
}

func transform(admissionRequestInfo kyvernov1beta1.AdmissionRequestInfoObject, userRequestInfo kyvernov1beta1.RequestInfo, er *engineapi.EngineResponse, ruleType kyvernov1beta1.RequestType) kyvernov1beta1.UpdateRequestSpec {
	var PolicyNameNamespaceKey string
	if er.Policy.GetNamespace() != "" {
		PolicyNameNamespaceKey = er.Policy.GetNamespace() + "/" + er.Policy.GetName()
	} else {
		PolicyNameNamespaceKey = er.Policy.GetName()
	}

	ur := kyvernov1beta1.UpdateRequestSpec{
		Type:   ruleType,
		Policy: PolicyNameNamespaceKey,
		Resource: kyvernov1.ResourceSpec{
			Kind:       er.Resource.GetKind(),
			Namespace:  er.Resource.GetNamespace(),
			Name:       er.Resource.GetName(),
			APIVersion: er.Resource.GetAPIVersion(),
		},
		Context: kyvernov1beta1.UpdateRequestSpecContext{
			UserRequestInfo:      userRequestInfo,
			AdmissionRequestInfo: admissionRequestInfo,
		},
	}

	return ur
}
