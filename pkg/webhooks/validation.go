package webhooks

import (
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policystatus"
	"reflect"
	"sort"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionReviewLatency "github.com/kyverno/kyverno/pkg/metrics/admissionreviewlatency"
	policyRuleExecutionLatency "github.com/kyverno/kyverno/pkg/metrics/policyruleexecutionlatency"
	policyRuleResults "github.com/kyverno/kyverno/pkg/metrics/policyruleresults"
	"github.com/kyverno/kyverno/pkg/policyreport"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type validationHandler struct {
	log logr.Logger
	statusListener policystatus.Listener
	eventGen event.Interface
	prGenerator policyreport.GeneratorInterface
}


// handleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (v *validationHandler) handleValidation(
	promConfig *metrics.PromConfig,
	request *v1beta1.AdmissionRequest,
	policies []*kyverno.ClusterPolicy,
	policyContext *engine.PolicyContext,
	namespaceLabels map[string]string,
	admissionRequestTimestamp int64) (bool, string) {

	if len(policies) == 0 {
		return true, ""
	}

	resourceName := getResourceName(request)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		deletionTimeStamp = policyContext.NewResource.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = policyContext.OldResource.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == v1beta1.Update {
		return true, ""
	}

	var engineResponses []*response.EngineResponse
	var triggeredPolicies []kyverno.ClusterPolicy
	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.Name)
		policyContext.Policy = *policy
		policyContext.NamespaceLabels = namespaceLabels
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}

		// registering the kyverno_policy_rule_results_info metric concurrently
		go registerPolicyRuleResultsMetricValidation(promConfig, logger, string(request.Operation), policyContext.Policy, *engineResponse, admissionRequestTimestamp)
		// registering the kyverno_policy_rule_execution_latency_milliseconds metric concurrently
		go registerPolicyRuleExecutionLatencyMetricValidate(promConfig, logger, string(request.Operation), policyContext.Policy, *engineResponse, admissionRequestTimestamp)

		engineResponses = append(engineResponses, engineResponse)
		triggeredPolicies = append(triggeredPolicies, *policy)
		v.statusListener.Update(validateStats{
			resp:      engineResponse,
			namespace: policy.Namespace,
		})

		if !engineResponse.IsSuccessful() {
			logger.V(2).Info("validation failed", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		if len(engineResponse.GetSuccessRules()) > 0 {
			logger.V(2).Info("validation passed", "policy", policy.Name)
		}
	}

	// If Validation fails then reject the request
	// no violations will be created on "enforce"
	blocked := toBlockResource(engineResponses, logger)

	// REPORTING EVENTS
	// Scenario 1:
	//   resource is blocked, as there is a policy in "enforce" mode that failed.
	//   create an event on the policy to inform the resource request was blocked
	// Scenario 2:
	//   some/all policies failed to apply on the resource. a policy violation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 3:
	//   all policies were applied successfully.
	//   create an event on the resource
	events := generateEvents(engineResponses, blocked, (request.Operation == v1beta1.Update), logger)
	v.eventGen.Add(events...)
	if blocked {
		logger.V(4).Info("resource blocked")
		//registering the kyverno_admission_review_latency_milliseconds metric concurrently
		admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
		go registerAdmissionReviewLatencyMetricValidate(promConfig, logger, string(request.Operation), engineResponses, triggeredPolicies, admissionReviewLatencyDuration, admissionRequestTimestamp)
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	if request.Operation == v1beta1.Delete {
		v.prGenerator.Add(buildDeletionPrInfo(policyContext.OldResource))
		return true, ""
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	v.prGenerator.Add(prInfos...)

	//registering the kyverno_admission_review_latency_milliseconds metric concurrently
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go registerAdmissionReviewLatencyMetricValidate(promConfig, logger, string(request.Operation), engineResponses, triggeredPolicies, admissionReviewLatencyDuration, admissionRequestTimestamp)

	return true, ""
}

func getResourceName(request *v1beta1.AdmissionRequest) string {
	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	return resourceName
}

func registerPolicyRuleResultsMetricValidation(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleResults.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleResults.ParsePromMetrics(*promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
}

func registerPolicyRuleExecutionLatencyMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleExecutionLatency.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleExecutionLatency.ParsePromMetrics(*promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
}

func registerAdmissionReviewLatencyMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse, triggeredPolicies []kyverno.ClusterPolicy, admissionReviewLatencyDuration int64, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := admissionReviewLatency.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_latency_milliseconds metrics")
	}
	if err := admissionReviewLatency.ParsePromMetrics(*promConfig.Metrics).ProcessEngineResponses(engineResponses, triggeredPolicies, admissionReviewLatencyDuration, resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_latency_milliseconds metrics")
	}
}

func buildDeletionPrInfo(oldR unstructured.Unstructured) policyreport.Info {
	return policyreport.Info{
		Namespace: oldR.GetNamespace(),
		Results: []policyreport.EngineResponseResult{
			{Resource: response.ResourceSpec{
				Kind:       oldR.GetKind(),
				APIVersion: oldR.GetAPIVersion(),
				Namespace:  oldR.GetNamespace(),
				Name:       oldR.GetName(),
				UID:        string(oldR.GetUID()),
			}},
		},
	}
}

type validateStats struct {
	resp      *response.EngineResponse
	namespace string
}

func (vs validateStats) PolicyName() string {
	if vs.namespace == "" {
		return vs.resp.PolicyResponse.Policy
	}
	return vs.namespace + "/" + vs.resp.PolicyResponse.Policy

}

func (vs validateStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {
	if reflect.DeepEqual(response.EngineResponse{}, vs.resp) {
		return status
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range status.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range vs.resp.PolicyResponse.Rules {
		ruleStat := nameToRule[rule.Name]
		ruleStat.Name = rule.Name

		averageOver := int64(ruleStat.AppliedCount + ruleStat.FailedCount)
		ruleStat.ExecutionTime = updateAverageTime(
			rule.ProcessingTime,
			ruleStat.ExecutionTime,
			averageOver).String()

		if rule.Success {
			status.RulesAppliedCount++
			ruleStat.AppliedCount++
		} else {
			status.RulesFailedCount++
			ruleStat.FailedCount++
			if vs.resp.PolicyResponse.ValidationFailureAction == "enforce" {
				status.ResourcesBlockedCount++
				ruleStat.ResourcesBlockedCount++
			}
		}

		nameToRule[rule.Name] = ruleStat
	}

	var policyAverageExecutionTime time.Duration
	var ruleStats = make([]v1.RuleStats, 0, len(nameToRule))
	for _, ruleStat := range nameToRule {
		executionTime, err := time.ParseDuration(ruleStat.ExecutionTime)
		if err == nil {
			policyAverageExecutionTime += executionTime
		}
		ruleStats = append(ruleStats, ruleStat)
	}

	sort.Slice(ruleStats, func(i, j int) bool {
		return ruleStats[i].Name < ruleStats[j].Name
	})

	status.AvgExecutionTime = policyAverageExecutionTime.String()
	status.Rules = ruleStats

	return status
}
