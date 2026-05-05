package generation

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetAppliedRules(t *testing.T) {
	policy := &kyvernov1.ClusterPolicy{
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "rule1",
					Generation: &kyvernov1.Generation{
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								Kind: "ConfigMap",
							},
						},
					},
				},
				{
					Name:     "rule2",
					Mutation: &kyvernov1.Mutation{},
				},
				{
					Name: "rule3",
					Generation: &kyvernov1.Generation{
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								Kind: "Secret",
							},
						},
					},
				},
			},
		},
	}

	appliedRules := []engineapi.RuleResponse{
		*engineapi.RulePass("rule1", engineapi.Generation, "", nil),
		*engineapi.RulePass("rule3", engineapi.Generation, "", nil),
	}

	result := getAppliedRules(policy, appliedRules)

	if len(result) != 2 {
		t.Errorf("expected 2 rules, got %d", len(result))
	}

	if result[0].Name != "rule1" {
		t.Errorf("expected rule1, got %s", result[0].Name)
	}

	if result[1].Name != "rule3" {
		t.Errorf("expected rule3, got %s", result[1].Name)
	}
}

func TestGetAppliedRulesNoGenerateRules(t *testing.T) {
	policy := &kyvernov1.ClusterPolicy{
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name:     "rule1",
					Mutation: &kyvernov1.Mutation{},
				},
			},
		},
	}

	appliedRules := []engineapi.RuleResponse{
		*engineapi.RulePass("rule1", engineapi.Mutation, "", nil),
	}

	result := getAppliedRules(policy, appliedRules)

	if len(result) != 0 {
		t.Errorf("expected 0 rules, got %d", len(result))
	}
}

func TestNewGenerationHandler(t *testing.T) {
	log := logr.Discard()
	kyvernoClient := fake.NewSimpleClientset()

	handler := NewGenerationHandler(
		log,
		nil,
		nil,
		kyvernoClient,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
		"system:serviceaccount:kyverno:kyverno-reports-controller",
	)

	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestHandleWithNoPolicies(t *testing.T) {
	log := logr.Discard()
	kyvernoClient := fake.NewSimpleClientset()

	handler := NewGenerationHandler(
		log,
		nil,
		nil,
		kyvernoClient,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
		"system:serviceaccount:kyverno:kyverno-reports-controller",
	)

	request := admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Kind: "Pod",
		},
	}

	policyContext := &engine.PolicyContext{}
	ctx := context.Background()

	handler.Handle(ctx, request, nil, policyContext)
}

func TestApplyGenerationWithNoRules(t *testing.T) {
	log := logr.Discard()
	kyvernoClient := fake.NewSimpleClientset()

	urGenerator := &mockURGenerator{}

	handler := NewGenerationHandler(
		log,
		nil,
		nil,
		kyvernoClient,
		nil,
		nil,
		nil,
		nil,
		urGenerator,
		nil,
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
		"system:serviceaccount:kyverno:kyverno-reports-controller",
	)

	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	request := admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Operation: admissionv1.Create,
	}

	policyContext := &engine.PolicyContext{}

	ctx := context.Background()
	h := handler.(*generationHandler)

	h.applyGeneration(ctx, request, policy, []engineapi.RuleResponse{}, policyContext)

	if urGenerator.applyCalled {
		t.Error("expected urGenerator.Apply NOT to be called with no rules")
	}
}

func TestSyncTriggerActionWithNoRules(t *testing.T) {
	log := logr.Discard()
	kyvernoClient := fake.NewSimpleClientset()

	urGenerator := &mockURGenerator{}

	handler := NewGenerationHandler(
		log,
		nil,
		nil,
		kyvernoClient,
		nil,
		nil,
		nil,
		nil,
		urGenerator,
		nil,
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
		"system:serviceaccount:kyverno:kyverno-reports-controller",
	)

	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	request := admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Operation: admissionv1.Delete,
	}

	policyContext := &engine.PolicyContext{}

	ctx := context.Background()
	h := handler.(*generationHandler)

	h.syncTriggerAction(ctx, request, policy, []engineapi.RuleResponse{}, policyContext)

	if urGenerator.applyCalled {
		t.Error("expected urGenerator.Apply NOT to be called with no failed rules")
	}
}

func TestSyncTriggerActionWithSynchronizeRule(t *testing.T) {
	log := logr.Discard()
	kyvernoClient := fake.NewSimpleClientset()

	urGenerator := &mockURGenerator{}

	handler := NewGenerationHandler(
		log,
		nil,
		nil,
		kyvernoClient,
		nil,
		nil,
		nil,
		nil,
		urGenerator,
		nil,
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
		"system:serviceaccount:kyverno:kyverno-reports-controller",
	)

	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "sync-rule",
					Generation: &kyvernov1.Generation{
						Synchronize: true,
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								Kind: "ConfigMap",
							},
						},
					},
				},
			},
		},
	}

	failedRules := []engineapi.RuleResponse{
		*engineapi.RuleFail("sync-rule", engineapi.Generation, "", nil),
	}

	request := admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Operation: admissionv1.Update,
	}

	policyContext := &engine.PolicyContext{}

	ctx := context.Background()
	h := handler.(*generationHandler)

	h.syncTriggerAction(ctx, request, policy, failedRules, policyContext)

	if !urGenerator.applyCalled {
		t.Error("expected urGenerator.Apply to be called for synchronize")
	}

	if len(urGenerator.applySpec.RuleContext) == 0 {
		t.Error("expected RuleContext to be populated")
	}

	if len(urGenerator.applySpec.RuleContext) > 0 && !urGenerator.applySpec.RuleContext[0].DeleteDownstream {
		t.Error("expected DeleteDownstream to be true for synchronize")
	}
}

type mockURGenerator struct {
	applyCalled bool
	applySpec   kyvernov2.UpdateRequestSpec
	applyErr    error
}

func (m *mockURGenerator) Apply(ctx context.Context, ur kyvernov2.UpdateRequestSpec) error {
	m.applyCalled = true
	m.applySpec = ur
	return m.applyErr
}
