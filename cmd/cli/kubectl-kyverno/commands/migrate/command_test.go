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

// TestCommandRequiresResource verifies that the migrate command requires the --resource flag.
func TestCommandRequiresResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, `required flag(s) "resource" not set`)
}

// TestCommandRejectsEmptyResource verifies that passing an empty or whitespace-only string to --resource is rejected.
func TestCommandRejectsEmptyResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"--resource", "   "})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "resource cannot be empty")
}

// TestMigrateResources_SkipsMissingCRD verifies that a CRD not present on the cluster is skipped cleanly without failing the command.
func TestMigrateResources_SkipsMissingCRD(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	apiServerClient := apiextensionsfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	err := migrateResources(context.Background(), []string{"nonexistent.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)
}

// TestMigrateResources_UpToDateCRD verifies that a CRD whose storedVersions is already up to date requires no resource updates.
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

// TestMigrateResources_MigratesAndUpdatesStoredVersions verifies that existing custom resources are rewritten and status.storedVersions is updated.
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

// TestMigrateResources_SkipsLegacyPolicyKind verifies that legacy policy kinds with admission write blocks are skipped cleanly.
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

// TestMigrateResources_IgnoresDeletedResourceDuringUpdate verifies that an in-flight deletion during update does not stop subsequent objects from migrating.
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

// TestMigrateResources_FailsOnUnrelatedUpdateError_LeavesStoredVersionsUnchanged verifies that unexpected update errors abort migration and preserve storedVersions.
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

// TestMigrateResources_SkipsResourceWhenListReturnsNotFound verifies that if a resource returns NotFound during dynamic listing, migration skips it cleanly.
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

// TestMigrateResources_SkipsCRDWhenStatusRefetchReturnsNotFound verifies that if a CRD returns NotFound during status refetch, migration skips to the next CRD.
func TestMigrateResources_SkipsCRDWhenStatusRefetchReturnsNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd1 := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crd1.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "crd1s",
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
	crd2 := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crd2.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "crd2s",
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

	gvr1 := schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "crd1s"}
	gvr2 := schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "crd2s"}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd1, crd2)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr1: "Crd1List",
		gvr2: "Crd2List",
	})

	// When refetching crd1 in status update, return NotFound (simulating deletion before status update Get)
	firstGetCRD1 := true
	apiServerClient.PrependReactor("get", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "crd1.policies.kyverno.io" {
			if firstGetCRD1 {
				firstGetCRD1 = false
				return false, nil, nil
			}
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "crd1.policies.kyverno.io")
		}
		return false, nil, nil
	})

	err := migrateResources(context.Background(), []string{"crd1.policies.kyverno.io", "crd2.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)

	// Verify crd2 was migrated successfully and its storedVersions updated
	updatedCrd2, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "crd2.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, updatedCrd2.Status.StoredVersions)
}

// TestMigrateResources_SkipsCRDWhenStatusUpdateReturnsNotFound verifies that if a CRD returns NotFound during UpdateStatus, migration skips to the next CRD.
func TestMigrateResources_SkipsCRDWhenStatusUpdateReturnsNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	crd1 := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crd1.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "crd1s",
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
	crd2 := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crd2.policies.kyverno.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "policies.kyverno.io",
			Names: v1.CustomResourceDefinitionNames{
				Plural: "crd2s",
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

	gvr1 := schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "crd1s"}
	gvr2 := schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "crd2s"}

	apiServerClient := apiextensionsfake.NewSimpleClientset(crd1, crd2)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr1: "Crd1List",
		gvr2: "Crd2List",
	})

	// When updating status for crd1, return NotFound (simulating deletion during status update)
	apiServerClient.PrependReactor("update", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			updateAction := action.(ktesting.UpdateAction)
			obj := updateAction.GetObject().(*v1.CustomResourceDefinition)
			if obj.GetName() == "crd1.policies.kyverno.io" {
				return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "crd1.policies.kyverno.io")
			}
		}
		return false, nil, nil
	})

	err := migrateResources(context.Background(), []string{"crd1.policies.kyverno.io", "crd2.policies.kyverno.io"}, apiServerClient, dynamicClient)
	require.NoError(t, err)

	// Verify crd2 was migrated successfully and its storedVersions updated
	updatedCrd2, err := apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "crd2.policies.kyverno.io", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, updatedCrd2.Status.StoredVersions)
}

// TestMigrateResources_AbortsWhenCRDGenerationOrUIDChanges verifies that if a CRD UID or generation changes concurrently, migration aborts without writing status.
func TestMigrateResources_AbortsWhenCRDGenerationOrUIDChanges(t *testing.T) {
	testCases := []struct {
		name        string
		mutateCRD   func(crd *v1.CustomResourceDefinition)
		expectedErr string
	}{
		{
			name: "generation changed",
			mutateCRD: func(crd *v1.CustomResourceDefinition) {
				crd.Generation = 2
			},
			expectedErr: "was modified during migration (UID or generation changed), please rerun migration",
		},
		{
			name: "UID changed",
			mutateCRD: func(crd *v1.CustomResourceDefinition) {
				crd.UID = "uid-2"
			},
			expectedErr: "was modified during migration (UID or generation changed), please rerun migration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			firstGet := true
			apiServerClient.PrependReactor("get", "customresourcedefinitions", func(action ktesting.Action) (bool, runtime.Object, error) {
				if firstGet {
					firstGet = false
					return false, nil, nil
				}
				modifiedCRD := crd.DeepCopy()
				tc.mutateCRD(modifiedCRD)
				return true, modifiedCRD, nil
			})

			err := migrateResources(context.Background(), []string{"testpolicies.policies.kyverno.io"}, apiServerClient, dynamicClient)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectedErr)

			for _, action := range apiServerClient.Actions() {
				if action.GetVerb() == "update" && action.GetSubresource() == "status" {
					t.Fatalf("unexpected UpdateStatus action when CRD identity was modified: %v", action)
				}
			}

			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}
			trackedObj, err := apiServerClient.Tracker().Get(crdGVR, "", "testpolicies.policies.kyverno.io")
			require.NoError(t, err)
			trackedCrd := trackedObj.(*v1.CustomResourceDefinition)
			require.Equal(t, []string{"v1alpha1", "v1"}, trackedCrd.Status.StoredVersions)
		})
	}
}

// TestMigrateResources_AbortsWhenStorageVersionChangesDuringMigration verifies that if a CRD storage version is changed concurrently, migration aborts without writing status.
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

	for _, action := range apiServerClient.Actions() {
		if action.GetVerb() == "update" && action.GetSubresource() == "status" {
			t.Fatalf("unexpected UpdateStatus action when storage version was modified: %v", action)
		}
	}

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	trackedObj, err := apiServerClient.Tracker().Get(crdGVR, "", "testpolicies.policies.kyverno.io")
	require.NoError(t, err)
	trackedCrd := trackedObj.(*v1.CustomResourceDefinition)
	require.Equal(t, []string{"v1alpha1", "v1"}, trackedCrd.Status.StoredVersions)
}

// TestMigrateResources_RetriesOnConflict verifies that optimistic concurrency conflicts during status updates are retried with the latest resourceVersion.
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
