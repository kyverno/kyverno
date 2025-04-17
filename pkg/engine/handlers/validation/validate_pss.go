package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/pss"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/pkg/errors"
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
	engineLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	resource, ruleResp := h.validate(ctx, logger, policyContext, resource, rule, engineLoader, exceptions)
	return resource, handlers.WithResponses(ruleResp)
}

func (h validatePssHandler) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	engineLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, *engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		logger.V(3).Info("skipping PSS validation on deleted resource")
		return resource, nil
	}

	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		var polex kyvernov2.PolicyException
		hasPodSecurity := true

		for i, exception := range matchedExceptions {
			if !exception.HasPodSecurity() {
				hasPodSecurity = false
				polex = matchedExceptions[i]
				break
			}
		}

		if !hasPodSecurity {
			key, err := cache.MetaNamespaceKeyFunc(&polex)
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", polex.GetNamespace(), "name", polex.GetName())
				return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "failed to compute exception key", err, rule.ReportProperties)
			}
			logger.V(3).Info("policy rule is skipped due to policy exception", "exception", key)
			return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exception "+key, rule.ReportProperties).WithExceptions([]engineapi.GenericException{engineapi.NewPolicyException(&polex)})
		}
	}

	// Marshal pod metadata and spec
	podSecurity := rule.Validation.PodSecurity
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	podSpec, metadata, err := getSpec(resource)
	if err != nil {
		return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "Error while getting new resource", err, rule.ReportProperties)
	}
	pod := &corev1.Pod{
		Spec:       *podSpec,
		ObjectMeta: *metadata,
	}
	levelVersion, err := pss.ParseVersion(podSecurity.Level, podSecurity.Version)
	if err != nil {
		return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "failed to parse pod security api version", err, rule.ReportProperties)
	}
	allowed, pssChecks := pss.EvaluatePod(levelVersion, podSecurity.Exclude, pod)
	podSecurityChecks := engineapi.PodSecurityChecks{
		Level:   podSecurity.Level,
		Version: podSecurity.Version,
		Checks:  pssChecks,
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
		return resource, engineapi.RulePass(rule.Name, engineapi.Validation, msg, rule.ReportProperties).WithPodSecurityChecks(podSecurityChecks)
	} else {
		// apply pod security exceptions if exist
		genericExceptions := make([]engineapi.GenericException, 0, len(matchedExceptions))
		var excludes []kyvernov1.PodSecurityStandard
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, engineapi.RuleError(rule.Name, engineapi.Validation, "failed to compute exception key", err, rule.ReportProperties)
			}
			keys = append(keys, key)
			excludes = append(excludes, exception.Spec.PodSecurity...)
			genericExceptions = append(genericExceptions, engineapi.NewPolicyException(&exception))
		}

		pssChecks, err = pss.ApplyPodSecurityExclusion(levelVersion, excludes, pssChecks, pod)
		if len(pssChecks) == 0 && err == nil {
			podSecurityChecks.Checks = pssChecks
			logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
			return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exceptions "+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(genericExceptions).WithPodSecurityChecks(podSecurityChecks)
		}
		pssChecks = convertChecks(pssChecks, resource.GetKind())
		pssChecks = addImages(pssChecks, policyContext.JSONContext().ImageInfo())
		podSecurityChecks.Checks = pssChecks
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, rule.Name, podSecurity.Level, podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		ruleResponse := engineapi.RuleFail(rule.Name, engineapi.Validation, msg, rule.ReportProperties).WithPodSecurityChecks(podSecurityChecks)
		var action kyvernov1.ValidationFailureAction
		if rule.Validation.FailureAction != nil {
			action = *rule.Validation.FailureAction
		} else {
			action = policyContext.Policy().GetSpec().ValidationFailureAction
		}

		// process the old object for UPDATE admission requests in case of enforce policies
		if action.Enforce() {
			allowExisitingViolations := rule.HasValidateAllowExistingViolations()
			if engineutils.IsUpdateRequest(policyContext) && allowExisitingViolations {
				priorResp, err := h.validateOldObject(ctx, logger, policyContext, resource, rule, engineLoader, exceptions)
				if err != nil {
					logger.V(4).Info("warning: failed to validate old object", "rule", rule.Name, "error", err.Error())
					return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "failed to validate old object", rule.ReportProperties)
				}

				if priorResp != nil && ruleResponse.Status() == priorResp.Status() {
					logger.V(2).Info("warning: skipping the rule evaluation as pre-existing violations are allowed", "oldResp", priorResp, "newResp", ruleResponse)
					if ruleResponse.Status() == engineapi.RuleStatusPass {
						return resource, ruleResponse
					}
					return resource, engineapi.RuleSkip(rule.Name, engineapi.Validation, "skipping the rule evaluation as pre-existing violations are allowed", rule.ReportProperties)
				}
			}
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
	exceptions []*kyvernov2.PolicyException,
) (resp *engineapi.RuleResponse, err error) {
	if policyContext.Operation() != kyvernov1.Update {
		return nil, nil
	}

	newResource := policyContext.NewResource()
	oldResource := policyContext.OldResource()
	emptyResource := unstructured.Unstructured{}

	if err = policyContext.SetResources(emptyResource, oldResource); err != nil {
		return nil, errors.Wrapf(err, "failed to set resources")
	}
	if err = policyContext.SetOperation(kyvernov1.Create); err != nil { // simulates the condition when old object was "created"
		return nil, errors.Wrapf(err, "failed to set operation")
	}

	defer func() {
		if err = policyContext.SetResources(oldResource, newResource); err != nil {
			logger.Error(errors.Wrapf(err, "failed to reset resources"), "")
		}

		if err = policyContext.SetOperation(kyvernov1.Update); err != nil {
			logger.Error(errors.Wrapf(err, "failed to reset operations"), "")
		}
	}()

	if ok := matchResource(logger, oldResource, rule, policyContext.NamespaceLabels(), policyContext.Policy().GetNamespace(), kyvernov1.Create, policyContext.JSONContext()); !ok {
		return
	}

	_, resp = h.validate(ctx, logger, policyContext, oldResource, rule, engineLoader, exceptions)

	return
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

// Extract container names from PSS error details. Here are some example inputs:
// - "containers \"nginx\", \"busybox\" must set securityContext.allowPrivilegeEscalation=false"
// - "containers \"nginx\", \"busybox\" must set securityContext.capabilities.drop=[\"ALL\"]"
// - "pod or containers \"nginx\", \"busybox\" must set securityContext.runAsNonRoot=true"
// - "pod or containers \"nginx\", \"busybox\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\""
// - "pod or container \"nginx\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\""
// - "container \"nginx\" must set securityContext.allowPrivilegeEscalation=false"
var regexContainerNames = regexp.MustCompile(`container(?:s)?\s*(.*?)\s*must`)

func addImages(checks []pssutils.PSSCheckResult, imageInfos map[string]map[string]api.ImageInfo) []pssutils.PSSCheckResult {
	for i, check := range checks {
		text := check.CheckResult.ForbiddenDetail
		matches := regexContainerNames.FindAllStringSubmatch(text, -1)
		if len(matches) > 0 {
			s := strings.ReplaceAll(matches[0][1], "\"", "")
			s = strings.ReplaceAll(s, " ", "")
			containerNames := strings.Split(s, ",")
			checks[i].Images = getImages(containerNames, imageInfos)
		}
	}
	return checks
}

// return image references for containers
func getImages(containerNames []string, imageInfos map[string]map[string]api.ImageInfo) []string {
	images := make([]string, 0, len(containerNames))
	for _, cn := range containerNames {
		image := getImageReference(cn, imageInfos)
		images = append(images, image)
	}
	return images
}

// return an image references for a container name
// if the image is not found, the name is returned
func getImageReference(name string, imageInfos map[string]map[string]api.ImageInfo) string {
	if containers, ok := imageInfos["containers"]; ok {
		if imageInfo, ok := containers[name]; ok {
			return imageInfo.String()
		}
	}
	if initContainers, ok := imageInfos["initContainers"]; ok {
		if imageInfo, ok := initContainers[name]; ok {
			return imageInfo.String()
		}
	}
	if ephemeralContainers, ok := imageInfos["ephemeralContainers"]; ok {
		if imageInfo, ok := ephemeralContainers[name]; ok {
			return imageInfo.String()
		}
	}
	return name
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
