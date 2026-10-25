package migrate

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apifake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestGlobalContextMigrationUpdatesObjectsBeforeStoredVersions(t *testing.T) {
	t.Parallel()

	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprintf("%d objects", count), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			crd, gvr := migrationGCECRD()
			objects := make([]runtime.Object, 0, count)
			for i := range count {
				objects = append(objects, migrationGCE(fmt.Sprintf("entry-%d", i)))
			}
			dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "GlobalContextEntryList"}, objects...)
			apiClient := apifake.NewSimpleClientset(crd.DeepCopy())
			var updated []string
			dynamicClient.PrependReactor("update", "globalcontextentries", func(action clienttesting.Action) (bool, runtime.Object, error) {
				if action.GetResource().Version != "v2" {
					t.Fatalf("migration used %s, want v2", action.GetResource().Version)
				}
				updated = append(updated, action.(clienttesting.UpdateAction).GetObject().(*unstructured.Unstructured).GetName())
				return false, nil, nil
			})
			apiClient.PrependReactor("update", "customresourcedefinitions", func(action clienttesting.Action) (bool, runtime.Object, error) {
				if action.GetSubresource() != "status" || len(updated) != count {
					t.Fatalf("storedVersions updated before all objects: subresource=%s, writes=%d, want=%d", action.GetSubresource(), len(updated), count)
				}
				return false, nil, nil
			})

			if err := migrate(ctx, crd, dynamicClient, apiClient); err != nil {
				t.Fatal(err)
			}
			stored, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(stored.Status.StoredVersions, []string{"v2"}) {
				t.Fatalf("storedVersions = %v, want [v2]", stored.Status.StoredVersions)
			}
			for _, obj := range objects {
				original := obj.(*unstructured.Unstructured)
				actual, err := dynamicClient.Resource(gvr).Get(ctx, original.GetName(), metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual.Object, original.Object) {
					t.Fatalf("migration changed object %s", original.GetName())
				}
			}
			writes := len(updated)
			if err := migrate(ctx, stored, dynamicClient, apiClient); err != nil {
				t.Fatal(err)
			}
			if len(updated) != writes {
				t.Fatal("completed migration rewrote objects on rerun")
			}
		})
	}
}

func TestGlobalContextMigrationRetainsMarkersOnPartialFailure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	crd, gvr := migrationGCECRD()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "GlobalContextEntryList"}, migrationGCE("a"), migrationGCE("b"))
	apiClient := apifake.NewSimpleClientset(crd.DeepCopy())
	conflict := true
	writes := 0
	dynamicClient.PrependReactor("update", "globalcontextentries", func(action clienttesting.Action) (bool, runtime.Object, error) {
		writes++
		obj := action.(clienttesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		if conflict && writes == 2 {
			return true, nil, apierrors.NewConflict(gvr.GroupResource(), obj.GetName(), errors.New("injected conflict"))
		}
		return false, nil, nil
	})
	if err := migrate(ctx, crd, dynamicClient, apiClient); !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
	stored, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 2 || !reflect.DeepEqual(stored.Status.StoredVersions, []string{"v2beta1", "v2"}) {
		t.Fatalf("partial migration cleared markers: writes=%d, storedVersions=%v", writes, stored.Status.StoredVersions)
	}
	conflict = false
	if err := migrate(ctx, stored, dynamicClient, apiClient); err != nil {
		t.Fatal(err)
	}
	stored, err = apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 4 || !reflect.DeepEqual(stored.Status.StoredVersions, []string{"v2"}) {
		t.Fatalf("migration did not converge: writes=%d, storedVersions=%v", writes, stored.Status.StoredVersions)
	}
}

func migrationGCECRD() (*apiextensionsv1.CustomResourceDefinition, schema.GroupVersionResource) {
	gvr := schema.GroupVersionResource{Group: "kyverno.io", Version: "v2", Resource: "globalcontextentries"}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "globalcontextentries.kyverno.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: gvr.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: gvr.Resource, Kind: "GlobalContextEntry"},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: "v2beta1", Served: true},
				{Name: "v2", Served: true, Storage: true},
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v2beta1", "v2"}},
	}, gvr
}

func migrationGCE(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kyverno.io/v2", "kind": "GlobalContextEntry",
		"metadata": map[string]interface{}{"name": name, "uid": "uid-" + name, "annotations": map[string]interface{}{"example.com/keep": "value"}},
		"spec":     map[string]interface{}{"kubernetesResource": map[string]interface{}{"version": "v1", "resource": "configmaps"}},
		"status":   map[string]interface{}{"ready": true, "lastRefreshTime": "2026-10-25T00:00:00Z"},
	}}
}

func TestGlobalContextMigrationRetainsMarkersOnStatusFailure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	crd, gvr := migrationGCECRD()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "GlobalContextEntryList"}, migrationGCE("a"))
	apiClient := apifake.NewSimpleClientset(crd.DeepCopy())
	failure := errors.New("injected CRD status failure")
	failStatus := true
	writes := 0
	dynamicClient.PrependReactor("update", "globalcontextentries", func(action clienttesting.Action) (bool, runtime.Object, error) {
		writes++
		return false, nil, nil
	})
	apiClient.PrependReactor("update", "customresourcedefinitions", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if failStatus && action.GetSubresource() == "status" {
			return true, nil, failure
		}
		return false, nil, nil
	})
	if err := migrate(ctx, crd, dynamicClient, apiClient); !errors.Is(err, failure) {
		t.Fatalf("expected CRD status failure, got %v", err)
	}
	stored, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 1 || !reflect.DeepEqual(stored.Status.StoredVersions, []string{"v2beta1", "v2"}) {
		t.Fatalf("failed status update cleared markers: writes=%d, storedVersions=%v", writes, stored.Status.StoredVersions)
	}
	failStatus = false
	if err := migrate(ctx, stored, dynamicClient, apiClient); err != nil {
		t.Fatal(err)
	}
	stored, err = apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 2 || !reflect.DeepEqual(stored.Status.StoredVersions, []string{"v2"}) {
		t.Fatalf("migration did not converge: writes=%d, storedVersions=%v", writes, stored.Status.StoredVersions)
	}
}
