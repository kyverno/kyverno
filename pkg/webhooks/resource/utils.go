package resource

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

type updateRequestResponse struct {
	ur  kyvernov1beta1.UpdateRequestSpec
	err error
}

func errorResponse(logger logr.Logger, uid types.UID, err error, message string) admissionv1.AdmissionResponse {
	logger.Error(err, message)
	return admissionutils.Response(uid, errors.New(message+": "+err.Error()))
}

func patchRequest(patches []byte, request admissionv1.AdmissionRequest, logger logr.Logger) admissionv1.AdmissionRequest {
	patchedResource := processResourceWithPatches(patches, request.Object.Raw, logger)
	request.Object.Raw = patchedResource
	return request
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

func applyUpdateRequest(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	ruleType kyvernov1beta1.RequestType,
	urGenerator updaterequest.Generator,
	userRequestInfo kyvernov1beta1.RequestInfo,
	action admissionv1.Operation,
	engineResponses ...*engineapi.EngineResponse,
) (failedUpdateRequest []updateRequestResponse) {
	admissionRequestInfo := kyvernov1beta1.AdmissionRequestInfoObject{
		AdmissionRequest: &request,
		Operation:        action,
	}

	for _, er := range engineResponses {
		urs := transform(admissionRequestInfo, userRequestInfo, er, ruleType)
		for _, ur := range urs {
			if err := urGenerator.Apply(ctx, ur); err != nil {
				failedUpdateRequest = append(failedUpdateRequest, updateRequestResponse{ur: ur, err: err})
			}
		}
	}

	return
}

func transform(admissionRequestInfo kyvernov1beta1.AdmissionRequestInfoObject, userRequestInfo kyvernov1beta1.RequestInfo, er *engineapi.EngineResponse, ruleType kyvernov1beta1.RequestType) (urs []kyvernov1beta1.UpdateRequestSpec) {
	var PolicyNameNamespaceKey string
	if er.Policy().GetNamespace() != "" {
		PolicyNameNamespaceKey = er.Policy().GetNamespace() + "/" + er.Policy().GetName()
	} else {
		PolicyNameNamespaceKey = er.Policy().GetName()
	}

	for _, rule := range er.PolicyResponse.Rules {
		ur := kyvernov1beta1.UpdateRequestSpec{
			Type:   ruleType,
			Policy: PolicyNameNamespaceKey,
			Rule:   rule.Name(),
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
		urs = append(urs, ur)
	}

	return urs
}
