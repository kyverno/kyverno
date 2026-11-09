package migrate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestCommandRequiresResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, `required flag(s) "resource" not set`)
}

func TestCommandRejectsEmptyResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"--resource", "   "})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "resource cannot be empty")
}

func TestMigrateResources_SkipsMissingCRD(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	apiServerClient := apiextensionsfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	err := migrateResources(context.Background(), []string{"nonexistent.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
}

func TestMigrateResources_UpToDateCRD(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testpolicies.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "testpolicies",
			},
			Scope: v1.ClusterScoped,
			Versions: []v1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Storage: true,
					Served:  true,
				},
			},
		},
		Status: v1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1"},
		},
	}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
}

func TestMigrateResources_MigratesAndUpdatesStoredVersions(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testpolicies.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "testpolicies",
			},
			Scope: v1.ClusterScoped,
			Versions: []v1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Storage: false,
					Served:  true,
				},
				{
					Name:    "v1",
					Storage: true,
					Served:  true,
				},
			},
		},
		Status: v1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1alpha1", "v1"},
		},
	}

	policyObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "policies.kyverno.io/v1",
			"kind":       "TestPolicy",
			"metadata": map[string]any{
				"name": "my-policy",
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "policies.kyverno.io",
		Version:  "v1",
		Resource: "testpolicies",
	}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "TestPolicyList",
	}, policyObj)

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)

	updatedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, updatedCrd.Status.StoredVersions)
}
