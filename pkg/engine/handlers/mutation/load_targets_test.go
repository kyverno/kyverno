package mutation

import (
	"context"
	"fmt"
	"io"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type fakeTargetClient struct {
	resources []engineapi.Resource
	call      getResourcesCall
}

type getResourcesCall struct {
	group       string
	version     string
	kind        string
	subresource string
	namespace   string
	name        string
	lselector   *metav1.LabelSelector
}

func (c *fakeTargetClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeTargetClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeTargetClient) GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string, lselector *metav1.LabelSelector) ([]engineapi.Resource, error) {
	c.call = getResourcesCall{
		group:       group,
		version:     version,
		kind:        kind,
		subresource: subresource,
		namespace:   namespace,
		name:        name,
		lselector:   lselector,
	}
	return c.resources, nil
}

func (c *fakeTargetClient) GetNamespace(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeTargetClient) IsNamespaced(group, version, kind string) (bool, error) {
	return true, nil
}

func (c *fakeTargetClient) CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, string, error) {
	return true, "", nil
}

func (c *fakeTargetClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

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

func Test_getTargets(t *testing.T) {
	tests := []struct {
		testName        string
		target          kyvernov1.ResourceSpec
		policy          kyvernov1.PolicyInterface
		wantGroup       string
		wantVersion     string
		wantKind        string
		wantSubresource string
		wantNamespace   string
		wantName        string
	}{
		{
			testName: "core-api-group",
			target: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Service",
				Namespace:  "default",
				Name:       "demo-frontend",
			},
			policy:          &kyvernov1.ClusterPolicy{},
			wantGroup:       "",
			wantVersion:     "v1",
			wantKind:        "Service",
			wantSubresource: "",
			wantNamespace:   "default",
			wantName:        "demo-frontend",
		},
		{
			testName: "split-subresource-from-kind",
			target: kyvernov1.ResourceSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment/status",
				Namespace:  "default",
				Name:       "demo-app",
			},
			policy:          &kyvernov1.ClusterPolicy{},
			wantGroup:       "apps",
			wantVersion:     "v1",
			wantKind:        "Deployment",
			wantSubresource: "status",
			wantNamespace:   "default",
			wantName:        "demo-app",
		},
		{
			testName: "preserve-non-core-api-group",
			target: kyvernov1.ResourceSpec{
				APIVersion: "serving.knative.dev/v1",
				Kind:       "Service",
				Namespace:  "default",
				Name:       "demo-knative-service",
			},
			policy:          &kyvernov1.ClusterPolicy{},
			wantGroup:       "serving.knative.dev",
			wantVersion:     "v1",
			wantKind:        "Service",
			wantSubresource: "",
			wantNamespace:   "default",
			wantName:        "demo-knative-service",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			client := &fakeTargetClient{}
			policyCtx := policycontext.PolicyContext{}.WithPolicy(test.policy)

			resources, err := getTargets(context.Background(), client, test.target, policyCtx, nil)
			require.NoError(t, err)
			assert.Empty(t, resources)
			assert.Equal(t, test.wantGroup, client.call.group)
			assert.Equal(t, test.wantVersion, client.call.version)
			assert.Equal(t, test.wantKind, client.call.kind)
			assert.Equal(t, test.wantSubresource, client.call.subresource)
			assert.Equal(t, test.wantNamespace, client.call.namespace)
			assert.Equal(t, test.wantName, client.call.name)
			assert.Nil(t, client.call.lselector)
		})
	}
}
