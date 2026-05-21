package policy

import (
	"testing"
    
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeClusterPolicyForUR(name string, rules []kyvernov1.Rule) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy {
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

			if result.Spec.Type != kyvernov2.Mutate {
				t.Errorf("wrong type. want: mutate, got: %v", result.Spec.Type)
			}
			if result.Spec.Rule != tt.ruleName {
				t.Errorf("wrong rule name. want: %q, got: %q", tt.ruleName, result.Spec.Rule)
			}
			if result.Spec.Policy != policyKey(tt.policy) {
				t.Errorf("Spec.Policy: %q, want: %q", result.Spec.Policy, policyKey(tt.policy))
			}

			// labels
			wantLabels := common.MutateLabelsSet(policyKey(tt.policy), tt.trigger)
			for k, wantVal := range wantLabels {
				if gotVal, ok := result.Labels[k]; !ok {
					t.Errorf("Labels missing key %q", k)
				} else if gotVal != wantVal {
					t.Errorf("Labels[%q]: %q, want: %q", k, gotVal, wantVal)
				}
			}

			// every field from trigger
			res := result.Spec.Resource
			if res.Kind != tt.trigger.GetKind() {
				t.Errorf("Spec.Resource.Kind: %q, want: %q", res.Kind, tt.trigger.GetKind())
			}
			if res.Namespace != tt.trigger.GetNamespace() {
				t.Errorf("Spec.Resource.Namespace: %q, want: %q", res.Namespace, tt.trigger.GetNamespace())
			}
			if res.Name != tt.trigger.GetName() {
				t.Errorf("Spec.Resource.Name: %q, want: %q", res.Name, tt.trigger.GetName())
			}
			if res.APIVersion != tt.trigger.GetAPIVersion() {
				t.Errorf("Spec.Resource.APIVersion: %q, want: %q", res.APIVersion, tt.trigger.GetAPIVersion())
			}
			if res.UID != tt.trigger.GetUID() {
				t.Errorf("Spec.Resource.UID: %q, want: %q", res.UID, tt.trigger.GetUID())
			}
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
			name: "cluster policy: Spec.Policy is the bare policy name",
			policy: engineapi.NewKyvernoPolicy(makeClusterPolicyForUR("my-cluster-policy", nil)),
			wantPolicy: "my-cluster-policy", 
			wantType: kyvernov2.Generate,
		},
		{
			name:       "generating policy sets Type to CELGenerate",
			policy:     engineapi.NewGeneratingPolicy(&policiesv1beta1.GeneratingPolicy{
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

			if result.Spec.Type != tt.wantType {
				t.Errorf("wrong type. want: %v, got: %v", tt.wantType, result.Spec.Type)
			}

			if result.Spec.Policy != tt.wantPolicy {
				t.Errorf("Spec.Policy: %v, want: %v", result.Spec.Policy, tt.wantPolicy)
			}

			// labels
			wantLabels := common.GenerateLabelsSet(tt.wantPolicy)
			for k, wantVal := range wantLabels {
				if gotVal, ok := result.Labels[k]; !ok {
					t.Errorf("Labels missing key %q", k)
				} else if gotVal != wantVal {
					t.Errorf("Labels[%q]: %q, want: %q", k, gotVal, wantVal)
				}
			}
		})
	}
}

func Test_newUrMeta(t *testing.T) {
	result := newUrMeta()

	wantKind := "UpdateRequest"
	if result.Kind != wantKind {
		t.Errorf("newUrMeta() kind: %q, want: %q", result.Kind, wantKind)
	}

	wantGenerateName := "ur-"
	if result.GenerateName != wantGenerateName {
		t.Errorf("newUrMeta() GenerateName = %q, want %q", result.GenerateName, wantGenerateName)
	}

	wantNamespace := config.KyvernoNamespace()
	if result.Namespace != wantNamespace {
		t.Errorf("newUrMeta() Namespace = %q, want %q", result.Namespace, wantNamespace)
	}

	wantAPIVersion := kyvernov2.SchemeGroupVersion.String()
	if result.APIVersion != wantAPIVersion {
		t.Errorf("newUrMeta() APIVersion = %q, want %q", result.APIVersion, wantAPIVersion)
	}
}
