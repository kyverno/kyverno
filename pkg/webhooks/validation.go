package webhooks

import (
	"reflect"
	"sort"
	"time"

	"github.com/nirmata/kyverno/pkg/policyStatus"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	glog.V(4).Infof("Receive request in validating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	evalTime := time.Now()

	// Get new and old resource
	newR, oldR, err := extractResources(patchedResource, request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		glog.Error(err)
		return true, ""
	}
	userRequestInfo := kyverno.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}
	// build context
	ctx := context.NewContext()
	// load incoming resource into the context
	err = ctx.AddResource(request.Object.Raw)
	if err != nil {
		glog.Infof("Failed to load resource in context:%v", err)
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		glog.Infof("Failed to load userInfo in context:%v", err)
	}

	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		glog.Infof("Failed to load service account in context:%v", err)
	}

	policyContext := engine.PolicyContext{
		NewResource:   newR,
		OldResource:   oldR,
		Context:       ctx,
		AdmissionInfo: userRequestInfo,
	}
	var engineResponses []response.EngineResponse
	for _, policy := range policies {
		glog.V(2).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			newR.GetKind(), newR.GetNamespace(), newR.GetName(), request.UID, request.Operation)
		policyContext.Policy = policy
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}
		engineResponses = append(engineResponses, engineResponse)
		go func() {
			ws.status.Listener <- updateStatusWithValidateStats(engineResponse)
		}()
		if !engineResponse.IsSuccesful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, newR.GetNamespace(), newR.GetName())
			continue
		}
	}
	glog.V(4).Infof("eval: %v %s/%s/%s ", time.Since(evalTime), request.Kind, request.Namespace, request.Name)
	// report time
	reportTime := time.Now()

	// If Validation fails then reject the request
	// no violations will be created on "enforce"
	// the event will be reported on owner by k8s
	blocked := toBlockResource(engineResponses)
	if blocked {
		glog.V(4).Infof("resource %s/%s/%s is blocked\n", newR.GetKind(), newR.GetNamespace(), newR.GetName())
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	// ADD POLICY VIOLATIONS
	// violations are created with resource on "audit"
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses)
	ws.pvGenerator.Add(pvInfos...)
	// ADD EVENTS
	events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
	ws.eventGen.Add(events...)
	// report time end
	glog.V(4).Infof("report: %v %s/%s/%s", time.Since(reportTime), request.Kind, request.Namespace, request.Name)
	return true, ""
}

type validateStats struct {
	resp response.EngineResponse
}

func updateStatusWithValidateStats(resp response.EngineResponse) *validateStats {
	return &validateStats{
		resp: resp,
	}
}

func (vs *validateStats) UpdateStatus(s *policyStatus.Sync) {
	if reflect.DeepEqual(response.EngineResponse{}, vs.resp) {
		return
	}

	s.Cache.Mutex.Lock()
	status, exist := s.Cache.Data[vs.resp.PolicyResponse.Policy]
	if !exist {
		if s.PolicyStore != nil {
			policy, _ := s.PolicyStore.Get(vs.resp.PolicyResponse.Policy)
			if policy != nil {
				status = policy.Status
			}
		}
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

	s.Cache.Data[vs.resp.PolicyResponse.Policy] = status
	s.Cache.Mutex.Unlock()
}
