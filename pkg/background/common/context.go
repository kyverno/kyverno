package common

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewBackgroundContext(
	logger logr.Logger,
	dclient dclient.Interface,
	ur *kyvernov1beta1.UpdateRequest,
	policy kyvernov1.PolicyInterface,
	trigger *unstructured.Unstructured,
	cfg config.Configuration,
	jp jmespath.Interface,
	namespaceLabels map[string]string,
) (*engine.PolicyContext, error) {
	ctx := context.NewContext(jp)
	var new, old unstructured.Unstructured
	var err error

	if ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		if err := ctx.AddRequest(*ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest); err != nil {
			return nil, fmt.Errorf("failed to load request in context: %w", err)
		}

		new, old, err = admissionutils.ExtractResources(nil, *ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to load request in context: %w", err)
		}

		if new.Object != nil {
			if !check(&new, trigger) {
				err := fmt.Errorf("resources don't match")
				return nil, fmt.Errorf("resource %v: %w", ur.Spec.GetResource().String(), err)
			}
		}
	}

	if trigger == nil {
		trigger = &old
	}

	if trigger == nil {
		return nil, fmt.Errorf("trigger resource does not exist")
	}

	err = ctx.AddResource(trigger.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource in context: %w", err)
	}

	err = ctx.AddOldResource(old.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource in context: %w", err)
	}

	err = ctx.AddUserInfo(ur.Spec.Context.UserRequestInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to load SA in context: %w", err)
	}

	err = ctx.AddServiceAccount(ur.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to load UserInfo in context: %w", err)
	}

	if err := ctx.AddImageInfos(trigger, cfg); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	policyContext := engine.NewPolicyContextWithJsonContext(kyvernov1.AdmissionOperation(ur.Spec.Context.AdmissionRequestInfo.Operation), ctx).
		WithPolicy(policy).
		WithNewResource(*trigger).
		WithOldResource(old).
		WithAdmissionInfo(ur.Spec.Context.UserRequestInfo).
		WithNamespaceLabels(namespaceLabels)

	return policyContext, nil
}

func check(admissionRsc, existingRsc *unstructured.Unstructured) bool {
	if existingRsc == nil {
		return admissionRsc == nil
	}
	if admissionRsc.GetName() != existingRsc.GetName() {
		return false
	}
	if admissionRsc.GetNamespace() != existingRsc.GetNamespace() {
		return false
	}
	if admissionRsc.GetKind() != existingRsc.GetKind() {
		return false
	}
	if admissionRsc.GetAPIVersion() != existingRsc.GetAPIVersion() {
		return false
	}
	return true
}
