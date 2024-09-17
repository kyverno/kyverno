package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/pss"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type validatePssHandler struct{}

func NewValidatePssHandler() (handlers.Handler, error) {
	return validatePssHandler{}, nil
}

func (h validatePssHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	engineLoader engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	resource, ruleResp := h.validate(ctx, logger, policyContext, resource, rule, engineLoader)
	return resource, handlers.WithResponses(ruleResp)
}

func (h validatePssHandler) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	engineLoader engineapi.EngineContextLoader,
) (unstructured.Unstructured, *engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		logger.V(3).Info("skipping PSS validation on deleted resource")
		return resource, nil
	}

	// Marshal pod metadata and spec
	podSecurity := rule.Validation.PodSecurity
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	podSpec, metadata, err := getSpec(resource)
	if err != nil {
		return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "Error while getting new resource", err)
	}
	pod := &corev1.Pod{
		Spec:       *podSpec,
		ObjectMeta: *metadata,
	}
	if err != nil {
		return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "failed to parse pod security api version", err)
	}
	allowed, pssChecks, _ := pss.EvaluatePod(rule.Validation.PodSecurity, pod)
	podSecurityChecks := engineapi.PodSecurityChecks{
		Level:   podSecurity.Level,
		Version: podSecurity.Version,
		Checks:  pssChecks,
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
		return resource, engineapi.RulePass(rule.Name, engineapi.Validation, msg).WithPodSecurityChecks(podSecurityChecks)
	} else {
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, rule.Name, podSecurity.Level, podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		ruleResponse := engineapi.RuleFail(rule.Name, engineapi.Validation, msg).WithPodSecurityChecks(podSecurityChecks)
		allowExisitingViolations := rule.HasValidateAllowExistingViolations()
		if engineutils.IsUpdateRequest(policyContext) && allowExisitingViolations {
			logger.V(4).Info("is update request")
			priorResp, err := h.validateOldObject(ctx, logger, policyContext, resource, rule, engineLoader)
			if err != nil {
				logger.V(2).Info("warning: failed to validate old object, skipping the rule evaluation as pre-existing violations are allowed", "rule", rule.Name, "error", err.Error())
				return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "failed to validate old object, skipping as preexisting violations are allowed")
			}

			if ruleResponse.Status() == priorResp.Status() {
				logger.V(3).Info("skipping modified resource as validation results have not changed", "oldResp", priorResp, "newResp", ruleResponse)
				if ruleResponse.Status() == engineapi.RuleStatusPass {
					return resource, ruleResponse
				}
				return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "skipping modified resource as validation results have not changed")
			}
			logger.V(4).Info("old object response is different", "oldResp", priorResp, "newResp", ruleResponse)
		}

		return resource, ruleResponse
	}
}

func (h validatePssHandler) validateOldObject(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	engineLoader engineapi.EngineContextLoader,
) (*engineapi.RuleResponse, error) {
	if policyContext.Operation() != kyvernov1.Update {
		return nil, nil
	}

	newResource := policyContext.NewResource()
	oldResource := policyContext.OldResource()
	emptyResource := unstructured.Unstructured{}

	if ok := matchResource(oldResource, rule); !ok {
		return nil, nil
	}
	if err := policyContext.SetResources(emptyResource, oldResource); err != nil {
		return nil, errors.Wrapf(err, "failed to set resources")
	}
	if err := policyContext.SetOperation(kyvernov1.Create); err != nil { // simulates the condition when old object was "created"
		return nil, errors.Wrapf(err, "failed to set operation")
	}

	_, resp := h.validate(ctx, logger, policyContext, oldResource, rule, engineLoader)

	if err := policyContext.SetResources(oldResource, newResource); err != nil {
		return nil, errors.Wrapf(err, "failed to reset resources")
	}

	if err := policyContext.SetOperation(kyvernov1.Update); err != nil {
		return nil, errors.Wrapf(err, "failed to reset operation")
	}

	return resp, nil
}

// Extract container names from PSS error details. Here are some example inputs:
// - "containers \"nginx\", \"busybox\" must set securityContext.allowPrivilegeEscalation=false"
// - "containers \"nginx\", \"busybox\" must set securityContext.capabilities.drop=[\"ALL\"]"
// - "pod or containers \"nginx\", \"busybox\" must set securityContext.runAsNonRoot=true"
// - "pod or containers \"nginx\", \"busybox\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\""
// - "pod or container \"nginx\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\""
// - "container \"nginx\" must set securityContext.allowPrivilegeEscalation=false"

func getSpec(resource unstructured.Unstructured) (podSpec *corev1.PodSpec, metadata *metav1.ObjectMeta, err error) {
	kind := resource.GetKind()

	if kind == "DaemonSet" || kind == "Deployment" || kind == "Job" || kind == "StatefulSet" || kind == "ReplicaSet" || kind == "ReplicationController" {
		var deployment appsv1.Deployment
		resourceBytes, err := resource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &deployment)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &deployment.Spec.Template.Spec
		metadata = &deployment.Spec.Template.ObjectMeta
		return podSpec, metadata, nil
	} else if kind == "CronJob" {
		var cronJob batchv1.CronJob
		resourceBytes, err := resource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &cronJob)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &cronJob.Spec.JobTemplate.Spec.Template.Spec
		metadata = &cronJob.Spec.JobTemplate.ObjectMeta
		return podSpec, metadata, nil
	} else if kind == "Pod" {
		var pod corev1.Pod
		resourceBytes, err := resource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &pod)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &pod.Spec
		metadata = &pod.ObjectMeta
		return podSpec, metadata, nil
	}

	return nil, nil, fmt.Errorf("could not find correct resource type")
}
