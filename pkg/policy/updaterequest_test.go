package policy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func makeUR(n int) *kyvernov2.UpdateRequest {
	ur := &kyvernov2.UpdateRequest{}
	for i := 0; i < n; i++ {
		ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{
			Rule: "test-rule",
			Trigger: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       "ns",
			},
		})
	}
	return ur
}

func TestSplitUR(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		batchSize     int
		wantBatches   int
		wantLastBatch int
	}{
		{"empty", 0, 100, 1, 0},
		{"below batch", 50, 100, 1, 50},
		{"exact batch", 100, 100, 1, 100},
		{"one over", 101, 100, 2, 1},
		{"10k namespaces", 10000, 100, 100, 100},
		{"uneven split", 250, 100, 3, 50},
		{"zero batchSize clamped to 1", 3, 0, 3, 1},
		{"negative batchSize clamped to 1", 3, -5, 3, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ur := makeUR(tc.total)
			batches := splitUR(ur, tc.batchSize)
			assert.Len(t, batches, tc.wantBatches)
			assert.Len(t, batches[len(batches)-1].Spec.RuleContext, tc.wantLastBatch)

			// total entries across all batches must equal original
			effectiveBatch := tc.batchSize
			if effectiveBatch <= 0 {
				effectiveBatch = 1
			}
			total := 0
			for _, b := range batches {
				assert.LessOrEqual(t, len(b.Spec.RuleContext), effectiveBatch)
				total += len(b.Spec.RuleContext)
			}
			assert.Equal(t, tc.total, total)
		})
	}
}

func TestSplitURNoAlias(t *testing.T) {
	ur := makeUR(2)
	ur.ObjectMeta = metav1.ObjectMeta{
		Labels: map[string]string{"a": "b"},
	}
	ur.Spec.Context.UserRequestInfo.Roles = []string{"role"}
	ur.Status.GeneratedResources = []kyvernov1.ResourceSpec{
		{Name: "original"},
	}

	batches := splitUR(ur, 1)
	assert.Len(t, batches, 2)

	batches[0].ObjectMeta.Labels["a"] = "changed"
	batches[0].Spec.Context.UserRequestInfo.Roles[0] = "changed"
	batches[0].Status.GeneratedResources[0].Name = "changed"

	assert.Equal(t, "b", ur.ObjectMeta.Labels["a"])
	assert.Equal(t, "role", ur.Spec.Context.UserRequestInfo.Roles[0])
	assert.Equal(t, "original", ur.Status.GeneratedResources[0].Name)
}

func TestNewGenerateURNGpolPolicyKey(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "team-a",
		},
	}

	ur := newGenerateUR(engineapi.NewNamespacedGeneratingPolicy(ngpol))

	assert.Equal(t, kyvernov2.CELGenerate, ur.Spec.Type)
	assert.Equal(t, "team-a/sample", ur.Spec.Policy)
	assert.Equal(t, map[string]string(common.GenerateLabelsSet("team-a/sample")), ur.Labels)
}

func TestFilterTriggersByNamespace(t *testing.T) {
	mk := func(kind, ns, name string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetKind(kind)
		u.SetNamespace(ns)
		u.SetName(name)
		return u
	}

	triggers := []*unstructured.Unstructured{
		mk("Deployment", "team-a", "dep-a"),
		mk("Deployment", "team-b", "dep-b"),
		mk("Namespace", "", "team-a"),
		mk("Namespace", "", "team-b"),
	}

	filtered := filterTriggersByNamespace(triggers, "team-a")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "dep-a", filtered[0].GetName())
	assert.Equal(t, "team-a", filtered[1].GetName())
}

func makeClusterPolicyForUR(name string, rules []kyvernov1.Rule) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kyvernov1.Spec{
			Rules: rules,
		},
	}
}

func Test_newMutateUR(t *testing.T) {
	tests := []struct {
		name     string
		policy   kyvernov1.PolicyInterface
		trigger  kyvernov1.ResourceSpec
		ruleName string
	}{
		{
			name:     "Successfully creates a mutate UpdateRequest",
			policy:   makeClusterPolicyForUR("test-policy", nil),
			ruleName: "check-pod-labels",
			trigger: kyvernov1.ResourceSpec{
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "test-pod",
				APIVersion: "v1",
				UID:        types.UID("abc-123"),
			},
		},
		{
			name:     "empty trigger fields enforcing policy without panicking",
			policy:   makeClusterPolicyForUR("test-policy", nil),
			ruleName: "check-empty-fields",
			trigger:  kyvernov1.ResourceSpec{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newMutateUR(tt.policy, tt.trigger, tt.ruleName)

			assert.Equal(t, kyvernov2.Mutate, result.Spec.Type)
			assert.Equal(t, tt.ruleName, result.Spec.Rule)
			assert.Equal(t, policyKey(tt.policy), result.Spec.Policy)

			// labels
			wantLabels := common.MutateLabelsSet(policyKey(tt.policy), tt.trigger)
			for k, wantLabel := range wantLabels {
				if gotVal, ok := result.Labels[k]; !ok {
					t.Errorf("Labels missing key %q", k)
				} else if gotVal != wantLabel {
					t.Errorf("Labels[%q]: %q, want: %q", k, gotVal, wantLabel)
				}
			}
			if gotVal, ok := result.Labels[kyvernov2.URMutatePolicyLabel]; !ok {
				t.Errorf("Labels missing key %q", kyvernov2.URMutatePolicyLabel)
			} else if gotVal != tt.policy.GetName() {
				t.Errorf("Labels[%q]: %q, want: %q", kyvernov2.URMutatePolicyLabel,
					gotVal, tt.policy.GetName())
			}
			if gotVal, ok := result.Labels[kyvernov2.URMutateTriggerKindLabel]; !ok {
				t.Errorf("Labels missing key %q", kyvernov2.URMutateTriggerKindLabel)
			} else if gotVal != tt.trigger.GetKind() {
				t.Errorf("Labels[%q]: %q, want: %q", kyvernov2.URMutateTriggerKindLabel,
					gotVal, tt.trigger.GetKind())
			}
			if gotVal, ok := result.Labels[kyvernov2.URMutateTriggerNSLabel]; !ok {
				t.Errorf("Labels missing key %q", kyvernov2.URMutateTriggerNSLabel)
			} else if gotVal != tt.trigger.GetNamespace() {
				t.Errorf("Labels[%q]: %q, want: %q", kyvernov2.URMutateTriggerNSLabel,
					gotVal, tt.trigger.GetNamespace())
			}
			if gotVal, ok := result.Labels[kyvernov2.URMutateTriggerNameLabel]; !ok {
				t.Errorf("Labels missing key %q", kyvernov2.URMutateTriggerNameLabel)
			} else if gotVal != tt.trigger.GetName() {
				t.Errorf("Labels[%q]: %q, want: %q", kyvernov2.URMutateTriggerNameLabel,
					gotVal, tt.trigger.GetName())
			}

			// every field from trigger
			res := result.Spec.Resource
			assert.Equal(t, tt.trigger.GetKind(), res.Kind)
			assert.Equal(t, tt.trigger.GetNamespace(), res.Namespace)
			assert.Equal(t, tt.trigger.GetName(), res.Name)
			assert.Equal(t, tt.trigger.GetAPIVersion(), res.APIVersion)
			assert.Equal(t, tt.trigger.GetUID(), res.UID)
		})
	}
}

func Test_newGenerateUR(t *testing.T) {
	tests := []struct {
		name       string
		policy     engineapi.GenericPolicy
		wantPolicy string
		wantType   kyvernov2.RequestType
	}{
		{
			name:       "cluster policy: Spec.Policy is the bare policy name",
			policy:     engineapi.NewKyvernoPolicy(makeClusterPolicyForUR("my-cluster-policy", nil)),
			wantPolicy: "my-cluster-policy",
			wantType:   kyvernov2.Generate,
		},
		{
			name: "generating policy sets Type to CELGenerate",
			policy: engineapi.NewGeneratingPolicy(&policiesv1beta1.GeneratingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-celpolicy",
				},
			}),
			wantPolicy: "test-celpolicy",
			wantType:   kyvernov2.CELGenerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newGenerateUR(tt.policy)
			assert.Equal(t, tt.wantType, result.Spec.Type)
			assert.Equal(t, tt.wantPolicy, result.Spec.Policy)

			// labels
			if gotVal, ok := result.Labels[kyvernov2.URGeneratePolicyLabel]; !ok {
				t.Errorf("Labels missing key %q", kyvernov2.URGeneratePolicyLabel)
			} else if gotVal != tt.wantPolicy {
				t.Errorf("Labels[%q]: %q, want: %q", kyvernov2.URGeneratePolicyLabel,
					gotVal, tt.wantPolicy)
			}
		})
	}
}

func Test_newUrMeta(t *testing.T) {
	result := newUrMeta()

	assert.Equal(t, "UpdateRequest", result.Kind)
	assert.Equal(t, "ur-", result.GenerateName)
	assert.Equal(t, config.KyvernoNamespace(), result.Namespace)
	assert.Equal(t, kyvernov2.SchemeGroupVersion.String(), result.APIVersion)
}
