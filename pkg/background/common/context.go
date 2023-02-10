package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewBackgroundContext(dclient dclient.Interface, ur *kyvernov1beta1.UpdateRequest,
	policy kyvernov1.PolicyInterface,
	trigger *unstructured.Unstructured,
	cfg config.Configuration,
	namespaceLabels map[string]string,
	logger logr.Logger,
) (*engine.PolicyContext, bool, error) {
	ctx := context.NewContext()
	var new, old unstructured.Unstructured
	var err error

	if ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		if err := ctx.AddRequest(ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest); err != nil {
			return nil, false, fmt.Errorf("failed to load request in context: %w", err)
		}

		new, old, err = admissionutils.ExtractResources(nil, ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest)
		if err != nil {
			return nil, false, fmt.Errorf("failed to load request in context: %w", err)
		}

		if !reflect.DeepEqual(new, unstructured.Unstructured{}) {
			if !check(&new, trigger) {
				err := fmt.Errorf("resources don't match")
				return nil, false, fmt.Errorf("resource %v: %w", ur.Spec.GetResource().String(), err)
			}
		}
	}

	if trigger == nil {
		trigger = &old
	}

	if trigger == nil {
		return nil, false, fmt.Errorf("trigger resource does not exist")
	}

	err = ctx.AddResource(trigger.Object)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load resource in contex: %w", err)
	}

	err = ctx.AddOldResource(old.Object)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load resource in context: %w", err)
	}

	err = ctx.AddUserInfo(ur.Spec.Context.UserRequestInfo)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load SA in context: %w", err)
	}

	err = ctx.AddServiceAccount(ur.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load UserInfo in context: %w", err)
	}

	if err := ctx.AddImageInfos(trigger, cfg); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	policyContext := engine.NewPolicyContextWithJsonContext(ctx).
		WithPolicy(policy).
		WithNewResource(*trigger).
		WithOldResource(old).
		WithAdmissionInfo(ur.Spec.Context.UserRequestInfo).
		WithNamespaceLabels(namespaceLabels)

	return policyContext, false, nil
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
