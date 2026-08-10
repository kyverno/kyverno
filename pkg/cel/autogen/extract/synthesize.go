package extract

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

var (
	podGVK = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	podGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
)

// SynthesizePodAttributes builds a v1/Pod-shaped admission.Attributes from an
// extracted pod template, reusing the real parent admission.Attributes for
// everything that isn't part of the template itself (operation, dry-run,
// user info, subresource). oldTpl may be nil (e.g. on a CREATE request, or
// when the old and new parent objects don't have a matching number of
// templates).
func SynthesizePodAttributes(tpl Extracted, oldTpl *Extracted, parent admission.Attributes) admission.Attributes {
	newPod := buildPod(&tpl, parent)
	oldPod := buildPod(oldTpl, parent)
	return admission.NewAttributesRecord(
		newPod,
		oldPod,
		podGVK,
		newPod.GetNamespace(),
		newPod.GetName(),
		podGVR,
		parent.GetSubresource(),
		parent.GetOperation(),
		parent.GetOperationOptions(),
		parent.IsDryRun(),
		parent.GetUserInfo(),
	)
}

func buildPod(tpl *Extracted, parent admission.Attributes) *unstructured.Unstructured {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
	}}
	if tpl == nil {
		return pod
	}
	var metadata map[string]any
	if m, ok := tpl.Template["metadata"].(map[string]any); ok {
		metadata = runtime.DeepCopyJSON(m)
	} else {
		metadata = map[string]any{}
	}
	if name, _ := metadata["name"].(string); name == "" {
		// Best-effort placeholder: the real name (if any) is derived from
		// the parent's naming convention, which is CRD-specific and out of
		// scope for this pass - see the deferred multi-template naming work.
		metadata["name"] = parent.GetName()
	}
	if ns, _ := metadata["namespace"].(string); ns == "" {
		metadata["namespace"] = parent.GetNamespace()
	}
	pod.Object["metadata"] = metadata
	if spec, ok := tpl.Template["spec"].(map[string]any); ok {
		pod.Object["spec"] = runtime.DeepCopyJSON(spec)
	}
	return pod
}
