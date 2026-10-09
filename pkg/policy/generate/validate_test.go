package generate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"gotest.tools/v3/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	var err error
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "(podSelector)": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var genRule kyverno.Generation
	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}

	rawGenerate = []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker = NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}
}

func newGenerateAuthClient(t *testing.T, allowed func(authorizationv1.SubjectAccessReviewSpec) bool) (dclient.Interface, *[]authorizationv1.SubjectAccessReviewSpec) {
	t.Helper()
	client := dclient.NewEmptyFakeClient()
	kubeClient, ok := client.GetKubeClient().(*kubefake.Clientset)
	assert.Assert(t, ok)
	reviews := []authorizationv1.SubjectAccessReviewSpec{}
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		reviews = append(reviews, *review.Spec.DeepCopy())
		return true, &authorizationv1.SubjectAccessReview{
			ObjectMeta: metav1.ObjectMeta{Name: review.Name},
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: allowed(review.Spec),
				Reason:  "test authorization decision",
			},
		}, nil
	})
	return client, &reviews
}

func generatePattern(kind, namespace string) kyverno.GeneratePattern {
	return kyverno.GeneratePattern{
		ResourceSpec: kyverno.ResourceSpec{
			APIVersion: "v1",
			Kind:       kind,
			Name:       "generated",
			Namespace:  namespace,
		},
	}
}

func TestGenerateAuthorAuthorization(t *testing.T) {
	groups := []string{"system:authenticated", "policy-authors"}
	client, reviews := newGenerateAuthClient(t, func(authorizationv1.SubjectAccessReviewSpec) bool { return true })
	rule := &kyverno.Rule{
		Generation: &kyverno.Generation{GeneratePattern: generatePattern("ConfigMap", "target")},
	}
	checker := NewGenerateFactoryWithGroups(client, rule, "alice", groups, "", logr.Discard())
	_, _, err := checker.Validate(context.Background(), nil)
	assert.NilError(t, err)
	assert.Equal(t, len(*reviews), 2)
	assert.DeepEqual(t, (*reviews)[0].Groups, groups)
	for _, review := range *reviews {
		assert.Equal(t, review.User, "alice")
		assert.Equal(t, review.ResourceAttributes.Namespace, "target")
	}
	assert.Equal(t, (*reviews)[0].ResourceAttributes.Verb, "get")
	assert.Equal(t, (*reviews)[1].ResourceAttributes.Verb, "create")
}

func TestGenerateVariableTargetsAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		generation kyverno.Generation
	}{
		{
			name: "direct",
			generation: kyverno.Generation{
				GeneratePattern: generatePattern("{{request.object.kind}}", "target"),
			},
		},
		{
			name: "direct apiVersion",
			generation: kyverno.Generation{
				GeneratePattern: kyverno.GeneratePattern{
					ResourceSpec: kyverno.ResourceSpec{
						APIVersion: "{{request.object.apiVersion}}",
						Kind:       "ConfigMap",
						Name:       "generated",
						Namespace:  "target",
					},
				},
			},
		},
		{
			name: "foreach",
			generation: kyverno.Generation{
				ForEachGeneration: []kyverno.ForEachGeneration{{GeneratePattern: generatePattern("{{element.kind}}", "target")}},
			},
		},
		{
			name: "foreach apiVersion",
			generation: kyverno.Generation{
				ForEachGeneration: []kyverno.ForEachGeneration{{GeneratePattern: kyverno.GeneratePattern{
					ResourceSpec: kyverno.ResourceSpec{
						APIVersion: "{{element.apiVersion}}",
						Kind:       "ConfigMap",
						Name:       "generated",
						Namespace:  "target",
					},
				}}},
			},
		},
		{
			name: "cloneList",
			generation: kyverno.Generation{
				GeneratePattern: kyverno.GeneratePattern{
					ResourceSpec: kyverno.ResourceSpec{Namespace: "target"},
					CloneList:    kyverno.CloneList{Kinds: []string{"{{request.object.kind}}"}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, reviews := newGenerateAuthClient(t, func(authorizationv1.SubjectAccessReviewSpec) bool { return true })
			checker := NewGenerateFactoryWithGroups(client, &kyverno.Rule{Generation: &test.generation}, "alice", nil, "", logr.Discard())
			_, _, err := checker.Validate(context.Background(), nil)
			assert.ErrorContains(t, err, "apiVersion and kind must be static")
			assert.Equal(t, len(*reviews), 0)

			offline := NewFakeGenerate(test.generation)
			warnings, path, err := offline.Validate(context.Background(), nil)
			assert.NilError(t, err)
			assert.Equal(t, path, "")
			assert.Equal(t, len(warnings), 0)
		})
	}
}

func TestGenerateVariableNamespaceRequiresClusterWideAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		generation kyverno.Generation
	}{
		{
			name:       "direct",
			generation: kyverno.Generation{GeneratePattern: generatePattern("ConfigMap", "{{request.object.metadata.namespace}}")},
		},
		{
			name: "foreach",
			generation: kyverno.Generation{
				ForEachGeneration: []kyverno.ForEachGeneration{{GeneratePattern: generatePattern("ConfigMap", "{{element.namespace}}")}},
			},
		},
		{
			name: "cloneList",
			generation: kyverno.Generation{
				GeneratePattern: kyverno.GeneratePattern{
					ResourceSpec: kyverno.ResourceSpec{Namespace: "{{request.object.metadata.namespace}}"},
					CloneList:    kyverno.CloneList{Kinds: []string{"v1/ConfigMap"}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
				return spec.ResourceAttributes.Namespace == ""
			})
			checker := NewGenerateFactoryWithGroups(client, &kyverno.Rule{Generation: &test.generation}, "alice", nil, "", logr.Discard())
			_, _, err := checker.Validate(context.Background(), nil)
			assert.NilError(t, err)
			assert.Assert(t, len(*reviews) > 0)
			for _, review := range *reviews {
				assert.Equal(t, review.ResourceAttributes.Namespace, "")
			}
		})
	}
}

func TestGenerateCloneListChecksEveryKind(t *testing.T) {
	client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
		return spec.ResourceAttributes.Resource != "secrets"
	})
	rule := &kyverno.Rule{
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Namespace: "target"},
				CloneList:    kyverno.CloneList{Kinds: []string{"v1/ConfigMap", "v1/Secret"}},
			},
		},
	}
	checker := NewGenerateFactoryWithGroups(client, rule, "alice", nil, "", logr.Discard())
	_, _, err := checker.Validate(context.Background(), nil)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "Secret"))
	resources := make([]string, 0, len(*reviews))
	for _, review := range *reviews {
		resources = append(resources, review.ResourceAttributes.Resource)
	}
	assert.Assert(t, strings.Contains(strings.Join(resources, ","), "configmaps"))
	assert.Assert(t, strings.Contains(strings.Join(resources, ","), "secrets"))
}

func TestFakeGeneratePreservesStructuralValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		generation string
		path       string
		err        string
	}{
		{
			name:       "data anchors",
			generation: `{"kind":"{{request.object.kind}}","data":{"metadata":{"(name)":"generated"}}}`,
			path:       "data.",
			err:        "anchors not supported on generate resources",
		},
		{
			name:       "cloneList selector wildcards",
			generation: `{"cloneList":{"kinds":["{{request.object.kind}}"],"selector":{"matchLabels":{"app":"*"}}}}`,
			path:       "selector",
			err:        "wildcard characters `*/?` not supported",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var generation kyverno.Generation
			assert.NilError(t, json.Unmarshal([]byte(test.generation), &generation))
			checker := NewFakeGenerate(generation)
			_, path, err := checker.Validate(context.Background(), nil)
			assert.ErrorContains(t, err, test.err)
			assert.Assert(t, strings.HasPrefix(path, test.path))
		})
	}
}
