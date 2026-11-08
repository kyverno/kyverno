package mutation

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_match(t *testing.T) {
	tests := []struct {
		testName         string
		namespacePattern string
		namePattern      string
		namespace        string
		name             string
		expectedResult   bool
	}{
		{
			testName:         "empty-namespacePattern-namePattern-1",
			namespacePattern: "",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-namePattern-2",
			namespacePattern: "",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-1",
			namespacePattern: "",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-2",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-3",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "empty-namespacePattern-4",
			namespacePattern: "",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-5",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-6",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-1",
			namespacePattern: "foo",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-2",
			namespacePattern: "foo",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namePattern-3",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namePattern-4",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-5",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-1",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "",
			name:             "",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-2",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-3",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-4",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-5",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-6",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-7",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-8",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-9",
			namespacePattern: "foo",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-10",
			namespacePattern: "foo",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		// {
		// 	testName:         "",
		// 	namespacePattern: "",
		// 	namePattern:      "",
		// 	namespace:        "",
		// 	name:             "",
		// 	expectedResult:   false,
		// },
	}

	for _, test := range tests {
		res := match(test.namespacePattern, test.namePattern, test.namespace, test.name)
		assert.Equal(t, test.expectedResult, res, fmt.Sprintf("test %s failed", test.testName))
	}
}

type fakeTargetClient struct {
	engineapi.Client
	resources []engineapi.Resource
}

func (c fakeTargetClient) GetResources(_ context.Context, _, _, _, _, _, _ string, _ *metav1.LabelSelector) ([]engineapi.Resource, error) {
	return c.resources, nil
}

func newConfigMapTarget(namespace, name string) engineapi.Resource {
	obj := unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return engineapi.Resource{Version: "v1", Resource: "configmaps", Unstructured: obj}
}

func Test_loadTargets_skipsFilteredTargets(t *testing.T) {
	t.Parallel()
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "kyverno"},
		Data:       map[string]string{"resourceFilters": "[*/*,kyverno,*][*/*,kube-system,*]"},
	})
	trigger := unstructured.Unstructured{}
	trigger.SetAPIVersion("v1")
	trigger.SetKind("ConfigMap")
	trigger.SetNamespace("default")
	trigger.SetName("trigger")
	policyContext, err := policycontext.NewPolicyContext(jmespath.New(cfg), trigger, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)
	policyContext = policyContext.WithPolicy(&kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mutate-existing"}})
	client := fakeTargetClient{resources: []engineapi.Resource{
		newConfigMapTarget("kyverno", "kyverno"),
		newConfigMapTarget("kube-system", "coredns"),
		newConfigMapTarget("default", "app"),
	}}
	targets := []kyvernov1.TargetResourceSpec{{
		TargetSelector: kyvernov1.TargetSelector{
			ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap"},
		},
	}}

	loaded, err := loadTargets(context.Background(), client, cfg, targets, policyContext, logr.Discard())
	assert.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "default", loaded[0].unstructured.GetNamespace())
	assert.Equal(t, "app", loaded[0].unstructured.GetName())

	// without a configuration nothing is filtered
	loaded, err = loadTargets(context.Background(), client, nil, targets, policyContext, logr.Discard())
	assert.NoError(t, err)
	assert.Len(t, loaded, 3)
}
