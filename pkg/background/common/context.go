package common

import (
	"encoding/json"

	"github.com/gardener/controller-manager-library/pkg/logger"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewBackgroundContext(dclient *dclient.Client, ur *urkyverno.UpdateRequest,
	policy kyverno.PolicyInterface, trigger, target *unstructured.Unstructured,
	cfg config.Interface, namespaceLabels map[string]string) (*engine.PolicyContext, bool, error) {

	ctx := context.NewContext()
	requestString := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	var request admissionv1.AdmissionRequest

	err := json.Unmarshal([]byte(requestString), &request)
	if err != nil {
		logger.Error(err, "error parsing the request string")
	}

	if ur.Spec.Context.AdmissionRequestInfo.Operation == admissionv1.Update {
		request.Operation = ur.Spec.Context.AdmissionRequestInfo.Operation
	}

	if err := ctx.AddRequest(&request); err != nil {
		logger.Error(err, "failed to load request in context")
		return nil, false, err
	}

	err = ctx.AddResource(trigger.Object)
	if err != nil {
		logger.Error(err, "failed to load resource in context")
		return nil, false, err
	}

	err = ctx.AddUserInfo(ur.Spec.Context.UserRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load SA in context")
		return nil, false, err
	}

	err = ctx.AddServiceAccount(ur.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load UserInfo in context")
		return nil, false, err
	}

	if err := ctx.AddImageInfos(trigger); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	policyContext := &engine.PolicyContext{
		NewResource: *trigger,
		// ExistingResource:    target,
		Policy:              policy,
		AdmissionInfo:       ur.Spec.Context.UserRequestInfo,
		ExcludeGroupRole:    cfg.GetExcludeGroupRole(),
		ExcludeResourceFunc: cfg.ToFilter,
		JSONContext:         ctx,
		NamespaceLabels:     namespaceLabels,
		Client:              dclient,
		AdmissionOperation:  false,
	}

	return policyContext, false, nil
}
