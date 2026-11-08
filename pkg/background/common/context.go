package common

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewBackgroundContext(
	logger logr.Logger,
	dclient dclient.Interface,
	urContext kyvernov2.UpdateRequestSpecContext,
	policy kyvernov1.PolicyInterface,
	trigger *unstructured.Unstructured,
	cfg config.Configuration,
	jp jmespath.Interface,
	namespaceLabels map[string]string,
) (*engine.PolicyContext, error) {
	var new, old unstructured.Unstructured
	var err error

	if urContext.AdmissionRequestInfo.AdmissionRequest != nil {
		new, old, err = admissionutils.ExtractResources(nil, *urContext.AdmissionRequestInfo.AdmissionRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to load request in context: %w", err)
		}
		if new.Object != nil {
			if !check(&new, trigger) {
				return nil, fmt.Errorf("resources don't match, want: %v/%v, got: %v/%v",
					trigger.GetNamespace(), trigger.GetName(), new.GetNamespace(), new.GetName())
			}
		}
	}
	if trigger == nil {
		trigger = &old
	}
	if trigger == nil {
		return nil, fmt.Errorf("trigger resource does not exist")
	}

	// A non-admission DELETE has no admission OldObject to extract. Keep the
	// trigger as the old resource so generation cleanup can still find it.
	if urContext.AdmissionRequestInfo.AdmissionRequest == nil &&
		kyvernov1.AdmissionOperation(urContext.AdmissionRequestInfo.Operation) == kyvernov1.Delete {
		old = *trigger
	}

	var policyContext *engine.PolicyContext
	if urContext.AdmissionRequestInfo.AdmissionRequest == nil {
		operation := kyvernov1.AdmissionOperation(urContext.AdmissionRequestInfo.Operation)
		if operation == "" {
			// Policy reconciliation and periodic resync have no admission request.
			// Evaluate existing triggers as CREATE, consistent with GeneratingPolicy.
			operation = kyvernov1.Create
		}
		policyContext, err = engine.NewPolicyContext(
			jp,
			*trigger,
			operation,
			&urContext.UserRequestInfo,
			cfg,
		)
	} else {
		policyContext, err = engine.NewPolicyContextFromAdmissionRequest(
			jp,
			*urContext.AdmissionRequestInfo.AdmissionRequest,
			urContext.UserRequestInfo,
			trigger.GroupVersionKind(),
			cfg,
		)
	}
	if err != nil {
		return nil, err
	}
	policyContext = policyContext.
		WithPolicy(policy).
		WithNewResource(*trigger).
		WithOldResource(old).
		WithNamespaceLabels(namespaceLabels).
		WithAdmissionOperation(false)
	if err = policyContext.JSONContext().AddResource(trigger.Object); err != nil {
		return nil, fmt.Errorf("failed to load resource in context: %w", err)
	}
	if err = policyContext.JSONContext().AddOldResource(old.Object); err != nil {
		return nil, fmt.Errorf("failed to load resource in context: %w", err)
	}
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
