package utils

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type mockResourceClient struct {
	resources map[string]*unstructured.Unstructured
}

func (m *mockResourceClient) GetResource(_ context.Context, _ string, kind string, namespace string, name string, _ ...string) (*unstructured.Unstructured, error) {
	key := kind + "/" + namespace + "/" + name
	if obj, ok := m.resources[key]; ok {
		return obj, nil
	}
	return nil, fmt.Errorf("resource not found: %s", key)
}
func (m *mockResourceClient) ListResource(_ context.Context, _ string, _ string, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, nil
}
func (m *mockResourceClient) GetResources(_ context.Context, _, _, _, _, _, _ string, _ *metav1.LabelSelector) ([]engineapi.Resource, error) {
	return nil, nil
}
func (m *mockResourceClient) GetNamespace(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, nil
}
func (m *mockResourceClient) IsNamespaced(_, _, _ string) (bool, error) {
	return true, nil
}
func (m *mockResourceClient) CanI(_ context.Context, _, _, _, _, _ string) (bool, string, error) {
	return true, "", nil
}
func (m *mockResourceClient) RawAbsPath(_ context.Context, _ string, _ string, _ io.Reader) ([]byte, error) {
	return nil, nil
}
func (m *mockResourceClient) BuildRemoteOption(_ context.Context) ([]gcrremote.Option, error) {
	return nil, nil
}
func (m *mockResourceClient) BuildNameOption() []name.Option {
	return nil
}

func makeTestUnstructured(kind, namespace, name, uid string, ts metav1.Time, ownerKind, ownerName, ownerUID string, isController bool) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetUID(types.UID(uid))
	obj.SetCreationTimestamp(ts)
	if kind == "Job" || kind == "CronJob" {
		obj.SetAPIVersion("batch/v1")
	} else if kind == "Pod" {
		obj.SetAPIVersion("v1")
	} else {
		obj.SetAPIVersion("apps/v1")
	}
	if ownerName != "" {
		controller := isController
		apiVer := "apps/v1"
		if ownerKind == "Job" || ownerKind == "CronJob" {
			apiVer = "batch/v1"
		}
		obj.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: apiVer,
			Kind:       ownerKind,
			Name:       ownerName,
			UID:        types.UID(ownerUID),
			Controller: &controller,
		}})
	}
	template := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "test",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	if kind == "Deployment" {
		_ = unstructured.SetNestedMap(obj.Object, template, "spec", "template")
	} else if kind == "ReplicaSet" {
		pts := corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
			},
		}
		hash := ComputePodTemplateHash(&pts, nil)
		obj.SetLabels(map[string]string{"pod-template-hash": hash})
		_ = unstructured.SetNestedField(obj.Object, hash, "spec", "selector", "matchLabels", "pod-template-hash")
		rsTemplate := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"app":               "test",
					"pod-template-hash": hash,
				},
			},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "nginx", "image": "nginx"},
				},
			},
		}
		_ = unstructured.SetNestedMap(obj.Object, rsTemplate, "spec", "template")
	} else if kind == "Job" {
		jobTemplate := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"app": "test",
				},
			},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "nginx", "image": "nginx"},
				},
			},
		}
		_ = unstructured.SetNestedMap(obj.Object, jobTemplate, "spec", "template")
		obj.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager:    "kube-controller-manager",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &ts,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)},
		}})
	} else if kind == "CronJob" {
		cronJobTemplate := map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "nginx", "image": "nginx"},
						},
					},
				},
			},
		}
		_ = unstructured.SetNestedMap(obj.Object, cronJobTemplate, "spec", "jobTemplate")
		obj.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager:    "kube-controller-manager",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &ts,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:jobTemplate":{}}}`)},
		}})
	} else if kind == "Pod" {
		podLabels := map[string]string{
			"app": "test",
		}
		if ownerKind == "ReplicaSet" {
			pts := corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
				},
			}
			podLabels["pod-template-hash"] = ComputePodTemplateHash(&pts, nil)
		}
		obj.SetLabels(podLabels)
		_ = unstructured.SetNestedField(obj.Object, []any{
			map[string]any{"name": "nginx", "image": "nginx"},
		}, "spec", "containers")
	}
	return obj
}

func TestRootOwnerCreationTimestamp_NoOwner(t *testing.T) {
	ts := metav1.Unix(1000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", ts, "", "", "", false)
	client := &mockResourceClient{}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil timestamp for unmanaged resource, got %v", got)
	}
}

func TestRootOwnerCreationTimestamp_WalksToRoot(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(2000, 0)
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected deploy timestamp %v, got nil", deployTs)
	}
	if !got.Time.Equal(deployTs.Time) {
		t.Errorf("expected deploy timestamp %v, got %v", deployTs, got)
	}
}

func TestRootOwnerCreationTimestamp_NewerChildReplicaSetTimestampWins(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(2000, 0) // New ReplicaSet created after template update
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected rs timestamp %v, got nil", rsTs)
	}
	// The newer ReplicaSet creation timestamp must be returned, not the older Deployment timestamp
	if !got.Time.Equal(rsTs.Time) {
		t.Errorf("expected newest timestamp %v, got %v", rsTs, got)
	}
}

func TestRootOwnerCreationTimestamp_NonControllerOwnerIgnored(t *testing.T) {
	podTs := metav1.Unix(3000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", false)
	client := &mockResourceClient{}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil timestamp when owner has Controller=false, got %v", got)
	}
}

func TestRootOwnerCreationTimestamp_OwnerNotFound_ReturnsError(t *testing.T) {
	ResetOwnerCaches()
	podTs := metav1.Unix(3000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs-missing", "rs-missing-uid", true)
	client := &mockResourceClient{resources: map[string]*unstructured.Unstructured{}}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error when owner not found, got nil")
	}
}

func TestRootOwnerCreationTimestamp_UIDMismatch(t *testing.T) {
	ResetOwnerCaches()
	podTs := metav1.Unix(3000, 0)
	rsTs := metav1.Unix(2000, 0)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "actual-rs-uid", rsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "forged-rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1": &rs,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error due to UID mismatch, got nil")
	}
}

func TestRootOwnerCreationTimestamp_LiveDirectEvaluationWithoutCache(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if !got.Time.Equal(rsTs.Time) {
		t.Fatalf("expected rsTs %v, got %v", rsTs, got)
	}

	// Update rs creationTimestamp in client, verifying subsequent call gets updated state after cache reset
	updatedRSTs := metav1.Unix(2500, 0)
	rsUpdated := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", updatedRSTs, "Deployment", "deploy1", "deploy-uid", true)
	client.resources["ReplicaSet/default/rs1"] = &rsUpdated

	ResetOwnerCaches()
	gotUpdated, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if !gotUpdated.Time.Equal(updatedRSTs.Time) {
		t.Fatalf("expected updated rsTs %v, got %v", updatedRSTs, gotUpdated)
	}
}

func TestRootOwnerCreationTimestamp_StatefulSet_MissingRevisionHash_ReturnsError(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	// Pod has no controller-revision-hash label
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1": &sts,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error when pod managed by StatefulSet lacks controller-revision-hash label, got nil")
	}
}

func TestRootOwnerCreationTimestamp_StatefulSet_RevisionNotFound_ReturnsError(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	pod.SetLabels(map[string]string{"controller-revision-hash": "hash123"})
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1": &sts,
			// ControllerRevision is missing
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error when ControllerRevision is missing, got nil")
	}
}

func TestRootOwnerCreationTimestamp_StatefulSet_WithRevision(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	revTs := metav1.Unix(2000, 0) // Pod template updated later
	podTs := metav1.Unix(3000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	pod.SetLabels(map[string]string{"app": "test", "controller-revision-hash": "hash123"})

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(revTs)
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "sts-uid",
		Controller: &isCtrl,
	}})
	revTemplate := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":                      "test",
				"controller-revision-hash": "hash123",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, revTemplate, "data", "spec", "template")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || !got.Time.Equal(revTs.Time) {
		t.Errorf("expected ControllerRevision timestamp %v to win, got %v", revTs, got)
	}
}

func TestRootOwnerCreationTimestamp_StatefulSet_FullRevisionName_Passes(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	revTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	// In real Kubernetes clusters, StatefulSet pods store the full ControllerRevision name in controller-revision-hash
	pod.SetLabels(map[string]string{"app": "test", "controller-revision-hash": "sts1-hash123"})

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(revTs)
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "sts-uid",
		Controller: &isCtrl,
	}})
	revTemplate := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":                      "test",
				"controller-revision-hash": "sts1-hash123",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, revTemplate, "data", "spec", "template")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.True(t, got.Time.Equal(revTs.Time))
}

func TestRootOwnerCreationTimestamp_StatefulSet_PVCVolume_Passes(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	revTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	pod.SetLabels(map[string]string{"app": "test", "controller-revision-hash": "sts1-hash123"})

	// StatefulSet controller adds PVC volume to Pod from spec.volumeClaimTemplates
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{
			"name": "data",
			"persistentVolumeClaim": map[string]any{
				"claimName": "data-pod1-0",
			},
		},
	}, "spec", "volumes")

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(revTs)
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "sts-uid",
		Controller: &isCtrl,
	}})
	revTemplate := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":                      "test",
				"controller-revision-hash": "sts1-hash123",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, revTemplate, "data", "spec", "template")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.True(t, got.Time.Equal(revTs.Time))
}

func TestRootOwnerCreationTimestamp_DaemonSet_WithRevision(t *testing.T) {
	ResetOwnerCaches()
	dsTs := metav1.Unix(1000, 0)
	revTs := metav1.Unix(2500, 0)
	podTs := metav1.Unix(3000, 0)
	ds := makeTestUnstructured("DaemonSet", "default", "ds1", "ds-uid", dsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "DaemonSet", "ds1", "ds-uid", true)
	pod.SetLabels(map[string]string{"app": "test", "controller-revision-hash": "dshash"})

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("ds1-dshash")
	rev.SetCreationTimestamp(revTs)
	isCtrlDS := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       "ds1",
		UID:        "ds-uid",
		Controller: &isCtrlDS,
	}})
	revTemplateDS := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":                      "test",
				"controller-revision-hash": "dshash",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, revTemplateDS, "data", "template")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"DaemonSet/default/ds1":                 &ds,
			"ControllerRevision/default/ds1-dshash": &rev,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || !got.Time.Equal(revTs.Time) {
		t.Errorf("expected ControllerRevision timestamp %v to win, got %v", revTs, got)
	}
}

func TestRootOwnerCreationTimestamp_StandaloneReplicaSet_ReturnsError(t *testing.T) {
	ResetOwnerCaches()
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	// Standalone ReplicaSet without Deployment controller
	rs := makeTestUnstructured("ReplicaSet", "default", "rs-standalone", "rs-uid", rsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs-standalone", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs-standalone": &rs,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error for standalone ReplicaSet without parent controller, got nil")
	}
}

func TestRootOwnerCreationTimestamp_ReplicationController_ReturnsError(t *testing.T) {
	ResetOwnerCaches()
	rcTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	rc := makeTestUnstructured("ReplicationController", "default", "rc1", "rc-uid", rcTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicationController", "rc1", "rc-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicationController/default/rc1": &rc,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error for ReplicationController, got nil")
	}
}

func TestRootOwnerCreationTimestamp_UnknownControllerKind_FailsClosed(t *testing.T) {
	ResetOwnerCaches()
	podTs := metav1.Unix(3000, 0)
	customTs := metav1.Unix(1000, 0)
	custom := makeTestUnstructured("CustomOperator", "default", "custom1", "custom-uid", customTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "CustomOperator", "custom1", "custom-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CustomOperator/default/custom1": &custom,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error for unknown controller kind without verified template-versioning, got nil")
	}
}

func TestRootOwnerCreationTimestamp_ParentChainRevalidation_DeletedParentInvalidatesCache(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	// Initial evaluation populates the cache with parent chain containing deploy1
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error on initial call: %v", err)
	}
	if !got.Time.Equal(rsTs.Time) {
		t.Fatalf("expected rsTs %v, got %v", rsTs, got)
	}

	// Delete deploy1 from the cluster (dangling parent chain)
	delete(client.resources, "Deployment/default/deploy1")

	// Invalidate cache before re-checking deleted parent
	ResetOwnerCaches()

	// Next call must revalidate parent chain live, detect deleted parent, and return error
	_, err = RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error when parent deployment is deleted, but got nil")
	}
}

func TestRootOwnerCreationTimestamp_ExceedsMaxDepth(t *testing.T) {
	ResetOwnerCaches()
	podTs := metav1.Unix(3000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod-loop", "loop-uid", podTs, "Job", "job-loop", "loop-uid", true)
	job := makeTestUnstructured("Job", "default", "job-loop", "loop-uid", podTs, "Job", "job-loop", "loop-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Job/default/job-loop": &job,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err == nil {
		t.Error("expected error when owner chain exceeds max depth, got nil")
	}
}

func TestRootOwnerCreationTimestamp_NilClient(t *testing.T) {
	ResetOwnerCaches()
	podTs := metav1.Unix(3000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-nil-uid", true)
	got, err := RootOwnerCreationTimestamp(context.TODO(), nil, pod)
	if err != nil {
		t.Fatalf("unexpected error with nil client: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil timestamp with nil client, got %v", got)
	}
}

func TestIsCreateRequest(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	res := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		},
	}

	t.Run("nil context returns false", func(t *testing.T) {
		assert.False(t, IsCreateRequest(nil))
	})

	t.Run("CREATE operation returns true", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Create, nil, cfg)
		assert.NoError(t, err)
		assert.True(t, IsCreateRequest(pc))
	})

	t.Run("DELETE operation returns false", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Delete, nil, cfg)
		assert.NoError(t, err)
		assert.False(t, IsCreateRequest(pc))
	})

	t.Run("UPDATE operation returns false", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Update, nil, cfg)
		assert.NoError(t, err)
		assert.False(t, IsCreateRequest(pc))
	})

	t.Run("CONNECT operation returns false", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Connect, nil, cfg)
		assert.NoError(t, err)
		assert.False(t, IsCreateRequest(pc))
	})

	t.Run("empty operation with nil old object and non-empty new object returns true", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, "", nil, cfg)
		assert.NoError(t, err)
		assert.True(t, IsCreateRequest(pc))
	})

	t.Run("empty operation with non-nil old object returns false", func(t *testing.T) {
		pc, err := policycontext.NewPolicyContext(jp, res, "", nil, cfg)
		assert.NoError(t, err)
		pc = pc.WithOldResource(res)
		assert.False(t, IsCreateRequest(pc))
	})

	t.Run("empty operation with empty new object returns false", func(t *testing.T) {
		emptyRes := unstructured.Unstructured{}
		pc, err := policycontext.NewPolicyContext(jp, emptyRes, "", nil, cfg)
		assert.NoError(t, err)
		pc = pc.WithNewResource(emptyRes)
		assert.False(t, IsCreateRequest(pc))
	})
}

func TestIsControllerAdmissionUser(t *testing.T) {
	tests := []struct {
		name     string
		userInfo authenticationv1.UserInfo
		want     bool
	}{
		{
			name:     "empty username returns false",
			userInfo: authenticationv1.UserInfo{},
			want:     false,
		},
		{
			name: "replicaset-controller returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			want: true,
		},
		{
			name: "daemon-set-controller returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:daemon-set-controller",
			},
			want: true,
		},
		{
			name: "kube-controller-manager returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:kube-controller-manager",
			},
			want: true,
		},
		{
			name: "node user with system:nodes group returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "system:node:worker-1",
				Groups:   []string{"system:nodes"},
			},
			want: false,
		},
		{
			name: "node user without system:nodes group returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "system:node:worker-1",
				Groups:   []string{"system:authenticated"},
			},
			want: false,
		},
		{
			name: "generic service account returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:ci",
				Groups:   []string{"system:serviceaccounts"},
			},
			want: false,
		},
		{
			name: "serviceaccounts group alone returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "custom-sa",
				Groups:   []string{"system:serviceaccounts"},
			},
			want: false,
		},
		{
			name: "nodes group without system:node: username prefix returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "custom-node",
				Groups:   []string{"system:nodes"},
			},
			want: false,
		},
		{
			name: "statefulset-controller returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			want: true,
		},
		{
			name: "job-controller returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:job-controller",
			},
			want: true,
		},
		{
			name: "cronjob-controller returns true",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:cronjob-controller",
			},
			want: true,
		},
		{
			name: "deployment-controller returns false (creates ReplicaSets not Pods)",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:deployment-controller",
			},
			want: false,
		},
		{
			name: "replication-controller returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replication-controller",
			},
			want: false,
		},
		{
			name: "untrusted user returns false",
			userInfo: authenticationv1.UserInfo{
				Username: "alice@example.com",
				Groups:   []string{"system:authenticated"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsControllerAdmissionUser(tt.userInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRootOwnerCreationTimestamp_ReplicaSet_InPlaceModification_Fails(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	// Tamper with ReplicaSet template in-place (e.g. change container image) while keeping selector hash
	_ = unstructured.SetNestedSlice(rs.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:tampered"},
	}, "spec", "template", "spec", "containers")
	// Even if managedFields is stripped by attacker
	rs.SetManagedFields(nil)

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template does not match immutable selector hash")
}

func TestRootOwnerCreationTimestamp_ReplicaSet_RolloutAndScale_Passes(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(1500, 0)
	podTs := metav1.Unix(3000, 0)
	// Deployment updated to v2, older ReplicaSet remains at v1
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	_ = unstructured.SetNestedMap(deploy.Object, map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "test"}},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "nginx", "image": "nginx:v2"}},
		},
	}, "spec", "template")

	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	// Even with kube-controller-manager updating managedFields after policy (e.g. scaling up)
	postPolicyTime := metav1.Unix(5000, 0)
	rs.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "kube-controller-manager",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		Time:       &postPolicyTime,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{},"f:template":{}}}`)},
	}})

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	// Even though Deployment template was updated to v2, older ReplicaSet at v1 succeeds during rollout/scale
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, rsTs.Time, got.Time)
}

func TestRootOwnerCreationTimestamp_Job_StrippedManagedFields_Fails(t *testing.T) {
	ResetOwnerCaches()
	jobTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "", "", "", false)
	// Attacker stripped managedFields to [{}] or empty
	job.SetManagedFields(nil)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Job/default/job1": &job,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no verifiable template managedFields")
}

func TestRootOwnerCreationTimestamp_CronJob_StrippedManagedFields_Fails(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	jobTs := metav1.Unix(2000, 0)
	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	// Attacker stripped managedFields to [{}] or empty
	cron.SetManagedFields(nil)
	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CronJob/default/cron1": &cron,
			"Job/default/job1":      &job,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no verifiable jobTemplate managedFields")
}

func TestRootOwnerCreationTimestamp_Deployment_PostPolicyManagedFieldsUnchangedTemplate_Passes(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	// Deployment has managedFields entry updated post-policy for replicas or annotations, but template unchanged
	postPolicyTime := metav1.Unix(5000, 0)
	deploy.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "kubectl",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		Time:       &postPolicyTime,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{},"f:template":{}}}`)},
	}})

	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	// Root owner timestamp remains pre-policy rsTs, not bumped by unrelated Deployment managedFields update
	assert.Equal(t, rsTs.Time, got.Time)
}

func TestRootOwnerCreationTimestamp_ControllerRevision_Unowned_Fails(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	pod.SetLabels(map[string]string{"controller-revision-hash": "hash123"})

	// Unowned ControllerRevision (forged by malicious actor)
	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(metav1.Unix(500, 0))

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not controlled by live StatefulSet/sts1")
}

func TestRootOwnerCreationTimestamp_ControllerRevision_WrongUID_Fails(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	// Live StatefulSet has UID "live-sts-uid"
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "live-sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "live-sts-uid", true)
	pod.SetLabels(map[string]string{"controller-revision-hash": "hash123"})

	// Revision belongs to an older/recreated StatefulSet with UID "old-recreated-sts-uid"
	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(metav1.Unix(500, 0))
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "old-recreated-sts-uid",
		Controller: &isCtrl,
	}})

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not controlled by live StatefulSet/sts1 (UID live-sts-uid)")
}

func TestRootOwnerCreationTimestamp_ControllerRevision_MismatchedPodSpec_Fails(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	pod.SetLabels(map[string]string{"controller-revision-hash": "hash123"})
	// Pod has a malicious container
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "malicious", "image": "malicious:latest"},
	}, "spec", "containers")

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(metav1.Unix(500, 0))
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "sts-uid",
		Controller: &isCtrl,
	}})
	// Revision template has benign nginx container
	_ = unstructured.SetNestedMap(rev.Object, map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx:1.14"},
					},
				},
			},
		},
	}, "data")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match ControllerRevision")
}

func TestRootOwnerCreationTimestamp_UnsupportedAPIVersion_Fails(t *testing.T) {
	ResetOwnerCaches()
	ts := metav1.Unix(1000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", ts, "ReplicaSet", "rs1", "rs-uid", true)
	// Spoof ownerReference APIVersion to a custom CRD
	pod.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "custom.io/v1",
		Kind:       "ReplicaSet",
		Name:       "rs1",
		UID:        "rs-uid",
		Controller: func() *bool { b := true; return &b }(),
	}})
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", ts, "Deployment", "deploy1", "deploy-uid", true)
	rs.SetAPIVersion("custom.io/v1")
	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1": &rs,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported ReplicaSet APIVersion")
}

func TestRootOwnerCreationTimestamp_ReplicaSet_MetadataMismatch_Fails(t *testing.T) {
	ResetOwnerCaches()
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "deploy1"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
		},
	}
	hash := ComputePodTemplateHash(&pts, nil)
	rs.SetLabels(map[string]string{
		"pod-template-hash": hash,
	})
	_ = unstructured.SetNestedField(rs.Object, hash, "spec", "selector", "matchLabels", "pod-template-hash")
	_ = unstructured.SetNestedMap(rs.Object, map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":               "deploy1",
				"pod-template-hash": hash,
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}, "spec", "template")

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	// Pod has mismatched template label value
	pod.SetLabels(map[string]string{
		"app":               "tampered",
		"pod-template-hash": hash,
	})
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx"},
	}, "spec", "containers")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match ReplicaSet rs1 template")
}

func TestRootOwnerCreationTimestamp_StatefulSet_MetadataMismatch_Fails(t *testing.T) {
	ResetOwnerCaches()
	stsTs := metav1.Unix(1000, 0)
	revTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", stsTs, "", "", "", false)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "StatefulSet", "sts1", "sts-uid", true)
	// Pod has mismatched template label
	pod.SetLabels(map[string]string{
		"app":                      "tampered",
		"controller-revision-hash": "hash123",
	})
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx"},
	}, "spec", "containers")

	rev := unstructured.Unstructured{}
	rev.SetAPIVersion("apps/v1")
	rev.SetKind("ControllerRevision")
	rev.SetNamespace("default")
	rev.SetName("sts1-hash123")
	rev.SetCreationTimestamp(revTs)
	isCtrl := true
	rev.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       "sts1",
		UID:        "sts-uid",
		Controller: &isCtrl,
	}})
	revTemplate := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":                      "sts1",
				"controller-revision-hash": "hash123",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, revTemplate, "data", "spec", "template")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"StatefulSet/default/sts1":                &sts,
			"ControllerRevision/default/sts1-hash123": &rev,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match ControllerRevision sts1-hash123 template")
}

func TestRootOwnerCreationTimestamp_CronJob_JobTemplateModification_AdvancesTimestamp(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	cronUpdateTs := metav1.Unix(2500, 0)
	jobTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(3000, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	cron.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "kubectl-edit",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:jobTemplate":{}}}`)},
		Time:       &cronUpdateTs,
	}})

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Pod/default/pod1":      &pod,
			"Job/default/job1":      &job,
			"CronJob/default/cron1": &cron,
		},
	}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, cronUpdateTs.Time, got.Time)
}

func TestRootOwnerCreationTimestamp_Job_MismatchCronJobTemplate_Fails(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	jobTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)
	// Modify Job template to something not in CronJob
	_ = unstructured.SetNestedSlice(job.Object, []any{
		map[string]any{"name": "malicious", "image": "malicious:latest"},
	}, "spec", "template", "spec", "containers")

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "malicious", "image": "malicious:latest"},
	}, "spec", "containers")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Pod/default/pod1":      &pod,
			"Job/default/job1":      &job,
			"CronJob/default/cron1": &cron,
		},
	}
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match CronJob cron1 jobTemplate")
}

func TestRootOwnerCreationTimestamp_UnexpectedDependentKind_Rejected(t *testing.T) {
	ResetOwnerCaches()
	ts := metav1.Unix(1000, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", ts, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", ts, "", "", "", false)
	sts := makeTestUnstructured("StatefulSet", "default", "sts1", "sts-uid", ts, "", "", "", false)
	ds := makeTestUnstructured("DaemonSet", "default", "ds1", "ds-uid", ts, "", "", "", false)
	job := makeTestUnstructured("Job", "default", "job1", "job-uid", ts, "", "", "", false)

	// Pod claiming CronJob ownerReference directly should be rejected
	podUnderCron := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", ts, "CronJob", "cron1", "cron-uid", true)
	// ConfigMap claiming CronJob ownerReference directly should be rejected
	cmUnderCron := makeTestUnstructured("ConfigMap", "default", "cm1", "cm-uid", ts, "CronJob", "cron1", "cron-uid", true)
	// Job claiming ReplicaSet ownerReference should be rejected
	jobUnderRS := makeTestUnstructured("Job", "default", "job1", "job-uid", ts, "ReplicaSet", "rs1", "rs-uid", true)
	// Job claiming StatefulSet ownerReference should be rejected
	jobUnderSTS := makeTestUnstructured("Job", "default", "job1", "job-uid", ts, "StatefulSet", "sts1", "sts-uid", true)
	// Job claiming DaemonSet ownerReference should be rejected
	jobUnderDS := makeTestUnstructured("Job", "default", "job1", "job-uid", ts, "DaemonSet", "ds1", "ds-uid", true)
	// ConfigMap claiming Job ownerReference should be rejected
	cmUnderJob := makeTestUnstructured("ConfigMap", "default", "cm1", "cm-uid", ts, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CronJob/default/cron1":    &cron,
			"ReplicaSet/default/rs1":   &rs,
			"StatefulSet/default/sts1": &sts,
			"DaemonSet/default/ds1":    &ds,
			"Job/default/job1":         &job,
		},
	}

	for _, tt := range []struct {
		name     string
		resource unstructured.Unstructured
		wantErr  string
	}{
		{
			name:     "Pod directly under CronJob is rejected",
			resource: podUnderCron,
			wantErr:  "unexpected dependent resource v1/Pod for CronJob; expected batch/v1 Job",
		},
		{
			name:     "ConfigMap directly under CronJob is rejected",
			resource: cmUnderCron,
			wantErr:  "unexpected dependent resource apps/v1/ConfigMap for CronJob; expected batch/v1 Job",
		},
		{
			name:     "Job under ReplicaSet is rejected",
			resource: jobUnderRS,
			wantErr:  "unexpected dependent resource batch/v1/Job for ReplicaSet; expected v1 Pod",
		},
		{
			name:     "Job under StatefulSet is rejected",
			resource: jobUnderSTS,
			wantErr:  "unexpected dependent resource batch/v1/Job for StatefulSet; expected v1 Pod",
		},
		{
			name:     "Job under DaemonSet is rejected",
			resource: jobUnderDS,
			wantErr:  "unexpected dependent resource batch/v1/Job for DaemonSet; expected v1 Pod",
		},
		{
			name:     "ConfigMap under Job is rejected",
			resource: cmUnderJob,
			wantErr:  "unexpected dependent resource apps/v1/ConfigMap for Job; expected v1 Pod",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RootOwnerCreationTimestamp(context.TODO(), client, tt.resource)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestMatchesPodTemplate_DefaultedPod(t *testing.T) {
	tmpl := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "nginx",
			},
			"annotations": map[string]any{
				"env": "production",
			},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.21",
					"ports": []any{
						map[string]any{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "0.2",
							"memory": "30Mi",
						},
						"requests": map[string]any{
							"cpu":    "0.1",
							"memory": "20Mi",
						},
					},
				},
			},
		},
	}

	// Normal API-server defaulted pod shape (matching validation_test.go:2059)
	defaultedPod := map[string]any{
		"metadata": map[string]any{
			"name":      "nginx-xyz",
			"namespace": "default",
			"labels": map[string]any{
				"app":               "nginx",
				"pod-template-hash": "6d4cf56db6",
			},
			"annotations": map[string]any{
				"env": "production",
			},
		},
		"spec": map[string]any{
			"restartPolicy":                 "Always",
			"dnsPolicy":                     "ClusterFirst",
			"terminationGracePeriodSeconds": int64(30),
			"serviceAccountName":            "default",
			"serviceAccount":                "default",
			"schedulerName":                 "default-scheduler",
			"securityContext":               map[string]any{},
			"enableServiceLinks":            true,
			"priority":                      int64(0),
			"tolerations": []any{
				map[string]any{
					"key":      "node.kubernetes.io/not-ready",
					"operator": "Exists",
					"effect":   "NoExecute",
				},
			},
			"volumes": []any{
				map[string]any{
					"name": "default-token-4q2mj",
					"secret": map[string]any{
						"secretName":  "default-token-4q2mj",
						"defaultMode": int64(420),
					},
				},
			},
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.21",
					"ports": []any{
						map[string]any{
							"containerPort": int64(80),
							"protocol":      "TCP",
						},
					},
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "30Mi",
						},
						"requests": map[string]any{
							"cpu":    "100m",
							"memory": "20Mi",
						},
					},
					"volumeMounts": []any{
						map[string]any{
							"name":      "default-token-4q2mj",
							"readOnly":  true,
							"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
						},
					},
					"terminationMessagePath":   "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy":          "Always",
				},
			},
		},
	}

	// 1. Exact match with API server defaulted fields must succeed
	assert.True(t, matchesPodTemplate(defaultedPod, tmpl))

	// 2. Tampered image fails
	tamperedImagePod := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:malicious",
				},
			},
		},
	}
	assert.False(t, matchesPodTemplate(tamperedImagePod, tmpl))

	// 3. Tampered container resource limit fails
	tamperedResourcePod := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.21",
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "500m",
						},
					},
				},
			},
		},
	}
	assert.False(t, matchesPodTemplate(tamperedResourcePod, tmpl))

	// 4. Missing required template label fails
	missingLabelPod := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "other",
			},
		},
		"spec": defaultedPod["spec"],
	}
	assert.False(t, matchesPodTemplate(missingLabelPod, tmpl))

	// 5. Injected mutating webhook annotations and labels succeed
	mutatedPod := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":               "nginx",
				"pod-template-hash": "6d4cf56db6",
				"istio.io/rev":      "default",
			},
			"annotations": map[string]any{
				"env":                     "production",
				"sidecar.istio.io/status": "injected",
			},
		},
		"spec": defaultedPod["spec"],
	}
	assert.True(t, matchesPodTemplate(mutatedPod, tmpl))

	// 6. Extra un-templated container in Pod fails closed
	extraContainerPod := map[string]any{
		"metadata": defaultedPod["metadata"],
		"spec": map[string]any{
			"containers": []any{
				defaultedPod["spec"].(map[string]any)["containers"].([]any)[0],
				map[string]any{
					"name":  "sidecar-attacker",
					"image": "crypto-miner:latest",
				},
			},
		},
	}
	assert.False(t, matchesPodTemplate(extraContainerPod, tmpl))

	// 7. Extra untrusted volume in Pod fails closed
	extraVolumePod := map[string]any{
		"metadata": defaultedPod["metadata"],
		"spec": map[string]any{
			"containers": defaultedPod["spec"].(map[string]any)["containers"],
			"volumes": []any{
				map[string]any{
					"name": "untrusted-hostpath",
					"hostPath": map[string]any{
						"path": "/etc/shadow",
					},
				},
			},
		},
	}
	assert.False(t, matchesPodTemplate(extraVolumePod, tmpl))
}

func TestGetCachedResource_LiveDirectEvaluationWithoutCache(t *testing.T) {
	ts := metav1.Unix(1000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", ts, "", "", "", false)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Deployment/default/deploy1": &deploy,
		},
	}

	// First read queries live client
	obj1, err := getCachedResource(context.TODO(), client, "apps/v1", "Deployment", "default", "deploy1")
	assert.NoError(t, err)
	assert.NotNil(t, obj1)
	assert.Equal(t, "deploy-uid", string(obj1.GetUID()))

	// Mutate resource in client; subsequent call immediately observes live updated state without cache reset
	updatedTs := metav1.Unix(2000, 0)
	deployUpdated := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid-updated", updatedTs, "", "", "", false)
	client.resources["Deployment/default/deploy1"] = &deployUpdated

	obj2, err := getCachedResource(context.TODO(), client, "apps/v1", "Deployment", "default", "deploy1")
	assert.NoError(t, err)
	assert.Equal(t, "deploy-uid-updated", string(obj2.GetUID()))
}

func TestRootOwnerCreationTimestamp_ReplicaSet_LiveEvaluation_MutatedTemplateWithoutCacheReset_Fails(t *testing.T) {
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs1":     &rs,
			"Deployment/default/deploy1": &deploy,
		},
	}

	// First check with unchanged template succeeds
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, deployTs.Time, got.Time)

	// Mutate ReplicaSet template in client without resetting cache (e.g. attacker modifies template in-place)
	rsMutated := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)
	_ = unstructured.SetNestedSlice(rsMutated.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:tampered"},
	}, "spec", "template", "spec", "containers")
	client.resources["ReplicaSet/default/rs1"] = &rsMutated

	// Immediate next check observes live mutated state and fails without needing ResetOwnerCaches()
	_, err = RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template does not match immutable selector hash")
}

func TestMatchesTemplateFields_DuplicateVolumeMounts_DifferentPaths_Passes(t *testing.T) {
	podMounts := []any{
		map[string]any{"name": "config-vol", "mountPath": "/etc/config/a", "subPath": "a"},
		map[string]any{"name": "config-vol", "mountPath": "/etc/config/b", "subPath": "b"},
	}
	tmplMounts := []any{
		map[string]any{"name": "config-vol", "mountPath": "/etc/config/a", "subPath": "a"},
		map[string]any{"name": "config-vol", "mountPath": "/etc/config/b", "subPath": "b"},
	}

	podSpec := map[string]any{
		"containers": []any{
			map[string]any{"name": "nginx", "volumeMounts": podMounts},
		},
	}
	tmplSpec := map[string]any{
		"containers": []any{
			map[string]any{"name": "nginx", "volumeMounts": tmplMounts},
		},
	}

	assert.True(t, matchesTemplateFields(podSpec, tmplSpec))
}

func TestRootOwnerCreationTimestamp_CronJob_PostPolicyJob_Pod_Passes(t *testing.T) {
	cronJobTs := metav1.Unix(1000, 0)
	jobTs := metav1.Unix(3000, 0) // Scheduled Job created after policy
	podTs := metav1.Unix(3010, 0)

	cronJob := makeTestUnstructured("CronJob", "default", "cj1", "cj-uid", cronJobTs, "", "", "", false)
	cronJob.SetAPIVersion("batch/v1")
	cronJob.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "kube-controller-manager",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		Time:       &cronJobTs,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:jobTemplate":{}}}`)},
	}})

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cj1", "cj-uid", true)
	job.SetAPIVersion("batch/v1")
	job.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "cronjob-controller",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		Time:       &jobTs,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)},
	}})

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CronJob/default/cj1": &cronJob,
			"Job/default/job1":    &job,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, cronJobTs.Time, got.Time, "Pod under scheduled Job should inherit parent CronJob's template timestamp")
}

func TestRootOwnerCreationTimestamp_DaemonSet_NodeAffinityTransformation_Passes(t *testing.T) {
	dsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)

	ds := makeTestUnstructured("DaemonSet", "default", "ds1", "ds-uid", dsTs, "", "", "", false)
	rev := makeTestUnstructured("ControllerRevision", "default", "ds1-rev1", "rev-uid", dsTs, "DaemonSet", "ds1", "ds-uid", true)

	// Historical template with zone node affinity
	tmpl := map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
			"affinity": map[string]any{
				"nodeAffinity": map[string]any{
					"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
						"nodeSelectorTerms": []any{
							map[string]any{
								"matchExpressions": []any{
									map[string]any{
										"key":      "topology.kubernetes.io/zone",
										"operator": "In",
										"values":   []any{"zone-a"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = unstructured.SetNestedMap(rev.Object, tmpl, "data", "template")

	// Generated Pod with controller-injected node affinity for the target node
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "DaemonSet", "ds1", "ds-uid", true)
	pod.SetLabels(map[string]string{
		"app":                      "nginx",
		"controller-revision-hash": "rev1",
	})
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{
			"matchFields": []any{
				map[string]any{
					"key":      "metadata.name",
					"operator": "In",
					"values":   []any{"worker-node-1"},
				},
			},
		},
	}, "spec", "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution", "nodeSelectorTerms")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"DaemonSet/default/ds1":               &ds,
			"ControllerRevision/default/ds1-rev1": &rev,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, dsTs.Time, got.Time)
}

func TestRootOwnerCreationTimestamp_ReplicaSet_HashMismatch_BothTemplatesEdited_Fails(t *testing.T) {
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(1000, 0)
	podTs := metav1.Unix(2000, 0)

	deploy := makeTestUnstructured("Deployment", "default", "deploy1", "deploy-uid", deployTs, "", "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", "rs-uid", rsTs, "Deployment", "deploy1", "deploy-uid", true)

	// An attacker edits both Deployment and ReplicaSet templates in-place to the same prohibited image
	_ = unstructured.SetNestedSlice(deploy.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:prohibited"},
	}, "spec", "template", "spec", "containers")
	_ = unstructured.SetNestedSlice(rs.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:prohibited"},
	}, "spec", "template", "spec", "containers")

	// Selector hash remains the original immutable hash from before the edit
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "ReplicaSet", "rs1", "rs-uid", true)
	_ = unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:prohibited"},
	}, "spec", "containers")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Deployment/default/deploy1": &deploy,
			"ReplicaSet/default/rs1":     &rs,
		},
	}

	// Must fail closed because the edited template does not match the immutable selector hash
	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template does not match immutable selector hash")
}

func TestMatchesPodTemplate_CompliantHistoricalTemplate_MutatingWebhookAddedHostNetwork_Fails(t *testing.T) {
	tmpl := map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}

	pod := map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
		"spec": map[string]any{
			"hostNetwork": true,
			"containers": []any{
				map[string]any{"name": "nginx", "image": "nginx"},
			},
		},
	}

	assert.False(t, matchesPodTemplate(pod, tmpl), "Pod with un-templated hostNetwork: true injected by mutating webhook must fail template match")
}

func TestRootOwnerCreationTimestamp_CronJob_ScheduleOnlyModification_Passes(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	postPolicyScheduleTs := metav1.Unix(2500, 0)
	jobTs := metav1.Unix(3000, 0)
	podTs := metav1.Unix(3010, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	// CronJob managedFields updated post-policy for schedule or suspend, but jobTemplate is unchanged
	cron.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "kubectl",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &cronTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:jobTemplate":{}}}`)},
		},
		{
			Manager:    "kubectl-edit",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &postPolicyScheduleTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:schedule":{}}}`)},
		},
	})

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CronJob/default/cron1": &cron,
			"Job/default/job1":      &job,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, cronTs.Time, got.Time, "Schedule-only CronJob update must not bump root owner timestamp past policy creation")
}

func TestRootOwnerCreationTimestamp_Job_ParallelismOnlyModification_Passes(t *testing.T) {
	ResetOwnerCaches()
	jobTs := metav1.Unix(1000, 0)
	postPolicyScaleTs := metav1.Unix(2500, 0)
	podTs := metav1.Unix(3000, 0)

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "", "", "", false)
	// Job managedFields updated post-policy for parallelism, but template is unchanged
	job.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "kubectl",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &jobTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)},
		},
		{
			Manager:    "kubectl-scale",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &postPolicyScaleTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:parallelism":{}}}`)},
		},
	})

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Job/default/job1": &job,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, jobTs.Time, got.Time, "Parallelism-only Job update must not bump root owner timestamp past policy creation")
}

func TestRootOwnerCreationTimestamp_Job_ExtraRunAsUserField_Fails(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	jobTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)

	// Job adds extra runAsUser field to container securityContext that is absent in CronJob template
	jContainers, _, _ := unstructured.NestedSlice(job.Object, "spec", "template", "spec", "containers")
	_ = unstructured.SetNestedField(jContainers[0].(map[string]any), int64(0), "securityContext", "runAsUser")
	_ = unstructured.SetNestedSlice(job.Object, jContainers, "spec", "template", "spec", "containers")

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)
	pContainers, _, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
	_ = unstructured.SetNestedField(pContainers[0].(map[string]any), int64(0), "securityContext", "runAsUser")
	_ = unstructured.SetNestedSlice(pod.Object, pContainers, "spec", "containers")

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Pod/default/pod1":      &pod,
			"Job/default/job1":      &job,
			"CronJob/default/cron1": &cron,
		},
	}

	_, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match CronJob cron1 jobTemplate")
}

func TestRootOwnerCreationTimestamp_CronJob_MixedEntryScheduleAndTemplate_AdvancesTimestamp(t *testing.T) {
	ResetOwnerCaches()
	cronTs := metav1.Unix(1000, 0)
	postPolicyScheduleTs := metav1.Unix(2500, 0)
	jobTs := metav1.Unix(3000, 0)
	podTs := metav1.Unix(3010, 0)

	cron := makeTestUnstructured("CronJob", "default", "cron1", "cron-uid", cronTs, "", "", "", false)
	// CronJob managedFields entry owns both schedule and jobTemplate with post-policy timestamp
	cron.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "kubectl",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &cronTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:jobTemplate":{}}}`)},
		},
		{
			Manager:    "kubectl-edit",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &postPolicyScheduleTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:schedule":{},"f:jobTemplate":{}}}`)},
		},
	})

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "CronJob", "cron1", "cron-uid", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"CronJob/default/cron1": &cron,
			"Job/default/job1":      &job,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, postPolicyScheduleTs.Time, got.Time, "Mixed managedFields entry owning both schedule and jobTemplate must advance timestamp past policy creation (fail closed)")
}

func TestRootOwnerCreationTimestamp_Job_MixedEntryParallelismAndTemplate_AdvancesTimestamp(t *testing.T) {
	ResetOwnerCaches()
	jobTs := metav1.Unix(1000, 0)
	postPolicyScaleTs := metav1.Unix(2500, 0)
	podTs := metav1.Unix(3000, 0)

	job := makeTestUnstructured("Job", "default", "job1", "job-uid", jobTs, "", "", "", false)
	// Job managedFields entry owns both parallelism and template with post-policy timestamp
	job.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "kubectl",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &jobTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)},
		},
		{
			Manager:    "kubectl-scale",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			Time:       &postPolicyScaleTs,
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:parallelism":{},"f:template":{}}}`)},
		},
	})

	pod := makeTestUnstructured("Pod", "default", "pod1", "pod-uid", podTs, "Job", "job1", "job-uid", true)

	client := &mockResourceClient{
		resources: map[string]*unstructured.Unstructured{
			"Job/default/job1": &job,
		},
	}

	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, postPolicyScaleTs.Time, got.Time, "Mixed managedFields entry owning both parallelism and template must advance timestamp past policy creation (fail closed)")
}

func TestMatchesVolumes_FakeTokenHostPathVolume_Fails(t *testing.T) {
	podSpec := map[string]any{
		"volumes": []any{
			map[string]any{
				"name": "kube-api-access-fake",
				"hostPath": map[string]any{
					"path": "/",
				},
			},
		},
	}
	tmplSpec := map[string]any{
		"volumes": []any{},
	}
	assert.False(t, matchesVolumes(podSpec, tmplSpec), "Untemplated hostPath volume with token-like name must fail")
}

func TestMatchesVolumes_RealProjectedSATokenVolume_Passes(t *testing.T) {
	podSpec := map[string]any{
		"volumes": []any{
			map[string]any{
				"name": "kube-api-access-abcde",
				"projected": map[string]any{
					"sources": []any{
						map[string]any{
							"serviceAccountToken": map[string]any{
								"path": "token",
							},
						},
					},
				},
			},
		},
	}
	tmplSpec := map[string]any{
		"volumes": []any{},
	}
	assert.True(t, matchesVolumes(podSpec, tmplSpec), "Untemplated projected serviceAccountToken volume must pass")
}
