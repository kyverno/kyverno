package webhooks

import (
	contextdefault "context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	gen "github.com/kyverno/kyverno/pkg/generate"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyRuleExecutionLatency "github.com/kyverno/kyverno/pkg/metrics/policyruleexecutionlatency"
	policyRuleResults "github.com/kyverno/kyverno/pkg/metrics/policyruleresults"
	kyvernoutils "github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

//HandleGenerate handles admission-requests for policies with generate rules
func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []*kyverno.ClusterPolicy, ctx *context.Context, userRequestInfo kyverno.RequestInfo, dynamicConfig config.Interface, admissionRequestTimestamp int64, latencySender *chan int64, triggeredGeneratePoliciesSender *chan []kyverno.ClusterPolicy, generateEngineResponsesSender *chan []*response.EngineResponse) {
	logger := ws.log.WithValues("action", "generation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	logger.V(4).Info("incoming request")

	var engineResponses []*response.EngineResponse
	var triggeredGeneratePolicies []kyverno.ClusterPolicy
	if (request.Operation == v1beta1.Create || request.Operation == v1beta1.Update) && len(policies) != 0 {
		// convert RAW to unstructured
		new, old, err := kyvernoutils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "failed to extract resource")
		}

		policyContext := &engine.PolicyContext{
			NewResource:         new,
			OldResource:         old,
			AdmissionInfo:       userRequestInfo,
			ExcludeGroupRole:    dynamicConfig.GetExcludeGroupRole(),
			ExcludeResourceFunc: ws.configHandler.ToFilter,
			ResourceCache:       ws.resCache,
			JSONContext:         ctx,
			Client:              ws.client,
		}

		for _, policy := range policies {
			var rules []response.RuleResponse
			policyContext.Policy = *policy
			if request.Kind.Kind != "Namespace" && request.Namespace != "" {
				policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, ws.nsLister, logger)
			}
			engineResponse := engine.Generate(policyContext)
			for _, rule := range engineResponse.PolicyResponse.Rules {
				if !rule.Success {
					ws.deleteGR(logger, engineResponse)
					continue
				}
				rules = append(rules, rule)
			}

			if len(rules) > 0 {
				engineResponse.PolicyResponse.Rules = rules
				// some generate rules do apply to the resource
				engineResponses = append(engineResponses, engineResponse)
				triggeredGeneratePolicies = append(triggeredGeneratePolicies, *policy)
				ws.statusListener.Update(generateStats{
					resp: engineResponse,
				})
			}
			// registering the kyverno_policy_rule_results_info metric concurrently
			go ws.registerPolicyRuleResultsMetricGeneration(logger, string(request.Operation), *policy, *engineResponse, admissionRequestTimestamp)

			// registering the kyverno_policy_rule_execution_latency_milliseconds metric concurrently
			go ws.registerPolicyRuleExecutionLatencyMetricGenerate(logger, string(request.Operation), *policy, *engineResponse, admissionRequestTimestamp)
		}

		// Adds Generate Request to a channel(queue size 1000) to generators
		if failedResponse := applyGenerateRequest(ws.grGenerator, userRequestInfo, request.Operation, engineResponses...); err != nil {
			// report failure event
			for _, failedGR := range failedResponse {
				events := failedEvents(fmt.Errorf("failed to create Generate Request: %v", failedGR.err), failedGR.gr, new)
				ws.eventGen.Add(events...)
			}
		}
	}

	if request.Operation == v1beta1.Update {
		ws.handleUpdate(request, policies)
	}

	// sending the admission request latency to other goroutine (reporting the metrics) over the channel
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	*latencySender <- admissionReviewLatencyDuration
	*triggeredGeneratePoliciesSender <- triggeredGeneratePolicies
	*generateEngineResponsesSender <- engineResponses
}

func (ws *WebhookServer) registerPolicyRuleResultsMetricGeneration(logger logr.Logger, resourceRequestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleResults.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleResults.ParsePromMetrics(*ws.promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
}

func (ws *WebhookServer) registerPolicyRuleExecutionLatencyMetricGenerate(logger logr.Logger, resourceRequestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleExecutionLatency.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleExecutionLatency.ParsePromMetrics(*ws.promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
}

//handleUpdate handles admission-requests for update
func (ws *WebhookServer) handleUpdate(request *v1beta1.AdmissionRequest, policies []*kyverno.ClusterPolicy) {
	logger := ws.log.WithValues("action", "generation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	resource, err := enginutils.ConvertToUnstructured(request.OldObject.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels["generate.kyverno.io/clone-policy-name"] != "" {
		ws.handleUpdateCloneSourceResource(resLabels, logger)
	}

	if resLabels["app.kubernetes.io/managed-by"] == "kyverno" && resLabels["policy.kyverno.io/synchronize"] == "enable" && request.Operation == v1beta1.Update {
		ws.handleUpdateTargetResource(request, policies, resLabels, logger)
	}
}

//handleUpdateCloneSourceResource - handles update of clone source for generate policy
func (ws *WebhookServer) handleUpdateCloneSourceResource(resLabels map[string]string, logger logr.Logger) {
	policyNames := strings.Split(resLabels["generate.kyverno.io/clone-policy-name"], ",")
	for _, policyName := range policyNames {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{
			"generate.kyverno.io/policy-name": policyName,
		}))

		grList, err := ws.grLister.List(selector)
		if err != nil {
			logger.Error(err, "failed to get generate request for the resource", "label", "generate.kyverno.io/policy-name")
			return
		}
		for _, gr := range grList {
			ws.updateAnnotationInGR(gr, logger)
		}
	}
}

// updateAnnotationInGR - function used to update GR annotation
// updating GR will trigger reprocessing of GR and recreation/updation of generated resource
func (ws *WebhookServer) updateAnnotationInGR(gr *v1.GenerateRequest, logger logr.Logger) {
	grAnnotations := gr.Annotations
	if len(grAnnotations) == 0 {
		grAnnotations = make(map[string]string)
	}
	grAnnotations["generate.kyverno.io/updation-time"] = time.Now().String()
	gr.SetAnnotations(grAnnotations)
	_, err := ws.kyvernoClient.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(contextdefault.TODO(), gr, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "failed to update generate request for the resource", "generate request", gr.Name)
		return
	}
}

//handleUpdateTargetResource - handles update of target resource for generate policy
func (ws *WebhookServer) handleUpdateTargetResource(request *v1beta1.AdmissionRequest, policies []*v1.ClusterPolicy, resLabels map[string]string, logger logr.Logger) {
	enqueueBool := false
	newRes, err := enginutils.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	policyName := resLabels["policy.kyverno.io/policy-name"]
	targetSourceName := newRes.GetName()
	targetSourceKind := newRes.GetKind()

	policy, err := ws.kyvernoClient.KyvernoV1().ClusterPolicies().Get(contextdefault.TODO(), policyName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get policy from kyverno client.", "policy name", policyName)
		return
	}

	for _, rule := range policy.Spec.Rules {
		if rule.Generation.Kind == targetSourceKind && rule.Generation.Name == targetSourceName {
			updatedRule, err := getGeneratedByResource(newRes, resLabels, ws.client, rule, logger)
			if err != nil {
				logger.V(4).Info("skipping generate policy and resource pattern validaton", "error", err)
			} else {
				data := updatedRule.Generation.DeepCopy().Data
				if data != nil {
					if _, err := gen.ValidateResourceWithPattern(logger, newRes.Object, data); err != nil {
						enqueueBool = true
						break
					}
				}

				cloneName := updatedRule.Generation.Clone.Name
				if cloneName != "" {
					obj, err := ws.client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
					if err != nil {
						logger.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
						continue
					}

					sourceObj, newResObj := stripNonPolicyFields(obj.Object, newRes.Object, logger)

					if _, err := gen.ValidateResourceWithPattern(logger, newResObj, sourceObj); err != nil {
						enqueueBool = true
						break
					}
				}
			}
		}
	}

	if enqueueBool {
		grName := resLabels["policy.kyverno.io/gr-name"]
		gr, err := ws.grLister.Get(grName)
		if err != nil {
			logger.Error(err, "failed to get generate request", "name", grName)
			return
		}
		ws.updateAnnotationInGR(gr, logger)
	}
}

func getGeneratedByResource(newRes *unstructured.Unstructured, resLabels map[string]string, client *client.Client, rule v1.Rule, logger logr.Logger) (v1.Rule, error) {
	var apiVersion, kind, name, namespace string
	sourceRequest := &v1beta1.AdmissionRequest{}
	kind = resLabels["kyverno.io/generated-by-kind"]
	name = resLabels["kyverno.io/generated-by-name"]
	if kind != "Namespace" {
		namespace = resLabels["kyverno.io/generated-by-namespace"]
	}
	obj, err := client.GetResource(apiVersion, kind, namespace, name)
	if err != nil {
		logger.Error(err, "source resource not found.")
		return rule, err
	}
	rawObj, err := json.Marshal(obj)
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return rule, err
	}
	sourceRequest.Object.Raw = rawObj
	sourceRequest.Operation = "CREATE"
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(sourceRequest); err != nil {
		logger.Error(err, "failed to load incoming request in context")
		return rule, err
	}
	if rule, err = variables.SubstituteAllInRule(logger, ctx, rule); err != nil {
		logger.Error(err, "variable substitution failed for rule %s", rule.Name)
		return rule, err
	}
	return rule, nil
}

//stripNonPolicyFields - remove feilds which get updated with each request by kyverno and are non policy fields
func stripNonPolicyFields(obj, newRes map[string]interface{}, logger logr.Logger) (map[string]interface{}, map[string]interface{}) {

	if metadata, found := obj["metadata"]; found {
		requiredMetadataInObj := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			delete(annotations.(map[string]interface{}), "kubectl.kubernetes.io/last-applied-configuration")
			requiredMetadataInObj["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			delete(labels.(map[string]interface{}), "generate.kyverno.io/clone-policy-name")
			requiredMetadataInObj["labels"] = labels
		}
		obj["metadata"] = requiredMetadataInObj
	}

	if metadata, found := newRes["metadata"]; found {
		requiredMetadataInNewRes := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			requiredMetadataInNewRes["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			requiredMetadataInNewRes["labels"] = labels
		}
		newRes["metadata"] = requiredMetadataInNewRes
	}

	if _, found := obj["status"]; found {
		delete(obj, "status")
	}

	if _, found := obj["spec"]; found {
		delete(obj["spec"].(map[string]interface{}), "tolerations")
	}

	if dataMap, found := obj["data"]; found {
		keyInData := make([]string, 0)
		switch dataMap.(type) {
		case map[string]interface{}:
			for k := range dataMap.(map[string]interface{}) {
				keyInData = append(keyInData, k)
			}
		}

		if len(keyInData) > 0 {
			for _, dataKey := range keyInData {
				originalResourceData := dataMap.(map[string]interface{})[dataKey]
				replaceData := strings.Replace(originalResourceData.(string), "\n", "", -1)
				dataMap.(map[string]interface{})[dataKey] = replaceData

				newResourceData := newRes["data"].(map[string]interface{})[dataKey]
				replacenewResourceData := strings.Replace(newResourceData.(string), "\n", "", -1)
				newRes["data"].(map[string]interface{})[dataKey] = replacenewResourceData
			}
		} else {
			logger.V(4).Info("data is not of type map[string]interface{}")
		}
	}

	return obj, newRes
}

//HandleDelete handles admission-requests for delete
func (ws *WebhookServer) handleDelete(request *v1beta1.AdmissionRequest) {
	logger := ws.log.WithValues("action", "generation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	resource, err := enginutils.ConvertToUnstructured(request.OldObject.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels["app.kubernetes.io/managed-by"] == "kyverno" && resLabels["policy.kyverno.io/synchronize"] == "enable" && request.Operation == v1beta1.Delete {
		grName := resLabels["policy.kyverno.io/gr-name"]
		gr, err := ws.grLister.Get(grName)
		if err != nil {
			logger.Error(err, "failed to get generate request", "name", grName)
			return
		}
		ws.updateAnnotationInGR(gr, logger)
	}
}

func (ws *WebhookServer) deleteGR(logger logr.Logger, engineResponse *response.EngineResponse) {
	logger.V(4).Info("querying all generate requests")
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		"generate.kyverno.io/policy-name":        engineResponse.PolicyResponse.Policy,
		"generate.kyverno.io/resource-name":      engineResponse.PolicyResponse.Resource.Name,
		"generate.kyverno.io/resource-kind":      engineResponse.PolicyResponse.Resource.Kind,
		"generate.kyverno.io/resource-namespace": engineResponse.PolicyResponse.Resource.Namespace,
	}))

	grList, err := ws.grLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to get generate request for the resource", "kind", engineResponse.PolicyResponse.Resource.Kind, "name", engineResponse.PolicyResponse.Resource.Name, "namespace", engineResponse.PolicyResponse.Resource.Namespace)
		return
	}

	for _, v := range grList {
		err := ws.kyvernoClient.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Delete(contextdefault.TODO(), v.GetName(), metav1.DeleteOptions{})
		if err != nil {
			logger.Error(err, "failed to update gr")
		}
	}
}

func applyGenerateRequest(gnGenerator generate.GenerateRequests, userRequestInfo kyverno.RequestInfo,
	action v1beta1.Operation, engineResponses ...*response.EngineResponse) (failedGenerateRequest []generateRequestResponse) {

	for _, er := range engineResponses {
		gr := transform(userRequestInfo, er)
		if err := gnGenerator.Apply(gr, action); err != nil {
			failedGenerateRequest = append(failedGenerateRequest, generateRequestResponse{gr: gr, err: err})
		}
	}

	return
}

func transform(userRequestInfo kyverno.RequestInfo, er *response.EngineResponse) kyverno.GenerateRequestSpec {
	gr := kyverno.GenerateRequestSpec{
		Policy: er.PolicyResponse.Policy,
		Resource: kyverno.ResourceSpec{
			Kind:       er.PolicyResponse.Resource.Kind,
			Namespace:  er.PolicyResponse.Resource.Namespace,
			Name:       er.PolicyResponse.Resource.Name,
			APIVersion: er.PolicyResponse.Resource.APIVersion,
		},
		Context: kyverno.GenerateRequestContext{
			UserRequestInfo: userRequestInfo,
		},
	}

	return gr
}

type generateStats struct {
	resp *response.EngineResponse
}

func (gs generateStats) PolicyName() string {
	return gs.resp.PolicyResponse.Policy
}

func (gs generateStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {
	if reflect.DeepEqual(response.EngineResponse{}, gs.resp) {
		return status
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range status.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range gs.resp.PolicyResponse.Rules {
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

func updateAverageTime(newTime time.Duration, oldAverageTimeString string, averageOver int64) time.Duration {
	if averageOver == 0 {
		return newTime
	}
	oldAverageExecutionTime, _ := time.ParseDuration(oldAverageTimeString)
	numerator := (oldAverageExecutionTime.Nanoseconds() * averageOver) + newTime.Nanoseconds()
	denominator := averageOver + 1
	newAverageTimeInNanoSeconds := numerator / denominator
	return time.Duration(newAverageTimeInNanoSeconds) * time.Nanosecond
}

type generateRequestResponse struct {
	gr  v1.GenerateRequestSpec
	err error
}

func (resp generateRequestResponse) info() string {
	return strings.Join([]string{resp.gr.Resource.Kind, resp.gr.Resource.Namespace, resp.gr.Resource.Name}, "/")
}

func (resp generateRequestResponse) error() string {
	return resp.err.Error()
}

func failedEvents(err error, gr kyverno.GenerateRequestSpec, resource unstructured.Unstructured) []event.Info {
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyFailed.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s failed to apply: %v", gr.Policy, err)

	return []event.Info{re}
}
