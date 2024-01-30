package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/pss"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
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
	exceptions []kyvernov2beta1.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		logger.V(3).Info("skipping PSS validation on deleted resource")
		return resource, nil
	}

	// check if there is a policy exception matches the incoming resource
	exception := engineutils.MatchesException(exceptions, policyContext, logger)
	if exception != nil && !exception.HasPodSecurity() {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
		} else {
			logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule skipped due to policy exception "+key).WithException(exception),
			)
		}
	}

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
	levelVersion, err := pss.ParseVersion(podSecurity.Level, podSecurity.Version)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "failed to parse pod security api version", err)
	}
	allowed, pssChecks := pss.EvaluatePod(levelVersion, podSecurity.Exclude, pod)
	pssChecks = convertChecks(pssChecks, resource.GetKind())
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
		// apply pod security exceptions if exist
		if exception != nil && exception.HasPodSecurity() {
			pssChecks, err = pss.ApplyPodSecurityExclusion(levelVersion, exception.Spec.PodSecurity, pssChecks, pod)
			if len(pssChecks) == 0 && err == nil {
				key, err := cache.MetaNamespaceKeyFunc(exception)
				if err != nil {
					logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
					return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
				} else {
					podSecurityChecks.Checks = pssChecks
					logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
					return resource, handlers.WithResponses(
						engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule skipped due to policy exception "+key).WithException(exception).WithPodSecurityChecks(podSecurityChecks),
					)
				}
			}
		}
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, rule.Name, podSecurity.Level, podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		return resource, handlers.WithResponses(
			engineapi.RuleFail(rule.Name, engineapi.Validation, msg).WithPodSecurityChecks(podSecurityChecks),
		)
	}
}

func convertChecks(checks []pssutils.PSSCheckResult, kind string) (newChecks []pssutils.PSSCheckResult) {
	if kind == "DaemonSet" || kind == "Deployment" || kind == "Job" || kind == "StatefulSet" || kind == "ReplicaSet" || kind == "ReplicationController" {
		for i := range checks {
			for j := range *checks[i].CheckResult.ErrList {
				(*checks[i].CheckResult.ErrList)[j].Field = strings.ReplaceAll((*checks[i].CheckResult.ErrList)[j].Field, "spec", "spec.template.spec")
			}
		}
	} else if kind == "CronJob" {
		for i := range checks {
			for j := range *checks[i].CheckResult.ErrList {
				(*checks[i].CheckResult.ErrList)[j].Field = strings.ReplaceAll((*checks[i].CheckResult.ErrList)[j].Field, "spec", "spec.jobTemplate.spec.template.spec")
			}
		}
	}
	for i := range checks {
		for j := range *checks[i].CheckResult.ErrList {
			(*checks[i].CheckResult.ErrList)[j].Field = strings.ReplaceAll((*checks[i].CheckResult.ErrList)[j].Field, "metadata", "spec.template.metadata")
		}
	}

	return checks
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
		return nil, nil, fmt.Errorf("could not find correct resource type")
	}
	if err != nil {
		return nil, nil, err
	}
	return podSpec, metadata, err
}
