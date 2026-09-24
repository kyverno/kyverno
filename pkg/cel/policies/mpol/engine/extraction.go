package engine

import (
	"context"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen/extract"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

// evaluateExtracted implements ExtractionMode for MutatingPolicy: instead of
// evaluating CompiledPolicy against the real admitted object (a custom
// workload CRD, whose shape CompiledPolicy's Pod-targeted mutation
// expression knows nothing about), it extracts every pod-template-shaped
// subtree, synthesizes a Pod from each, mutates the synthesized Pod with the
// same unmodified CompiledPolicy, and splices whatever changed back into the
// corresponding subtree of a copy of the real object.
//
// Mutation only ever applies to the object being admitted, unlike vpol's
// read-only validation counterpart (see vpol/engine.evaluateExtracted) which
// also supports evaluating oldObject alone for a DELETE - a request with no
// new object (e.g. a real DELETE) has nothing to mutate, so it is skipped
// (nil, "no match") rather than synthesized against oldObject.
func (e *engineImpl) evaluateExtracted(ctx context.Context, mpol Policy, attr admission.Attributes, request admissionv1.AdmissionRequest, namespace *corev1.Namespace, target bool) *compiler.EvaluationResult {
	newObj, ok := attr.GetObject().(*unstructured.Unstructured)
	if !ok || newObj == nil || len(newObj.Object) == 0 {
		return nil
	}
	templates := extract.ExtractPodTemplates(newObj.Object)
	if len(templates) == 0 {
		return &compiler.EvaluationResult{Error: fmt.Errorf("extraction mode: no pod template found in %s/%s", newObj.GetAPIVersion(), newObj.GetKind())}
	}
	oldByPath := templatesByPath(attr.GetOldObject())

	patchedRoot := newObj.DeepCopy()
	var (
		auditAnnotations map[string]string
		allExceptions    []*policiesv1beta1.PolicyException
		matched          bool
	)
	for _, tpl := range templates {
		var oldTpl *extract.Extracted
		if o, ok := oldByPath[tpl.Path]; ok {
			oldTpl = &o
		}
		synthAttr := extract.SynthesizePodAttributes(&tpl, oldTpl, attr)
		synthRequest := extract.SynthesizePodAdmissionRequest(&request, synthAttr)

		var result *compiler.EvaluationResult
		if target {
			result = mpol.CompiledPolicy.EvaluateTarget(ctx, synthAttr, namespace, *synthRequest, e.typeConverter, nil, e.contextProvider)
		} else {
			result = mpol.CompiledPolicy.Evaluate(ctx, synthAttr, namespace, *synthRequest, e.typeConverter, nil, e.contextProvider)
		}
		if result == nil {
			continue
		}
		if result.Error != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, result.Error)}
		}
		if len(result.Exceptions) > 0 {
			allExceptions = append(allExceptions, result.Exceptions...)
			continue
		}
		matched = true
		if result.PatchedResource != nil {
			merged, err := spliceTemplate(tpl.Template, synthAttr.GetObject().(*unstructured.Unstructured), result.PatchedResource)
			if err != nil {
				return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
			}
			if err := extract.SetAtPath(patchedRoot.Object, tpl.Path, merged); err != nil {
				return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
			}
		}
		for k, v := range result.AuditAnnotations {
			if auditAnnotations == nil {
				auditAnnotations = map[string]string{}
			}
			auditAnnotations[k] = v
		}
	}
	if !matched {
		if len(allExceptions) > 0 {
			return &compiler.EvaluationResult{Exceptions: allExceptions}
		}
		// no template matched matchConditions/targetMatchConditions - the
		// same "skip" outcome handlePolicy already gives a nil result from
		// the non-extraction path.
		return nil
	}
	return &compiler.EvaluationResult{PatchedResource: patchedRoot, AuditAnnotations: auditAnnotations}
}

func templatesByPath(obj runtime.Object) map[string]extract.Extracted {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil || len(u.Object) == 0 {
		return nil
	}
	templates := extract.ExtractPodTemplates(u.Object)
	if len(templates) == 0 {
		return nil
	}
	out := make(map[string]extract.Extracted, len(templates))
	for _, t := range templates {
		out[t.Path] = t
	}
	return out
}

// spliceTemplate rebases whatever the policy actually changed between the
// synthesized input Pod (before) and the mutated output Pod (after) onto
// original - the pristine pod-template subtree as it existed in the real
// object, before SynthesizePodAttributes injected any placeholder
// metadata.name/namespace. A JSON merge patch (RFC 7396) diffed between
// before and after contains only genuine mutations - a patcher that never
// touches metadata.name produces no merge-patch entry for it - so applying
// that same merge patch to original reproduces the mutation without also
// introducing the synthesis placeholders as if they were real changes.
func spliceTemplate(original map[string]any, before, after *unstructured.Unstructured) (map[string]any, error) {
	beforeJSON, err := json.Marshal(podTemplateFields(before))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod template before mutation: %w", err)
	}
	afterJSON, err := json.Marshal(podTemplateFields(after))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod template after mutation: %w", err)
	}
	mergePatch, err := jsonpatch.CreateMergePatch(beforeJSON, afterJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to diff mutated pod template: %w", err)
	}
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original pod template: %w", err)
	}
	mergedJSON, err := jsonpatch.MergePatch(originalJSON, mergePatch)
	if err != nil {
		return nil, fmt.Errorf("failed to apply mutation to pod template: %w", err)
	}
	var merged map[string]any
	if err := json.Unmarshal(mergedJSON, &merged); err != nil {
		return nil, fmt.Errorf("failed to decode merged pod template: %w", err)
	}
	return merged, nil
}

// podTemplateFields returns pod's metadata/spec fields only, matching the
// shape of a pod-template subtree (see extract.isPodTemplateSpec) - pod also
// carries apiVersion/kind, which are synthesis artifacts (see
// extract.buildPod) that must never leak into the spliced-back template.
func podTemplateFields(pod *unstructured.Unstructured) map[string]any {
	out := map[string]any{}
	if md, ok := pod.Object["metadata"]; ok {
		out["metadata"] = md
	}
	if spec, ok := pod.Object["spec"]; ok {
		out["spec"] = spec
	}
	return out
}
