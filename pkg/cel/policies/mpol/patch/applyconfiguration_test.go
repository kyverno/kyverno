package patch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	k8spatch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/environment"
	"k8s.io/client-go/openapi/openapitest"
	"k8s.io/kube-openapi/pkg/spec3"
)

func TestApplyConfiguration_AllowsAtomicLists(t *testing.T) {
	expression := `Object{
		spec: Object.spec{
			initContainers: [
				Object.spec.initContainers{
					name: "mesh-proxy",
					image: "mesh/proxy:v1.0.0",
					args: ["proxy", "sidecar"]
				}
			]
		}
	}`

	patcher := newApplyConfigPatcher(t, expression)

	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	object := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
			InitContainers: []corev1.Container{
				{Name: "existing-init", Image: "busybox"},
			},
		},
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	typeConverter := getTypeConverterForGVK(t, gvk)
	req := newPatchRequest(object, gvk, gvr, scheme, typeConverter)
	patched, err := patcher.Patch(context.Background(), req, celconfig.RuntimeCELCostBudget)
	if err != nil {
		t.Fatalf("unexpected patch error: %v", err)
	}

	patchedPod, ok := patched.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("unexpected patched object type %T", patched)
	}

	initContainers, found, err := unstructured.NestedSlice(patchedPod.Object, "spec", "initContainers")
	if err != nil {
		t.Fatalf("failed to read initContainers: %v", err)
	}
	if !found {
		t.Fatal("expected initContainers to be present")
	}

	if len(initContainers) != 2 {
		t.Fatalf("expected 2 initContainers, got %d", len(initContainers))
	}

	foundMeshProxy := false
	for _, c := range initContainers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if containerMap["name"] == "mesh-proxy" {
			foundMeshProxy = true
			args, ok := containerMap["args"].([]any)
			if !ok || len(args) != 2 || args[0] != "proxy" || args[1] != "sidecar" {
				t.Fatalf("unexpected args for mesh-proxy: %#v", containerMap["args"])
			}
		}
	}

	if !foundMeshProxy {
		t.Fatal("expected mesh-proxy init container to be present")
	}
}

func TestApplyConfiguration_StillRejectsAtomicStructs(t *testing.T) {
	expression := `Object{
		spec: Object.spec{
			selector: Object.spec.selector{
				matchLabels: {"app": "test"}
			}
		}
	}`

	patcher := newApplyConfigPatcher(t, expression)

	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	object := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
		},
	}

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	typeConverter := getTypeConverterForGVK(t, gvk)
	req := newPatchRequest(object, gvk, gvr, scheme, typeConverter)
	_, err := patcher.Patch(context.Background(), req, celconfig.RuntimeCELCostBudget)
	if err == nil {
		t.Fatal("expected error for atomic struct mutation")
	}
	if !strings.Contains(err.Error(), ".spec.selector") {
		t.Fatalf("expected selector path in error, got: %v", err)
	}
}

func newApplyConfigPatcher(t *testing.T, expression string) k8spatch.Patcher {
	t.Helper()
	compiler, err := plugincel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))
	if err != nil {
		t.Fatal(err)
	}
	mutationEvaluator := compiler.CompileMutatingEvaluator(
		&k8spatch.ApplyConfigurationCondition{Expression: expression},
		plugincel.OptionalVariableDeclarations{HasPatchTypes: true},
		environment.StoredExpressions,
	)
	return NewApplyConfigurationPatcher(mutationEvaluator)
}

func getTypeConverterForGVK(t *testing.T, gvk schema.GroupVersionKind) managedfields.TypeConverter {
	t.Helper()
	client := openapitest.NewEmbeddedFileClient()
	paths, err := client.Paths()
	if err != nil {
		t.Fatalf("failed to fetch openapi paths: %v", err)
	}
	path := "apis/" + gvk.Group + "/" + gvk.Version
	if gvk.Group == "" {
		path = "api/" + gvk.Version
	}
	entry, ok := paths[path]
	if !ok {
		t.Fatalf("missing openapi path %q for gvk %v", path, gvk)
	}

	schBytes, err := entry.Schema(runtime.ContentTypeJSON)
	if err != nil {
		t.Fatalf("failed to get openapi schema for %v: %v", gvk, err)
	}
	var sch spec3.OpenAPI
	if err := json.Unmarshal(schBytes, &sch); err != nil {
		t.Fatalf("failed to unmarshal openapi schema for %v: %v", gvk, err)
	}
	tc, err := managedfields.NewTypeConverter(sch.Components.Schemas, false)
	if err != nil {
		t.Fatalf("failed to create type converter for %v: %v", gvk, err)
	}
	return tc
}

func newPatchRequest(
	object runtime.Object,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	scheme *runtime.Scheme,
	typeConverter managedfields.TypeConverter,
) k8spatch.Request {
	attrs := admission.NewAttributesRecord(
		object,
		nil,
		gvk,
		"default",
		"demo",
		gvr,
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		nil,
	)
	return k8spatch.Request{
		MatchedResource: gvr,
		VersionedAttributes: &admission.VersionedAttributes{
			Attributes:      attrs,
			VersionedKind:   gvk,
			VersionedObject: object,
		},
		ObjectInterfaces: admission.NewObjectInterfacesFromScheme(scheme),
		TypeConverter:    typeConverter,
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
