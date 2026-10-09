package generate

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type cleanupRetryClient struct {
	*provenanceCleanupClient
	deleteAttempts int
}

func (c *cleanupRetryClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	obj, err := c.provenanceCleanupClient.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
	if apierrors.IsNotFound(err) {
		return c.Interface.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
	}
	return obj, err
}

func (c *cleanupRetryClient) DeleteResource(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
	c.deleteAttempts++
	if c.deleteAttempts == 1 {
		return apierrors.NewTimeoutError("temporary cleanup failure", 1)
	}
	return c.provenanceCleanupClient.DeleteResource(ctx, apiVersion, kind, namespace, name, dryRun, options)
}

type cleanupRetryEngine struct{ provenanceStampEngine }

func (cleanupRetryEngine) Generate(_ context.Context, ctx engineapi.PolicyContext) engineapi.EngineResponse {
	return engineapi.NewEngineResponseFromPolicyContext(ctx).WithPolicyResponse(engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{*engineapi.RulePass(ctx.Policy().GetSpec().Rules[0].Name, engineapi.Generation, "", nil)},
	})
}

// This test temporarily disables global reporting and must not run in parallel.
func TestProcessURRetriesCleanupAfterAnotherRuleGeneratedResources(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })

	gen, generated := newProvenanceStampGenerator(t)
	_, err := gen.generate()
	require.NoError(t, err)
	cleanupTarget := generated.target.DeepCopy()
	createRule := *gen.rule.DeepCopy()
	createRule.Name = "generate-other"
	createRule.Generation.Name = "other-target"
	gen.policy.GetSpec().Rules = append(gen.policy.GetSpec().Rules, createRule)
	client := &cleanupRetryClient{provenanceCleanupClient: &provenanceCleanupClient{
		Interface: generated,
		items:     []unstructured.Unstructured{*gen.trigger.DeepCopy(), *cleanupTarget},
	}}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mixed-rule-cleanup-retry", Namespace: config.KyvernoNamespace(),
			Annotations: map[string]string{provenance.PolicyUIDAnnotation: string(gen.policy.GetUID())},
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.Generate, Policy: gen.policy.GetName(),
			RuleContext: []kyvernov2.RuleContext{
				{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger), DeleteDownstream: true},
				{Rule: createRule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)},
			},
		},
	}
	kyvernoClient := kyvernofake.NewSimpleClientset(ur)
	requests := kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace)
	configuration := config.NewDefaultConfiguration(false)
	controller := &GenerateController{
		client: client, provenance: gen.provenance,
		statusControl: common.NewStatusControl(kyvernoClient, nil),
		policyLister:  &fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)},
		engine:        cleanupRetryEngine{}, configuration: configuration,
		jp: jmespath.New(configuration), eventGen: event.NewFake(), log: logr.Discard(),
	}

	// The first rule fails cleanup, then the second rule succeeds at generation.
	// Real status persistence retains that generated resource on the failed UR.
	require.NoError(t, controller.ProcessUR(ur.DeepCopy()))
	failed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Failed, failed.Status.State)
	require.Contains(t, failed.Status.Message, "temporary cleanup failure")
	require.Len(t, failed.Status.GeneratedResources, 1)
	require.Equal(t, "other-target", failed.Status.GeneratedResources[0].Name)
	require.Empty(t, failed.GetAnnotations()[provenance.CleanupPolicyUIDAnnotation])
	require.Equal(t, 1, client.deleteAttempts)
	require.Empty(t, client.deleted)

	// A retry must still authenticate and delete the original cleanup target,
	// rather than treating the other rule's result as a deleted-policy record.
	failed.Status.State = kyvernov2.Pending
	retry, err := requests.UpdateStatus(t.Context(), failed, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, controller.ProcessUR(retry))
	completed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Completed, completed.Status.State)
	require.Equal(t, 2, client.deleteAttempts)
	require.Equal(t, []string{cleanupTarget.GetName()}, client.deleted)
	require.Equal(t, "other-target", generated.target.GetName())
}
