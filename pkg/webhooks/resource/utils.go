package resource

import (
	"errors"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
)

type updateRequestResponse struct {
	ur  kyvernov1beta1.UpdateRequestSpec
	err error
}

func errorResponse(logger logr.Logger, err error, message string) *admissionv1.AdmissionResponse {
	logger.Error(err, message)
	return admissionutils.Response(errors.New(message + ": " + err.Error()))
}

func patchRequest(patches []byte, request *admissionv1.AdmissionRequest, logger logr.Logger) *admissionv1.AdmissionRequest {
	patchedResource := processResourceWithPatches(patches, request.Object.Raw, logger)
	newRequest := request.DeepCopy()
	newRequest.Object.Raw = patchedResource
	return newRequest
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

func applyUpdateRequest(request *admissionv1.AdmissionRequest, ruleType kyvernov1beta1.RequestType, grGenerator updaterequest.Generator, userRequestInfo kyvernov1beta1.RequestInfo,
	action admissionv1.Operation, engineResponses ...*response.EngineResponse,
) (failedUpdateRequest []updateRequestResponse) {
	admissionRequestInfo := kyvernov1beta1.AdmissionRequestInfoObject{
		AdmissionRequest: request,
		Operation:        action,
	}

	for _, er := range engineResponses {
		ur := transform(admissionRequestInfo, userRequestInfo, er, ruleType)
		if err := grGenerator.Apply(ur, action); err != nil {
			failedUpdateRequest = append(failedUpdateRequest, updateRequestResponse{ur: ur, err: err})
		}
	}

	return
}

func transform(admissionRequestInfo kyvernov1beta1.AdmissionRequestInfoObject, userRequestInfo kyvernov1beta1.RequestInfo, er *response.EngineResponse, ruleType kyvernov1beta1.RequestType) kyvernov1beta1.UpdateRequestSpec {
	var PolicyNameNamespaceKey string
	if er.PolicyResponse.Policy.Namespace != "" {
		PolicyNameNamespaceKey = er.PolicyResponse.Policy.Namespace + "/" + er.PolicyResponse.Policy.Name
	} else {
		PolicyNameNamespaceKey = er.PolicyResponse.Policy.Name
	}

	ur := kyvernov1beta1.UpdateRequestSpec{
		Type:   ruleType,
		Policy: PolicyNameNamespaceKey,
		Resource: kyvernov1.ResourceSpec{
			Kind:       er.PolicyResponse.Resource.Kind,
			Namespace:  er.PolicyResponse.Resource.Namespace,
			Name:       er.PolicyResponse.Resource.Name,
			APIVersion: er.PolicyResponse.Resource.APIVersion,
		},
		Context: kyvernov1beta1.UpdateRequestSpecContext{
			UserRequestInfo:      userRequestInfo,
			AdmissionRequestInfo: admissionRequestInfo,
		},
	}

	return ur
}
