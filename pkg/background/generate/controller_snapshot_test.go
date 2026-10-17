package generate

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newSnapshotController(t *testing.T) (*GenerateController, *generator, *provenanceRetryClient, *kyvernov2.UpdateRequest) {
	t.Helper()
	gen, base := newProvenanceStampGenerator(t)
	foreach := gen.rule.DeepCopy()
	foreach.Name = "generate-foreach"
	pattern := *gen.pattern.DeepCopy()
	pattern.Name = "{{element}}"
	foreach.Generation.GeneratePattern = kyvernov1.GeneratePattern{}
	foreach.Generation.ForEachGeneration = []kyvernov1.ForEachGeneration{{
		List: "`[\"first\",\"second\"]`", GeneratePattern: pattern,
	}}
	gen.policy.GetSpec().Rules = append(gen.policy.GetSpec().Rules, *foreach)
	client := &provenanceRetryClient{provenanceStampClient: base, objects: map[string]*unstructured.Unstructured{
		retryObjectKey(gen.trigger.GetNamespace(), gen.trigger.GetName()): gen.trigger.DeepCopy(),
	}}
	ur, requests := newGenerationRetryRequest(t, gen)
	configuration := config.NewDefaultConfiguration(false)
	controller := NewGenerateController(client, requests, common.NewStatusControl(requests, nil), cleanupRetryEngine{},
		&fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)}, nil, nil, nil, configuration, event.NewFake(), logr.Discard(), jmespath.New(configuration))
	return controller, gen, client, ur
}

func snapshotKeyGets(client *kubernetesfake.Clientset) int {
	count := 0
	for _, action := range client.Actions() {
		if action.Matches("get", "secrets") && action.GetNamespace() == config.KyvernoNamespace() && action.(k8stesting.GetAction).GetName() == provenance.SecretName {
			count++
		}
	}
	return count
}

func disableSnapshotReports(t *testing.T) {
	t.Helper()
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })
}

// Reporting configuration is global, so these ProcessUR tests must not run in parallel.
func TestProcessURSharesKeySnapshotAcrossRuleContexts(t *testing.T) {
	disableSnapshotReports(t)
	for _, replace := range []bool{false, true} {
		name := "key deleted"
		if replace {
			name = "key replaced"
		}
		t.Run(name, func(t *testing.T) {
			controller, gen, client, ur := newSnapshotController(t)
			kube := client.GetKubeClient().(*kubernetesfake.Clientset)
			secrets := kube.CoreV1().Secrets(config.KyvernoNamespace())
			originalKey, err := secrets.Get(t.Context(), provenance.SecretName, metav1.GetOptions{})
			require.NoError(t, err)
			originalSnapshot, err := controller.provenance.Snapshot(t.Context())
			require.NoError(t, err)
			client.beforeReturn = func(resource *unstructured.Unstructured) {
				require.Equal(t, "target", resource.GetName())
				client.beforeReturn = nil
				require.NoError(t, secrets.Delete(t.Context(), provenance.SecretName, metav1.DeleteOptions{}))
				if replace {
					replacement := originalKey.DeepCopy()
					replacement.ResourceVersion = ""
					replacement.Data["key"][0] ^= 0xff
					_, err := secrets.Create(t.Context(), replacement, metav1.CreateOptions{})
					require.NoError(t, err)
				}
			}
			kube.ClearActions()

			// Change the Secret during the first write, before signing that target
			// and before processing the second rule's two foreach elements.
			require.NoError(t, controller.ProcessUR(ur.DeepCopy()))
			requests := controller.kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace)
			completed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, kyvernov2.Completed, completed.Status.State, completed.Status.Message)
			require.Equal(t, 1, snapshotKeyGets(kube))
			require.Equal(t, 3, client.createCount)
			require.Len(t, client.patches, 3)
			for _, target := range []string{"target", "first", "second"} {
				obj := client.objects[retryObjectKey("tenant", target)]
				require.NotNil(t, obj)
				valid, err := originalSnapshot.Verify(t.Context(), gen.policy, obj)
				require.NoError(t, err)
				require.True(t, valid, target)
			}

			// Reusing the controller must still read the current key on the next UR.
			completed.Status.State = kyvernov2.Pending
			pending, err := requests.UpdateStatus(t.Context(), completed, metav1.UpdateOptions{})
			require.NoError(t, err)
			kube.ClearActions()
			require.NoError(t, controller.ProcessUR(pending))
			latest, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, snapshotKeyGets(kube))
			require.Equal(t, 3, client.createCount)
			if !replace {
				require.Equal(t, kyvernov2.Failed, latest.Status.State)
				require.Contains(t, latest.Status.Message, "sign generated resource provenance")
				require.Zero(t, client.updateCount)
				require.Len(t, client.patches, 3, "a missing key must fail before changing any target")
				return
			}
			require.Equal(t, kyvernov2.Completed, latest.Status.State, latest.Status.Message)
			currentSnapshot, err := controller.provenance.Snapshot(t.Context())
			require.NoError(t, err)
			for _, target := range []string{"target", "first", "second"} {
				obj := client.objects[retryObjectKey("tenant", target)]
				valid, err := currentSnapshot.Verify(t.Context(), gen.policy, obj)
				require.NoError(t, err)
				require.True(t, valid, target)
				valid, err = originalSnapshot.Verify(t.Context(), gen.policy, obj)
				require.NoError(t, err)
				require.False(t, valid, "the next reconciliation must use the replacement key")
			}
		})
	}
}

func TestProcessURSharesMissingKeyFailureAcrossRuleContexts(t *testing.T) {
	disableSnapshotReports(t)
	controller, _, client, ur := newSnapshotController(t)
	kube := client.GetKubeClient().(*kubernetesfake.Clientset)
	secrets := kube.CoreV1().Secrets(config.KyvernoNamespace())
	require.NoError(t, secrets.Delete(t.Context(), provenance.SecretName, metav1.DeleteOptions{}))
	kube.ClearActions()

	require.NoError(t, controller.ProcessUR(ur.DeepCopy()))
	requests := controller.kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace)
	failed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Failed, failed.Status.State)
	require.Contains(t, failed.Status.Message, "rule generate failed")
	require.Contains(t, failed.Status.Message, "rule generate-foreach failed")
	require.Equal(t, 1, snapshotKeyGets(kube))
	require.Zero(t, client.createCount)
	require.Zero(t, client.updateCount)
	require.Zero(t, client.applyCount)
	require.Empty(t, client.patches)
	require.Len(t, client.objects, 1, "only the original trigger must remain")

	// The failure belongs to one ProcessUR invocation, not the controller's lifetime.
	require.NoError(t, provenance.EnsureKey(t.Context(), secrets))
	failed.Status.State = kyvernov2.Pending
	pending, err := requests.UpdateStatus(t.Context(), failed, metav1.UpdateOptions{})
	require.NoError(t, err)
	kube.ClearActions()
	require.NoError(t, controller.ProcessUR(pending))
	completed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Completed, completed.Status.State, completed.Status.Message)
	require.Equal(t, 1, snapshotKeyGets(kube))
	require.Equal(t, 3, client.createCount)
	require.Len(t, client.patches, 3)
}

func TestApplyGeneratePolicyRefreshesKeySnapshotForEachCall(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	kube := client.GetKubeClient().(*kubernetesfake.Clientset)
	controller := NewGenerateController(client, nil, nil, provenanceStampEngine{}, nil, nil, nil, nil, nil, nil, logr.Discard(), nil)
	client.beforeReturn = func(*unstructured.Unstructured) {
		require.NoError(t, kube.CoreV1().Secrets(config.KyvernoNamespace()).Delete(t.Context(), provenance.SecretName, metav1.DeleteOptions{}))
	}
	kube.ClearActions()
	_, err := controller.ApplyGeneratePolicy(logr.Discard(), gen.policyContext.(*engine.PolicyContext), []string{gen.rule.Name})
	require.NoError(t, err)
	require.Equal(t, 1, snapshotKeyGets(kube))
	require.Len(t, client.patches, 1)

	kube.ClearActions()
	_, err = controller.ApplyGeneratePolicy(logr.Discard(), gen.policyContext.(*engine.PolicyContext), []string{gen.rule.Name})
	require.ErrorIs(t, err, errGeneratedProvenance)
	require.Equal(t, 1, snapshotKeyGets(kube))
	require.Equal(t, 1, client.createCount)
	require.Zero(t, client.updateCount)
	require.Len(t, client.patches, 1)
}
