package generate

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

type receiptRetryEngine struct {
	cleanupRetryEngine
	ineligibleRule string
	status         engineapi.RuleStatus
}

func (e receiptRetryEngine) Generate(ctx context.Context, policyContext engineapi.PolicyContext) engineapi.EngineResponse {
	name := policyContext.Policy().GetSpec().Rules[0].Name
	if name != e.ineligibleRule {
		return e.cleanupRetryEngine.Generate(ctx, policyContext)
	}
	var response *engineapi.RuleResponse
	switch e.status {
	case engineapi.RuleStatusSkip:
		response = engineapi.RuleSkip(name, engineapi.Generation, "preconditions no longer match", nil)
	case engineapi.RuleStatusFail:
		response = engineapi.RuleFail(name, engineapi.Generation, "resource no longer matches", nil)
	case engineapi.RuleStatusError:
		response = engineapi.RuleError(name, engineapi.Generation, "condition evaluation failed", fmt.Errorf("condition error"), nil)
	default:
		return e.cleanupRetryEngine.Generate(ctx, policyContext)
	}
	return engineapi.NewEngineResponseFromPolicyContext(policyContext).WithPolicyResponse(engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{*response},
	})
}

// Reporting configuration is global; these controller/status lifecycle tests
// deliberately do not run in parallel.
func TestProcessURKeepsReceiptsForIneligibleRules(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })

	for _, mode := range []string{"skip", "fail", "error", "removed", "renamed", "non-generate", "mixed rules", "malformed receipt", "receipt read failure"} {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pending=%t", mode, pending), func(t *testing.T) {
				gen, base := newProvenanceStampGenerator(t)
				gen.rule.Generation.Synchronize = false
				original := *gen.rule.DeepCopy()
				gen.policy.GetSpec().Rules = []kyvernov1.Rule{original}
				client := &provenanceRetryClient{provenanceStampClient: base, objects: map[string]*unstructured.Unstructured{
					retryObjectKey(gen.trigger.GetNamespace(), gen.trigger.GetName()): gen.trigger.DeepCopy(),
				}}
				gen.client = client
				if mode == "mixed rules" {
					other := *original.DeepCopy()
					other.Name, other.Generation.Name = "other", "other-target"
					gen.policy.GetSpec().Rules = append(gen.policy.GetSpec().Rules, other)
				}
				ur, kyvernoClient := newGenerationRetryRequest(t, gen)
				requests := kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace)
				if pending {
					client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
					_, err := gen.generate()
					require.ErrorContains(t, err, "temporary annotation failure")
					client.patchErr = nil
					client.objects["tenant/target"].Object["data"] = map[string]any{"key": "ordinary-user-edit"}
				}
				receipts := retryReceipts(t, kyvernoClient, ur)
				if pending {
					require.Len(t, receipts, 1)
				}
				engine := receiptRetryEngine{ineligibleRule: original.Name, status: engineapi.RuleStatusSkip}
				switch mode {
				case "fail":
					engine.status = engineapi.RuleStatusFail
				case "error":
					engine.status = engineapi.RuleStatusError
				case "removed":
					gen.policy.GetSpec().Rules = nil
				case "renamed":
					gen.policy.GetSpec().Rules[0].Name = "renamed"
				case "non-generate":
					gen.policy.GetSpec().Rules[0].Generation = nil
					engine.ineligibleRule = ""
				case "malformed receipt":
					if pending {
						latest, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
						require.NoError(t, err)
						latest.Annotations[pendingProvenanceAnnotation] = "invalid-json"
						_, err = requests.Update(t.Context(), latest, metav1.UpdateOptions{})
						require.NoError(t, err)
					}
				}
				readFailure := pending && mode == "receipt read failure"
				kyvernoClient.PrependReactor("get", "updaterequests", func(k8stesting.Action) (bool, runtime.Object, error) {
					if readFailure {
						readFailure = false // Status control can still fetch and record the failure.
						return true, nil, apierrors.NewTimeoutError("receipt read failed", 1)
					}
					return false, nil, nil
				})
				configuration := config.NewDefaultConfiguration(false)
				process := func(request *kyvernov2.UpdateRequest) {
					controller := NewGenerateController(client, kyvernoClient, common.NewStatusControl(kyvernoClient, nil), engine,
						&fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)}, nil, nil, nil, configuration, event.NewFake(), logr.Discard(), jmespath.New(configuration))
					require.NoError(t, controller.ProcessUR(request))
				}
				patchesBefore := len(client.patches)
				// Use the stale informer object without the receipt annotation.
				require.Empty(t, ur.Annotations[pendingProvenanceAnnotation])
				process(ur.DeepCopy())
				latest, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if !pending {
					require.Equal(t, kyvernov2.Completed, latest.Status.State, latest.Status.Message)
					return
				}
				require.Equal(t, kyvernov2.Failed, latest.Status.State, "ineligible rules must not discard pending receipts")
				switch mode {
				case "malformed receipt":
					require.Contains(t, latest.Status.Message, "decode generated resource receipts")
					require.Equal(t, "invalid-json", latest.Annotations[pendingProvenanceAnnotation])
					return
				case "receipt read failure":
					require.Contains(t, latest.Status.Message, "receipt read failed")
				default:
					require.Contains(t, latest.Status.Message, "pending receipt")
				}
				require.Equal(t, receipts, retryReceipts(t, kyvernoClient, ur))
				if mode == "mixed rules" {
					require.NotNil(t, client.objects["tenant/other-target"], "later rules must still make progress")
					require.NotEmpty(t, client.objects["tenant/other-target"].GetAnnotations()[provenance.Annotation])
					patchesBefore++
				}
				require.Len(t, client.patches, patchesBefore)
				require.Empty(t, client.objects["tenant/target"].GetAnnotations()[provenance.Annotation])
				require.Equal(t, "ordinary-user-edit", client.objects["tenant/target"].Object["data"].(map[string]any)["key"])
				gen.policy.GetSpec().Rules = []kyvernov1.Rule{original}
				engine.ineligibleRule = ""
				latest.Status.State = kyvernov2.Pending
				latest, err = requests.UpdateStatus(t.Context(), latest, metav1.UpdateOptions{})
				require.NoError(t, err)
				process(latest)
				latest, err = requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, kyvernov2.Completed, latest.Status.State, latest.Status.Message)
				require.Empty(t, retryReceipts(t, kyvernoClient, ur))
				valid, err := gen.provenance.Verify(t.Context(), gen.policy, client.objects["tenant/target"])
				require.NoError(t, err)
				require.True(t, valid)
				require.Equal(t, "ordinary-user-edit", client.objects["tenant/target"].Object["data"].(map[string]any)["key"])
			})
		}
	}
}

func TestProcessURPendingReceiptsPreserveCleanup(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })
	for _, mode := range []string{"cleanup", "deleted policy", "replaced policy", "mixed cleanup and generation"} {
		t.Run(mode, func(t *testing.T) {
			gen, base := newProvenanceStampGenerator(t)
			client := &provenanceRetryClient{provenanceStampClient: base, objects: map[string]*unstructured.Unstructured{
				retryObjectKey(gen.trigger.GetNamespace(), gen.trigger.GetName()): gen.trigger.DeepCopy(),
			}}
			gen.client = client
			ur, kyvernoClient := newGenerationRetryRequest(t, gen)
			client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
			_, err := gen.generate()
			require.Error(t, err)
			client.patchErr = nil
			require.Len(t, retryReceipts(t, kyvernoClient, ur), 1)
			lister := &fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)}
			switch mode {
			case "cleanup", "mixed cleanup and generation":
				ur.Spec.RuleContext[0].DeleteDownstream = true
				if mode == "mixed cleanup and generation" {
					other := *gen.rule.DeepCopy()
					other.Name, other.Generation.Name = "other", "other-target"
					gen.policy.GetSpec().Rules = append(gen.policy.GetSpec().Rules, other)
					ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{Rule: other.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)})
				}
			case "deleted policy":
				lister.policy, lister.err = nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusterpolicies"}, gen.policy.GetName())
			case "replaced policy":
				gen.policy.SetUID("replacement-policy")
			}
			configuration := config.NewDefaultConfiguration(false)
			controller := NewGenerateController(client, kyvernoClient, common.NewStatusControl(kyvernoClient, nil), cleanupRetryEngine{},
				lister, nil, nil, nil, configuration, event.NewFake(), logr.Discard(), jmespath.New(configuration))
			require.NoError(t, controller.ProcessUR(ur))
			latest, err := kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, kyvernov2.Completed, latest.Status.State, latest.Status.Message)
			require.Empty(t, client.objects["tenant/target"].GetAnnotations()[provenance.Annotation])
			if mode == "mixed cleanup and generation" {
				require.NotEmpty(t, client.objects["tenant/other-target"].GetAnnotations()[provenance.Annotation])
			}
		})
	}
}
