package webhooks

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	var engineResponses []response.EngineResponse

	// convert RAW to unstructured
	resource, err := engine.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		//TODO: skip applying the admission control ?
		glog.Errorf("unable to convert raw resource to unstructured: %v", err)
		return true, ""
	}

	// CREATE resources, do not have name, assigned in admission-request
	glog.V(4).Infof("Handle Generate: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)

	userRequestInfo := kyverno.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}
	// build context
	ctx := context.NewContext()
	// load incoming resource into the context
	// ctx.AddResource(request.Object.Raw)
	ctx.AddUserInfo(userRequestInfo)
	// load service account in context
	ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)

	policyContext := engine.PolicyContext{
		NewResource:   *resource,
		AdmissionInfo: userRequestInfo,
	}

	// engine.Generate returns a list of rules that are applicable on this resource
	for _, policy := range policies {
		policyContext.Policy = policy
		engineResponse := engine.GenerateNew(policyContext)
		if len(engineResponse.PolicyResponse.Rules) > 0 {
			// some generate rules do apply to the resource
			engineResponses = append(engineResponses, engineResponse)
		}
	}
	// Adds Generate Request to a channel(queue size 1000) to generators
	if err := createGenerateRequest(ws.grGenerator, userRequestInfo, engineResponses...); err != nil {
		//TODO: send appropriate error
		return false, "Kyverno blocked: failed to create Generate Requests"
	}
	// Generate Stats wont be used here, as we delegate the generate rule
	// - Filter policies that apply on this resource
	// - - build CR context(userInfo+roles+clusterRoles)
	// - Create CR
	// - send Success
	// HandleGeneration  always returns success

	// Filter Policies
	return true, ""
}

func createGenerateRequest(gnGenerator generate.GenerateRequests, userRequestInfo kyverno.RequestInfo, engineResponses ...response.EngineResponse) error {
	for _, er := range engineResponses {
		if err := gnGenerator.Create(transform(userRequestInfo, er)); err != nil {
			return err
		}
	}
	return nil
}

func transform(userRequestInfo kyverno.RequestInfo, er response.EngineResponse) kyverno.GenerateRequestSpec {
	gr := kyverno.GenerateRequestSpec{
		Policy: er.PolicyResponse.Policy,
		Resource: kyverno.ResourceSpec{
			Kind:      er.PolicyResponse.Resource.Kind,
			Namespace: er.PolicyResponse.Resource.Namespace,
			Name:      er.PolicyResponse.Resource.Name,
		},
		Context: kyverno.GenerateRequestContext{
			UserRequestInfo: userRequestInfo,
		},
	}
	return gr
}
