package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/pss"
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
	_ engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// Marshal pod metadata and spec
	podSecurity := rule.Validation.PodSecurity
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	podSpec, metadata, err := getSpec(resource)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while getting new resource", err)
	}
	pod := &corev1.Pod{
		Spec:       *podSpec,
		ObjectMeta: *metadata,
	}
	allowed, pssChecks, err := pss.EvaluatePod(podSecurity, pod)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "failed to parse pod security api version", err)
	}
	podSecurityChecks := engineapi.PodSecurityChecks{
		Level:   podSecurity.Level,
		Version: podSecurity.Version,
		Checks:  pssChecks,
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
		return resource, handlers.WithResponses(
			engineapi.RulePass(rule.Name, engineapi.Validation, msg).WithPodSecurityChecks(podSecurityChecks),
		)
	} else {
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, rule.Name, podSecurity.Level, podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		return resource, handlers.WithResponses(
			engineapi.RuleFail(rule.Name, engineapi.Validation, msg).WithPodSecurityChecks(podSecurityChecks),
		)
	}
}

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
	} else {
		return nil, nil, fmt.Errorf("Could not find correct resource type")
	}
	if err != nil {
		return nil, nil, err
	}
	return podSpec, metadata, err
}
