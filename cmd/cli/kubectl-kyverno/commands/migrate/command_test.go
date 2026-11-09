package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
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

func TestMigrateResources_SkipsLegacyPolicyKind(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterpolicies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Kind:   "ClusterPolicy",
				Plural: "clusterpolicies",
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
			StoredVersions: []string{"v1alpha2", "v1"},
		},
	}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	err := migrateResources(context.Background(), []string{"clusterpolicies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)

	unchangedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "clusterpolicies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1alpha2", "v1"}, unchangedCrd.Status.StoredVersions)
}

func TestMigrateResources_IgnoresDeletedResourceDuringUpdate(t *testing.T) {
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
			StoredVersions: []string{"v1alpha1", "v1"},
		},
	}

	transientPolicy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "policies.kyverno.io/v1",
			"kind":       "TestPolicy",
			"metadata": map[string]any{
				"name": "transient-policy",
			},
		},
	}
	persistedPolicy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "policies.kyverno.io/v1",
			"kind":       "TestPolicy",
			"metadata": map[string]any{
				"name": "persisted-policy",
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
	}, transientPolicy, persistedPolicy)

	persistedUpdated := false
	dynamicClient.PrependReactor("update", "testpolicies", func(action ktesting.Action) (bool, runtime.Object, error) {
		updateAction := action.(ktesting.UpdateAction)
		obj := updateAction.GetObject().(*unstructured.Unstructured)
		if obj.GetName() == "transient-policy" {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "policies.kyverno.io", Resource: "testpolicies"}, "transient-policy")
		}
		if obj.GetName() == "persisted-policy" {
			persistedUpdated = true
		}
		return false, nil, nil
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
	require.True(t, persistedUpdated, "persisted-policy should be updated even if prior object returned NotFound")

	updatedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, updatedCrd.Status.StoredVersions)
}

func TestMigrateResources_FailsOnUnrelatedUpdateError_LeavesStoredVersionsUnchanged(t *testing.T) {
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
			StoredVersions: []string{"v1alpha1", "v1"},
		},
	}

	policyObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "policies.kyverno.io/v1",
			"kind":       "TestPolicy",
			"metadata": map[string]any{
				"name": "failing-policy",
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

	dynamicClient.PrependReactor("update", "testpolicies", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("webhook connection refused")
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.Error(t, err)
	require.ErrorContains(t, err, "webhook connection refused")

	unchangedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1alpha1", "v1"}, unchangedCrd.Status.StoredVersions)
}

func TestMigrateResources_SkipsResourceWhenListReturnsNotFound(t *testing.T) {
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
			StoredVersions: []string{"v1alpha1", "v1"},
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
	})

	dynamicClient.PrependReactor("list", "testpolicies", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "policies.kyverno.io", Resource: "testpolicies"}, "")
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)

	unchangedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1alpha1", "v1"}, unchangedCrd.Status.StoredVersions)
}

func TestMigrateResources_SkipsCRDWhenStatusUpdateReturnsNotFound(t *testing.T) {
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
			StoredVersions: []string{"v1alpha1", "v1"},
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
	})

	// Return NotFound when refetching CRD during status update
	firstGet := true
	apiServerClient.PrependReactor("get", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		if firstGet {
			firstGet = false
			return false, nil, nil
		}
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "testpolicies.policies.kyverno.io")
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
}

func TestMigrateResources_AbortsWhenCRDGenerationOrUIDChanges(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "testpolicies.policies.kyverno.io",
			Generation: 1,
			UID:        "uid-1",
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
			StoredVersions: []string{"v1alpha1", "v1"},
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
	})

	// When refetched during status update, simulate CRD having been modified concurrently (generation bumped)
	firstGet := true
	apiServerClient.PrependReactor("get", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		if firstGet {
			firstGet = false
			return false, nil, nil
		}
		modifiedCRD := crd.DeepCopy()
		modifiedCRD.Generation = 2
		return true, modifiedCRD, nil
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.Error(t, err)
	require.ErrorContains(t, err, "was modified during migration (UID or generation changed), please rerun migration")

	unchangedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1alpha1", "v1"}, unchangedCrd.Status.StoredVersions)
}

func TestMigrateResources_AbortsWhenStorageVersionChangesDuringMigration(t *testing.T) {
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

	gvr := schema.GroupVersionResource{
		Group:    "policies.kyverno.io",
		Version:  "v1",
		Resource: "testpolicies",
	}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "TestPolicyList",
	})

	// When refetched during status update, simulate storage version having been switched to v2 concurrently
	firstGet := true
	apiServerClient.PrependReactor("get", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		if firstGet {
			firstGet = false
			return false, nil, nil
		}
		modifiedCRD := crd.DeepCopy()
		modifiedCRD.Spec.Versions = []v1.CustomResourceDefinitionVersion{
			{
				Name:    "v1",
				Storage: false,
				Served:  true,
			},
			{
				Name:    "v2",
				Storage: true,
				Served:  true,
			},
		}
		return true, modifiedCRD, nil
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.Error(t, err)
	require.ErrorContains(t, err, "storage version changed from v1 to v2 during migration, please rerun migration")

	unchangedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1alpha1", "v1"}, unchangedCrd.Status.StoredVersions)
}

func TestMigrateResources_RetriesOnConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testpolicies.policies.kyverno.io",
			ResourceVersion: "1",
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
			StoredVersions: []string{"v1alpha1", "v1"},
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
	})

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	attempts := 0
	apiServerClient.PrependReactor("update", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" && attempts == 0 {
			attempts++
			// Simulate a concurrent controller updating the CRD's ResourceVersion
			updatedCRD := crd.DeepCopy()
			updatedCRD.SetResourceVersion("2")
			_ = apiServerClient.Tracker().Update(crdGVR, updatedCRD, "")
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "testpolicies.policies.kyverno.io", errors.New("conflict"))
		}
		if action.GetSubresource() == "status" && attempts == 1 {
			updateAction := action.(ktesting.UpdateAction)
			updatedObj := updateAction.GetObject().(*v1.CustomResourceDefinition)
			require.Equal(t, "2", updatedObj.GetResourceVersion(), "retried update must use latest resourceVersion")
		}
		return false, nil, nil
	})

	err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
	require.Equal(t, 1, attempts)

	updatedCrd, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "testpolicies.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, updatedCrd.Status.StoredVersions)
}
