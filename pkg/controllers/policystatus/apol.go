package policystatus

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/kyverno/kyverno/pkg/controllers/webhook"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c controller) updateApolStatus(ctx context.Context, apol *policiesv1alpha1.AuthorizingPolicy) error {
	key := webhook.BuildRecorderKey(webhook.AuthorizingPolicyType, apol.Name, "")
	log := logger.WithValues("policy", apol.Name, "kind", "AuthorizingPolicy")
	originalStatus := apol.Status

	compiler := apolcompiler.NewCompiler()
	_, compileErrors := compiler.Compile(apol)
	status := &apol.Status.ConditionStatus

	if len(compileErrors) > 0 {
		log.V(2).Info("policy has compilation errors", "errors", compileErrors)
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionFalse, compileErrors.ToAggregate().Error())
		status.Message = compileErrors.ToAggregate().Error()
	} else {
		log.V(2).Info("policy compiled successfully")
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionTrue, "Policy compiled and cached.")
		status.Message = ""
	}

	if ready, ok := c.polStateRecorder.Ready(key); ok {
		if ready {
			status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "Webhook configured.")
		} else {
			log.V(2).Info("webhook not yet configured")
			status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "Policy is not configured in the webhook.")
			if len(compileErrors) == 0 {
				status.Message = "Waiting for webhook configuration"
			}
		}
	}

	ready := true
	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}
	status.Ready = &ready

	if datautils.DeepEqual(originalStatus, apol.Status) {
		return nil
	}
	_, err := c.dclient.UpdateStatusResource(ctx, policiesv1alpha1.GroupVersion.String(), webhook.AuthorizingPolicyType, "", apol, false)
	return err
}

func decodeAuthorizingPolicy(obj map[string]interface{}) (*policiesv1alpha1.AuthorizingPolicy, error) {
	var apol policiesv1alpha1.AuthorizingPolicy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj, &apol, true); err != nil {
		return nil, err
	}
	return &apol, nil
}
