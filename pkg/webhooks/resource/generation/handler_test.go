package generation

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
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

// TestHandleNonTrigger_CloneSourceDeletion_AttachesAdmissionContext verifies that
// deleting a clone source produces a delete-downstream UpdateRequest carrying the
// admission request in its context, which lets the background controller scope the
// cleanup to the deleted source (https://github.com/kyverno/kyverno/issues/9654).
// It drives the full non-trigger path (handleNonTrigger -> processRequest).
func TestHandleNonTrigger_CloneSourceDeletion_AttachesAdmissionContext(t *testing.T) {
	// downstream target cloned from the source, discoverable by its source-* labels
	target := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "dummy-secret-1",
				"namespace": "certs-replicated",
				"labels": map[string]interface{}{
					common.GeneratePolicyLabel:          "sync-secrets",
					common.GeneratePolicyNamespaceLabel: "",
					common.GenerateRuleLabel:            "sync-secret",
					kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
					common.GenerateSourceGroupLabel:     "",
					common.GenerateSourceVersionLabel:   "v1",
					common.GenerateSourceKindLabel:      "Secret",
					common.GenerateSourceNSLabel:        "certs",
					common.GenerateSourceNameLabel:      "dummy-secret-1",
					common.GenerateSourceUIDLabel:       "source-uid-1",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "secrets"}:    "SecretList",
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
	}
	client, err := dclient.NewFakeClient(scheme, gvrToListKind, target)
	assert.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	// cluster policy referenced by the target labels
	policy := &kyvernov1.ClusterPolicy{}
	policy.SetName("sync-secrets")
	policy.Spec = kyvernov1.Spec{
		Rules: []kyvernov1.Rule{
			{
				Name: "sync-secret",
				MatchResources: kyvernov1.MatchResources{
					Any: kyvernov1.ResourceFilters{
						{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Namespace"}}},
					},
				},
				Generation: &kyvernov1.Generation{
					Synchronize: true,
					GeneratePattern: kyvernov1.GeneratePattern{
						CloneList: kyvernov1.CloneList{
							Namespace: "certs",
							Kinds:     []string{"v1/Secret"},
						},
					},
				},
			},
		},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, indexer.Add(policy))
	cpolLister := kyvernov1listers.NewClusterPolicyLister(indexer)

	gen := &mockURGenerator{}
	h := &generationHandler{
		log:         logr.Discard(),
		client:      client,
		cpolLister:  cpolLister,
		urGenerator: gen,
	}

	// the deleted clone source
	source := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "dummy-secret-1",
				"namespace": "certs",
				"uid":       "source-uid-1",
				"labels": map[string]interface{}{
					common.GenerateTypeCloneSourceLabel: "",
				},
			},
		},
	}

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	policyContext, err := engine.NewPolicyContext(jp, source, kyvernov1.Delete, nil, cfg)
	assert.NoError(t, err)

	request := admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		Namespace: "certs",
		Name:      "dummy-secret-1",
	}

	// invoke via handleNonTrigger so the clone-source dispatch path is exercised
	// end to end (it recognizes the source by its clone-source label).
	h.handleNonTrigger(context.TODO(), request, policyContext)

	// a delete-downstream UR was created carrying the admission request so the
	// background controller can scope the cleanup to the deleted source.
	assert.True(t, gen.applyCalled, "an UpdateRequest should have been created")
	assert.Equal(t, "sync-secrets", gen.applySpec.Policy)
	assert.NotNil(t, gen.applySpec.Context.AdmissionRequestInfo.AdmissionRequest,
		"the admission request must be attached to the delete-downstream UR")
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
