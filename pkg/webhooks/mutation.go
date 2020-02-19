package webhooks

import (
	"time"

	"github.com/nirmata/kyverno/pkg/policy"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleMutation handles mutating webhook admission request
// return value: generated patches
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest, resource unstructured.Unstructured, policies []kyverno.ClusterPolicy, roles, clusterRoles []string) []byte {
	glog.V(4).Infof("Receive request in mutating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	var patches [][]byte
	var engineResponses []response.EngineResponse

	userRequestInfo := kyverno.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}

	// build context
	ctx := context.NewContext()
	var err error
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
		NewResource:   resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	for _, policy := range policies {
		glog.V(2).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)
		policyContext.Policy = policy
		engineResponse := engine.Mutate(policyContext)
		engineResponses = append(engineResponses, engineResponse)
		updateStatusWithMutate(ws.status, policy, engineResponse)
		if !engineResponse.IsSuccesful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, resource.GetNamespace(), resource.GetName())
			continue
		}
		// gather patches
		patches = append(patches, engineResponse.GetPatches()...)
		glog.V(4).Infof("Mutation from policy %s has applied successfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())

		policyContext.NewResource = engineResponse.PatchedResource
	}

	// generate annotations
	if annPatches := generateAnnotationPatches(engineResponses); annPatches != nil {
		patches = append(patches, annPatches)
	}

	// report time
	reportTime := time.Now()

	// AUDIT
	// generate violation when response fails
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses)
	ws.pvGenerator.Add(pvInfos...)

	// ADD EVENTS
	events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
	ws.eventGen.Add(events...)

	// debug info
	func() {
		if len(patches) != 0 {
			glog.V(4).Infof("Patches generated for %s/%s/%s, operation=%v:\n %v",
				resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.Operation, string(engineutils.JoinPatches(patches)))
		}

		// if any of the policies fails, print out the error
		if !isResponseSuccesful(engineResponses) {
			glog.Errorf("Failed to mutate the resource, report as violation: %s\n", getErrorMsg(engineResponses))
		}
	}()

	// report time end
	glog.V(4).Infof("report: %v %s/%s/%s", time.Since(reportTime), resource.GetKind(), resource.GetNamespace(), resource.GetName())

	// patches holds all the successful patches, if no patch is created, it returns nil
	return engineutils.JoinPatches(patches)
}

func updateStatusWithMutate(statusSync *policy.StatusSync, policy kyverno.ClusterPolicy, response response.EngineResponse) {
	stats := kyverno.PolicyStatus{
		ViolationCount:           0,
		RulesAppliedCount:        response.PolicyResponse.RulesAppliedCount,
		ResourcesBlockedCount:    0,
		AvgExecutionTimeMutation: response.PolicyResponse.ProcessingTime.String(),
		Rules:                    nil,
	}

	for _, rule := range response.PolicyResponse.Rules {
		ruleStats := kyverno.RuleStats{
			Name:           rule.Name,
			ExecutionTime:  rule.ProcessingTime.String(),
			AppliedCount:   0,
			ViolationCount: 0,
			MutationCount:  0,
		}

		if rule.Success {
			ruleStats.AppliedCount++
			ruleStats.MutationCount++
		}
	}

}
