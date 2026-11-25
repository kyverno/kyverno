package gpol

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func TestPlanGeneratingAdmissionSkipsUnrelatedSecrets(t *testing.T) {
	t.Parallel()
	policy := cloneSecretPolicy(true, `object.metadata.namespace == "src"`)

	source := secretObject("src", "source-secret")
	other := secretObject("other", "unrelated")

	tests := []struct {
		name            string
		operation       admissionv1.Operation
		object          unstructured.Unstructured
		oldObject       unstructured.Unstructured
		wantQueue       bool
		wantDelete      bool
		wantSynchronize bool
	}{
		{
			name:      "create named source",
			operation: admissionv1.Create,
			object:    source,
			wantQueue: true,
		},
		{
			name:      "create different name in the source namespace",
			operation: admissionv1.Create,
			object:    secretObject("src", "unrelated"),
		},
		{
			name:      "create named source in another namespace",
			operation: admissionv1.Create,
			object:    secretObject("other", "source-secret"),
		},
		{
			name:      "update unrelated secret",
			operation: admissionv1.Update,
			object:    other,
			oldObject: other,
		},
		{
			name:      "delete unrelated secret",
			operation: admissionv1.Delete,
			oldObject: other,
		},
		{
			name:            "update named source",
			operation:       admissionv1.Update,
			object:          source,
			oldObject:       source,
			wantQueue:       true,
			wantSynchronize: true,
		},
		{
			name:       "delete named source",
			operation:  admissionv1.Delete,
			oldObject:  source,
			wantQueue:  true,
			wantDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name := tt.object.GetName()
			if name == "" {
				name = tt.oldObject.GetName()
			}
			namespace := tt.object.GetNamespace()
			if namespace == "" {
				namespace = tt.oldObject.GetNamespace()
			}
			plan := planGeneratingAdmission(policy, secretAdmission(tt.operation, namespace, name), tt.object, tt.oldObject)
			assert.Equal(t, tt.wantQueue, plan.queue)
			assert.Equal(t, tt.wantDelete, plan.deleteDownstream)
			assert.Equal(t, tt.wantSynchronize, plan.synchronize)
		})
	}
}

func TestPlanGeneratingAdmissionKeepsTriggerThatStopsMatching(t *testing.T) {
	t.Parallel()
	policy := cloneSecretPolicy(true, `object.metadata.labels.keep == "yes"`)
	oldObject := secretObject("src", "source-secret")
	object := secretObject("src", "source-secret")
	unstructured.RemoveNestedField(object.Object, "metadata", "labels", "keep")

	plan := planGeneratingAdmission(policy, secretAdmission(admissionv1.Update, "src", "source-secret"), object, oldObject)
	assert.True(t, plan.queue)
	assert.True(t, plan.synchronize)
	assert.False(t, plan.deleteDownstream)
}

func TestPlanGeneratingAdmissionDeleteKeepsTriggerWhenConditionReadsRequest(t *testing.T) {
	t.Parallel()
	policy := cloneSecretPolicy(true, `request.operation == "CREATE" && object.metadata.namespace == "src"`)
	source := secretObject("src", "source-secret")

	deleted := planGeneratingAdmission(policy, secretAdmission(admissionv1.Delete, "src", "source-secret"), unstructured.Unstructured{}, source)
	assert.True(t, deleted.queue)
	assert.True(t, deleted.deleteDownstream)
	assert.False(t, deleted.synchronize)

	updated := planGeneratingAdmission(policy, secretAdmission(admissionv1.Update, "src", "source-secret"), source, source)
	assert.True(t, updated.queue)
	assert.True(t, updated.synchronize)
	assert.False(t, updated.deleteDownstream)

	other := secretObject("other", "unrelated")
	unrelated := planGeneratingAdmission(policy, secretAdmission(admissionv1.Delete, "other", "unrelated"), unstructured.Unstructured{}, other)
	assert.False(t, unrelated.queue)
}

func TestConditionsMatchTreatsRequestAsInconclusiveOnCleanup(t *testing.T) {
	t.Parallel()
	conditions := []admissionregistrationv1.MatchCondition{{
		Name:       "on-create",
		Expression: `request.operation == "CREATE" && object.metadata.namespace == "src"`,
	}}
	source := secretObject("src", "source-secret")
	request := secretAdmission(admissionv1.Delete, "src", "source-secret")

	assert.False(t, conditionsMatch(conditions, request, &source, &source, false))
	assert.True(t, conditionsMatch(conditions, request, &source, &source, true))

	objectOnly := []admissionregistrationv1.MatchCondition{{
		Name:       "source-namespace",
		Expression: `object.metadata.namespace == "src"`,
	}}
	elsewhere := secretObject("other", "source-secret")
	assert.False(t, conditionsMatch(objectOnly, secretAdmission(admissionv1.Delete, "other", "source-secret"), &elsewhere, &elsewhere, true))
}

func TestPlanGeneratingAdmissionIgnoresKyvernoOnlyMatchCondition(t *testing.T) {
	t.Parallel()
	policy := cloneSecretPolicy(true, `resource.Get("v1", "secrets", "src", "source-secret") != null && object.metadata.namespace == "src"`)

	matched := planGeneratingAdmission(policy, secretAdmission(admissionv1.Create, "src", "source-secret"), secretObject("src", "source-secret"), unstructured.Unstructured{})
	assert.True(t, matched.queue, "a Kyverno-only function must not hide a trigger the rest of the condition accepts")

	unrelated := planGeneratingAdmission(policy, secretAdmission(admissionv1.Update, "other", "unrelated"), secretObject("other", "unrelated"), secretObject("other", "unrelated"))
	assert.False(t, unrelated.queue)
}

func cloneSecretPolicy(sync bool, expression string) *policiesv1beta1.GeneratingPolicy {
	spec := policiesv1beta1.GeneratingPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				ResourceNames: []string{"source-secret"},
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
					},
				},
			}},
		},
		MatchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "source-namespace",
			Expression: expression,
		}},
	}
	if sync {
		spec.EvaluationConfiguration = &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
			SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
				Enabled: ptr.To(true),
			},
		}
	}
	return &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "clone-example"},
		Spec:       spec,
	}
}

func secretObject(namespace, name string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]any{
				"keep": "yes",
			},
		},
	}}
}

func secretAdmission(operation admissionv1.Operation, namespace, name string) admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Operation: operation,
		Name:      name,
		Namespace: namespace,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
	}
}
