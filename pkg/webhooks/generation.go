package webhooks

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

//HandleGenerate handles admission-requests for policies with generate rules
func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	var engineResponses []response.EngineResponse

	// convert RAW to unstructured
	resource, err := utils.ConvertToUnstructured(request.Object.Raw)
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
	err = ctx.AddResource(request.Object.Raw)
	if err != nil {
		glog.Infof("Failed to load resource in context:%v", err)
	}
	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		glog.Infof("Failed to load userInfo in context:%v", err)
	}
	// load service account in context
	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		glog.Infof("Failed to load service account in context:%v", err)
	}

	policyContext := engine.PolicyContext{
		NewResource:   *resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	// engine.Generate returns a list of rules that are applicable on this resource
	for _, policy := range policies {
		policyContext.Policy = policy
		engineResponse := engine.Generate(policyContext)
		go ws.status.UpdateStatusWithGenerateStats(engineResponse)
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
