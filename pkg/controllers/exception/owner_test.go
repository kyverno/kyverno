package exception

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newUnstructuredPod(name, namespace string, ownerRefs []metav1.OwnerReference) unstructured.Unstructured {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
	}
	data, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	return unstructured.Unstructured{Object: data}
}

func boolPtr(b bool) *bool {
	return &b
}

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return scheme
}

func newFakeClient(objects ...runtime.Object) dclient.Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                        "PodList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:             "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:             "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:            "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:              "DaemonSetList",
	}
	client, err := dclient.NewFakeClient(newTestScheme(), gvrToListKind, objects...)
	if err != nil {
		panic(err)
	}
	return client
}

func Test_resolveRootOwner_NoOwner(t *testing.T) {
	pod := newUnstructuredPod("my-pod", "default", nil)
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	assert.Equal(t, "Pod", owner.kind)
	assert.Equal(t, "my-pod", owner.name)
	assert.Equal(t, "default", owner.namespace)
}

func Test_resolveRootOwner_DirectDeploymentOwner(t *testing.T) {
	pod := newUnstructuredPod("my-pod", "default", []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "my-deploy",
			Controller: boolPtr(true),
		},
	})
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	assert.Equal(t, "Deployment", owner.kind)
	assert.Equal(t, "my-deploy", owner.name)
	assert.Equal(t, "default", owner.namespace)
}

func Test_resolveRootOwner_PodToReplicaSet(t *testing.T) {
	// ReplicaSet owned by a Deployment
	rs := &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy-abc123",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "my-deploy",
					Controller: boolPtr(true),
				},
			},
		},
	}

	pod := newUnstructuredPod("my-deploy-abc123-xyz", "default", []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Name:       "my-deploy-abc123",
			Controller: boolPtr(true),
		},
	})

	client := newFakeClient(rs)

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	// ReplicaSet is in topLevelControllers, so it stops there
	assert.Equal(t, "ReplicaSet", owner.kind)
	assert.Equal(t, "my-deploy-abc123", owner.name)
}

func Test_resolveRootOwner_StatefulSet(t *testing.T) {
	pod := newUnstructuredPod("my-sts-0", "default", []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
			Name:       "my-sts",
			Controller: boolPtr(true),
		},
	})
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	assert.Equal(t, "StatefulSet", owner.kind)
	assert.Equal(t, "my-sts", owner.name)
}

func Test_resolveRootOwner_DaemonSet(t *testing.T) {
	pod := newUnstructuredPod("my-ds-xyz", "kube-system", []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
			Name:       "my-ds",
			Controller: boolPtr(true),
		},
	})
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	assert.Equal(t, "DaemonSet", owner.kind)
	assert.Equal(t, "my-ds", owner.name)
	assert.Equal(t, "kube-system", owner.namespace)
}

func Test_resolveRootOwner_TopLevelResource(t *testing.T) {
	deploy := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "my-deploy",
				"namespace": "default",
			},
		},
	}
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, deploy)
	assert.NoError(t, err)
	assert.Equal(t, "Deployment", owner.kind)
	assert.Equal(t, "my-deploy", owner.name)
}

func Test_resolveRootOwner_NonControllerOwnerRef(t *testing.T) {
	pod := newUnstructuredPod("my-pod", "default", []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       "node-1",
			Controller: boolPtr(false),
		},
	})
	client := newFakeClient()

	owner, err := resolveRootOwner(context.Background(), client, pod)
	assert.NoError(t, err)
	assert.Equal(t, "Pod", owner.kind)
	assert.Equal(t, "my-pod", owner.name)
}
