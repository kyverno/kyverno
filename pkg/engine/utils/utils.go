package utils

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	jsonpatch "github.com/evanphx/json-patch/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
)

func IsDeleteRequest(ctx engineapi.PolicyContext) bool {
	if ctx == nil {
		return false
	}

	if op := ctx.Operation(); string(op) != "" {
		return op == kyvernov1.Delete
	}

	// if the NewResource is empty, the request is a DELETE
	newResource := ctx.NewResource()
	return IsEmptyUnstructured(&newResource)
}

func IsEmptyUnstructured(u *unstructured.Unstructured) bool {
	if u == nil {
		return true
	}
	if u.Object == nil {
		return true
	}
	return false
}

// ApplyPatches patches given resource with given patches and returns patched document
// return original resource if any error occurs
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	if len(patches) == 0 {
		return resource, nil
	}
	joinedPatches := jsonutils.JoinPatches(patches...)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		logging.V(4).Info("failed to decode JSON patch", "patch", patch)
		return resource, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		logging.V(4).Info("failed to apply JSON patch", "patch", patch)
		return resource, err
	}

	logging.V(4).Info("applied JSON patch", "patch", patch)
	return patchedDocument, err
}

// ApplyPatchNew patches given resource with given joined patches
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	jsonpatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return resource, err
	}

	return patchedResource, err
}

func TransformConditions(original apiextensions.JSON) (interface{}, error) {
	if original == nil {
		return kyvernov1.AnyAllConditions{}, nil
	}

	switch typedValue := original.(type) {
	case *kyvernov1.AnyAllConditions:
		if typedValue == nil {
			return kyvernov1.AnyAllConditions{}, nil
		}
		return *typedValue.DeepCopy(), nil
	case kyvernov1.AnyAllConditions:
		return *typedValue.DeepCopy(), nil
	case []kyvernov1.Condition: // backwards compatibility
		var copies []kyvernov1.Condition
		for _, condition := range typedValue {
			copies = append(copies, *condition.DeepCopy())
		}
		return copies, nil
	}
	return nil, fmt.Errorf("invalid preconditions")
}

func IsSameRuleResponse(r1 *engineapi.RuleResponse, r2 *engineapi.RuleResponse) bool {
	if r1.Name() != r2.Name() ||
		r1.RuleType() != r2.RuleType() ||
		r1.Message() != r2.Message() ||
		r1.Status() != r2.Status() {
		return false
	}

	return true
}

func IsUpdateRequest(ctx engineapi.PolicyContext) bool {
	// is the OldObject and NewObject are available, the request is an UPDATE
	return (ctx.OldResource().Object != nil && ctx.NewResource().Object != nil) || ctx.Operation() == kyvernov1.Update
}

func IsCreateRequest(ctx engineapi.PolicyContext) bool {
	if ctx == nil {
		return false
	}
	if ctx.Operation() == kyvernov1.Create {
		return true
	}
	if ctx.Operation() == kyvernov1.Update || ctx.Operation() == kyvernov1.Delete || ctx.Operation() == kyvernov1.Connect {
		return false
	}
	newResource := ctx.NewResource()
	return ctx.OldResource().Object == nil && !IsEmptyUnstructured(&newResource)
}

// ResetOwnerCaches clears owner caches (no-op now that provenance checks read live cluster state directly).
func ResetOwnerCaches() {}

func getCachedResource(
	ctx context.Context,
	client engineapi.Client,
	apiVersion, kind, namespace, name string,
) (*unstructured.Unstructured, error) {
	if client == nil {
		return nil, fmt.Errorf("nil client")
	}
	// Provenance checks must read live state; a cached owner can mask a template edit.
	return client.GetResource(ctx, apiVersion, kind, namespace, name)
}

// IsControllerAdmissionUser checks if the admission user represents a trusted Kubernetes controller
// responsible for managing workloads (e.g. kube-controller-manager or standard kube-system controller service accounts).
// Requests with an empty username or non-controller identities fail-closed to prevent
// forging ownerReferences to bypass validation.
func IsControllerAdmissionUser(userInfo authenticationv1.UserInfo) bool {
	if userInfo.Username == "" {
		return false
	}
	switch userInfo.Username {
	case "system:kube-controller-manager",
		"system:serviceaccount:kube-system:replicaset-controller",
		"system:serviceaccount:kube-system:daemon-set-controller",
		"system:serviceaccount:kube-system:statefulset-controller",
		"system:serviceaccount:kube-system:job-controller",
		"system:serviceaccount:kube-system:cronjob-controller":
		return true
	default:
		return false
	}
}

// ShouldSkipCreatePreExistingViolation checks whether a CREATE admission request for a managed
// resource should be skipped as a pre-existing violation because all its controllers in the
// ownership chain were created before the policy.
//
// Security & Provenance:
//  1. Caller identity is checked: if AdmissionUserInfo is present, it must belong to a built-in
//     controller-manager identity (e.g. system:kube-controller-manager or standard kube-system controller service accounts). Non-controller callers attempting to forge ownerReferences are rejected.
//  2. Owner UID is verified against live cluster state to prevent dangling or spoofed UIDs.
//  3. Every controller in the ownership chain must predate the policy. If a Deployment was updated
//     after policy creation, its newly created ReplicaSet will have a newer timestamp and the
//     violation will be enforced.
//  4. Results are cached in a thread-safe bounded TTL cache only after live verification.
func ShouldSkipCreatePreExistingViolation(
	ctx context.Context,
	client engineapi.Client,
	policyContext engineapi.PolicyContext,
) bool {
	if client == nil || policyContext == nil || policyContext.Policy() == nil {
		return false
	}
	if !policyContext.AdmissionOperation() || !IsCreateRequest(policyContext) {
		return false
	}
	oldResource := policyContext.OldResource()
	if !IsEmptyUnstructured(&oldResource) {
		return false
	}

	userInfo := policyContext.AdmissionInfo().AdmissionUserInfo
	if !IsControllerAdmissionUser(userInfo) {
		return false
	}

	policyTimestamp := policyContext.Policy().GetCreationTimestamp()
	if policyTimestamp.IsZero() {
		return false
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	rootTimestamp, err := RootOwnerCreationTimestamp(lookupCtx, client, policyContext.NewResource())
	cancel()
	if err != nil || rootTimestamp == nil || rootTimestamp.IsZero() {
		return false
	}

	return rootTimestamp.Time.Before(policyTimestamp.Time)
}

// RootOwnerCreationTimestamp traverses the controller ownerReferences chain and returns the
// newest creation timestamp across all controllers and template revisions in the chain.
//
// Security & Template Versioning:
//  1. Live owner validation: The immediate controller owner is ALWAYS fetched and verified against
//     live cluster state (existence, kind, namespace, and exact UID). A forged or dangling
//     ownerReference is rejected immediately.
//  2. Full GVK validation: Controller owners are strictly validated against their expected built-in
//     API versions (apps/v1 for ReplicaSet, StatefulSet, DaemonSet, Deployment; batch/v1 for Job, CronJob)
//     to prevent custom resources with matching kind names from bypassing validation.
//  3. Template immutability / versioning:
//     - For StatefulSet and DaemonSet, in-place pod-template updates do not produce new owner objects.
//     The ControllerRevision named by the resource's "controller-revision-hash" label is resolved
//     in apps/v1 and its creation timestamp is incorporated. If template-revision evidence is missing
//     or unresolvable, validation fails closed.
//     - For ReplicaSets, an owning Deployment in apps/v1 must exist to establish provenance,
//     and the ReplicaSet's pod template must match the Deployment's pod template (ignoring the
//     pod-template-hash label). If the ReplicaSet template was edited in-place or does not match
//     the Deployment, validation fails closed.
//     - For Jobs, pod templates are immutable per object in Kubernetes.
func RootOwnerCreationTimestamp(
	ctx context.Context,
	client engineapi.Client,
	resource unstructured.Unstructured,
) (*metav1.Time, error) {
	if client == nil {
		return nil, nil
	}
	owners := resource.GetOwnerReferences()
	var controllerOwner *metav1.OwnerReference
	for i := range owners {
		if owners[i].Controller != nil && *owners[i].Controller {
			controllerOwner = &owners[i]
			break
		}
	}
	if controllerOwner == nil {
		return nil, nil
	}

	// Always resolve the immediate controller owner against live cluster state first.
	// This ensures that:
	// 1. The referenced owner actually exists in the live cluster.
	// 2. The UID matches the live object's UID, rejecting forged, deleted, or dangling UIDs.
	ns := resource.GetNamespace()
	obj, err := getCachedResource(ctx, client, controllerOwner.APIVersion, controllerOwner.Kind, ns, controllerOwner.Name)
	if err != nil {
		return nil, err
	}
	if obj == nil || obj.Object == nil {
		return nil, fmt.Errorf("GetResource returned nil object for %s/%s", controllerOwner.Kind, controllerOwner.Name)
	}
	if controllerOwner.UID == "" || obj.GetUID() != controllerOwner.UID {
		return nil, fmt.Errorf("invalid or mismatched controller owner UID for %s/%s", controllerOwner.Kind, controllerOwner.Name)
	}

	labels := resource.GetLabels()
	revisionHash := labels["controller-revision-hash"]

	immediateTs := obj.GetCreationTimestamp()
	effectiveTs := immediateTs

	switch controllerOwner.Kind {
	case "StatefulSet", "DaemonSet":
		if resource.GetKind() != "Pod" || (resource.GetAPIVersion() != "v1" && resource.GetAPIVersion() != "" && resource.GetAPIVersion() != "core/v1") {
			return nil, fmt.Errorf("unexpected dependent resource %s/%s for %s; expected v1 Pod", resource.GetAPIVersion(), resource.GetKind(), controllerOwner.Kind)
		}
		if controllerOwner.APIVersion != "apps/v1" {
			return nil, fmt.Errorf("unsupported %s APIVersion %s; expected apps/v1", controllerOwner.Kind, controllerOwner.APIVersion)
		}
		if revisionHash == "" {
			return nil, fmt.Errorf("missing controller-revision-hash label on %s managed by %s/%s", resource.GetKind(), controllerOwner.Kind, controllerOwner.Name)
		}
		revisionName := revisionHash
		if !strings.HasPrefix(revisionHash, controllerOwner.Name+"-") {
			revisionName = fmt.Sprintf("%s-%s", controllerOwner.Name, revisionHash)
		}
		revisionObj, revErr := getCachedResource(ctx, client, "apps/v1", "ControllerRevision", ns, revisionName)
		if revErr != nil || revisionObj == nil || revisionObj.Object == nil {
			return nil, fmt.Errorf("failed to fetch ControllerRevision %s for %s/%s: %w", revisionName, controllerOwner.Kind, controllerOwner.Name, revErr)
		}
		var revOwnedByController bool
		for _, o := range revisionObj.GetOwnerReferences() {
			if o.APIVersion == "apps/v1" && o.Kind == controllerOwner.Kind && o.Name == controllerOwner.Name && o.UID != "" && o.UID == obj.GetUID() {
				if o.Controller != nil && *o.Controller {
					revOwnedByController = true
					break
				}
			}
		}
		if !revOwnedByController {
			return nil, fmt.Errorf("ControllerRevision %s is not controlled by live %s/%s (UID %s)", revisionObj.GetName(), controllerOwner.Kind, controllerOwner.Name, obj.GetUID())
		}
		if !matchesRevisionTemplate(resource.Object, revisionObj.Object) {
			return nil, fmt.Errorf("Pod %s does not match ControllerRevision %s template", resource.GetName(), revisionObj.GetName())
		}
		revTs := revisionObj.GetCreationTimestamp()
		if revTs.IsZero() {
			return nil, fmt.Errorf("ControllerRevision %s has zero creationTimestamp", revisionName)
		}
		if revTs.Time.After(effectiveTs.Time) {
			effectiveTs = revTs
		}
		parentTs, err := newestOwnerCreationTimestamp(ctx, client, *obj, 1)
		if err != nil {
			return nil, err
		}
		if parentTs != nil && !parentTs.IsZero() && parentTs.Time.After(effectiveTs.Time) {
			effectiveTs = *parentTs
		}

	case "ReplicaSet":
		if resource.GetKind() != "Pod" || (resource.GetAPIVersion() != "v1" && resource.GetAPIVersion() != "" && resource.GetAPIVersion() != "core/v1") {
			return nil, fmt.Errorf("unexpected dependent resource %s/%s for ReplicaSet; expected v1 Pod", resource.GetAPIVersion(), resource.GetKind())
		}
		if controllerOwner.APIVersion != "apps/v1" {
			return nil, fmt.Errorf("unsupported ReplicaSet APIVersion %s; expected apps/v1", controllerOwner.APIVersion)
		}
		var deployRef *metav1.OwnerReference
		for _, o := range obj.GetOwnerReferences() {
			if o.Controller != nil && *o.Controller && o.Kind == "Deployment" && o.APIVersion == "apps/v1" {
				ref := o
				deployRef = &ref
				break
			}
		}
		if deployRef == nil {
			return nil, fmt.Errorf("standalone ReplicaSet %s has no parent Deployment controller in apps/v1; cannot verify template immutability", controllerOwner.Name)
		}
		deployObj, err := getCachedResource(ctx, client, "apps/v1", "Deployment", ns, deployRef.Name)
		if err != nil || deployObj == nil || deployObj.Object == nil {
			return nil, fmt.Errorf("failed to fetch parent Deployment %s for ReplicaSet %s: %w", deployRef.Name, controllerOwner.Name, err)
		}
		if deployRef.UID == "" || deployObj.GetUID() != deployRef.UID {
			return nil, fmt.Errorf("mismatched parent Deployment UID for %s", deployRef.Name)
		}
		rsHash := obj.GetLabels()["pod-template-hash"]
		selectorHash, okSelector, _ := unstructured.NestedString(obj.Object, "spec", "selector", "matchLabels", "pod-template-hash")
		if rsHash == "" || !okSelector || selectorHash != rsHash {
			return nil, fmt.Errorf("ReplicaSet %s missing or mismatched pod-template-hash selector label", controllerOwner.Name)
		}
		rsTmpl, okRSTmpl, _ := unstructured.NestedMap(obj.Object, "spec", "template")
		if !okRSTmpl || rsTmpl == nil {
			return nil, fmt.Errorf("ReplicaSet %s missing pod template", controllerOwner.Name)
		}
		var pts corev1.PodTemplateSpec
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rsTmpl, &pts); err != nil {
			return nil, fmt.Errorf("failed to convert ReplicaSet %s pod template: %w", controllerOwner.Name, err)
		}
		delete(pts.Labels, "pod-template-hash")

		var matchedHash bool
		var collisionCountMax int32 = -1
		if cCount, ok, _ := unstructured.NestedInt64(deployObj.Object, "status", "collisionCount"); ok && cCount >= 0 && cCount <= 1000 {
			collisionCountMax = int32(cCount)
		}

		if ComputePodTemplateHash(&pts, nil) == selectorHash {
			matchedHash = true
		} else {
			for c := int32(0); c <= collisionCountMax; c++ {
				cVal := c
				if ComputePodTemplateHash(&pts, &cVal) == selectorHash {
					matchedHash = true
					break
				}
			}
		}
		if !matchedHash {
			return nil, fmt.Errorf("ReplicaSet %s template does not match immutable selector hash %s", controllerOwner.Name, selectorHash)
		}
		if !matchesReplicaSetTemplate(resource.Object, obj.Object) {
			return nil, fmt.Errorf("Pod %s does not match ReplicaSet %s template", resource.GetName(), controllerOwner.Name)
		}
		depTs := deployObj.GetCreationTimestamp()
		if depTs.IsZero() {
			return nil, fmt.Errorf("Deployment %s has zero creationTimestamp", deployRef.Name)
		}
		if depTs.Time.After(effectiveTs.Time) {
			effectiveTs = depTs
		}
		var deployHasController bool
		for _, o := range deployObj.GetOwnerReferences() {
			if o.Controller != nil && *o.Controller {
				deployHasController = true
				break
			}
		}
		if deployHasController {
			parentTs, err := newestOwnerCreationTimestamp(ctx, client, *deployObj, 2)
			if err != nil {
				return nil, err
			}
			if parentTs != nil && !parentTs.IsZero() && parentTs.Time.After(effectiveTs.Time) {
				effectiveTs = *parentTs
			}
		}

	case "Job":
		if resource.GetKind() != "Pod" || (resource.GetAPIVersion() != "v1" && resource.GetAPIVersion() != "" && resource.GetAPIVersion() != "core/v1") {
			return nil, fmt.Errorf("unexpected dependent resource %s/%s for Job; expected v1 Pod", resource.GetAPIVersion(), resource.GetKind())
		}
		if controllerOwner.APIVersion != "batch/v1" {
			return nil, fmt.Errorf("unsupported Job APIVersion %s; expected batch/v1", controllerOwner.APIVersion)
		}
		jobTmpl, okJobTmpl, _ := unstructured.NestedMap(obj.Object, "spec", "template")
		if !okJobTmpl || !matchesPodTemplate(resource.Object, jobTmpl) {
			return nil, fmt.Errorf("Pod %s does not match Job %s template", resource.GetName(), controllerOwner.Name)
		}

		// Check if Job was spawned by an ancestor controller (e.g. CronJob)
		parentTs, err := newestOwnerCreationTimestamp(ctx, client, *obj, 1)
		if err != nil {
			return nil, err
		}
		if parentTs != nil && !parentTs.IsZero() {
			// Scheduled Jobs spawned by a parent CronJob inherit the CronJob's verified template age
			effectiveTs = *parentTs
		}

		// Account for mutable Job template updates (e.g., suspended Jobs updated before execution)
		hasTemplateManagedFields := false
		for _, mf := range obj.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:template") {
				hasTemplateManagedFields = true
				if mf.Time == nil || mf.Time.IsZero() {
					return nil, fmt.Errorf("Job %s has template modification without timestamp", controllerOwner.Name)
				}
				if parentTs != nil && !parentTs.IsZero() {
					if mf.Time.Time.After(immediateTs.Time) && mf.Time.Time.After(effectiveTs.Time) {
						effectiveTs = *mf.Time
					}
				} else {
					if mf.Time.Time.After(effectiveTs.Time) {
						effectiveTs = *mf.Time
					}
				}
			}
		}
		if !hasTemplateManagedFields {
			return nil, fmt.Errorf("Job %s has no verifiable template managedFields", controllerOwner.Name)
		}

	case "CronJob":
		if resource.GetKind() != "Job" || resource.GetAPIVersion() != "batch/v1" {
			return nil, fmt.Errorf("unexpected dependent resource %s/%s for CronJob; expected batch/v1 Job", resource.GetAPIVersion(), resource.GetKind())
		}
		if controllerOwner.APIVersion != "batch/v1" {
			return nil, fmt.Errorf("unsupported CronJob APIVersion %s; expected batch/v1", controllerOwner.APIVersion)
		}
		hasJobTemplateManagedFields := false
		for _, mf := range obj.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:jobTemplate") {
				hasJobTemplateManagedFields = true
				if mf.Time == nil || mf.Time.IsZero() {
					return nil, fmt.Errorf("CronJob %s has jobTemplate modification without timestamp", controllerOwner.Name)
				}
				if mf.Time.Time.After(effectiveTs.Time) {
					effectiveTs = *mf.Time
				}
			}
		}
		if !hasJobTemplateManagedFields {
			return nil, fmt.Errorf("CronJob %s has no verifiable jobTemplate managedFields", controllerOwner.Name)
		}
		cronTmpl, okCron, _ := unstructured.NestedMap(obj.Object, "spec", "jobTemplate", "spec", "template")
		jobTmpl, okJob, _ := unstructured.NestedMap(resource.Object, "spec", "template")
		if !okCron || !okJob || !matchesCronJobJobTemplate(jobTmpl, cronTmpl) {
			return nil, fmt.Errorf("Job %s template does not match CronJob %s jobTemplate", resource.GetName(), controllerOwner.Name)
		}
		parentTs, err := newestOwnerCreationTimestamp(ctx, client, *obj, 1)
		if err != nil {
			return nil, err
		}
		if parentTs != nil && !parentTs.IsZero() && parentTs.Time.After(effectiveTs.Time) {
			effectiveTs = *parentTs
		}

	case "ReplicationController":
		return nil, fmt.Errorf("ReplicationController %s does not version pod templates", controllerOwner.Name)

	default:
		return nil, fmt.Errorf("controller owner kind %s has no verified template-versioning mechanism", controllerOwner.Kind)
	}

	return &effectiveTs, nil
}

var generatedLabelKeys = map[string]bool{
	"pod-template-hash":                  true,
	"controller-revision-hash":           true,
	"statefulset.kubernetes.io/pod-name": true,
	"apps.kubernetes.io/pod-index":       true,
	"job-name":                           true,
	"controller-uid":                     true,
	"batch.kubernetes.io/job-name":       true,
	"batch.kubernetes.io/controller-uid": true,
}

func filterGeneratedLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string)
	for k, v := range labels {
		if !generatedLabelKeys[k] {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasUntemplatedSecurityEscalations(podSpec, tmplSpec map[string]any) bool {
	// Check pod-level host namespace settings
	for _, hostField := range []string{"hostNetwork", "hostPID", "hostIPC", "shareProcessNamespace"} {
		pVal, okP := podSpec[hostField].(bool)
		tVal, okT := tmplSpec[hostField].(bool)
		if okP && pVal && (!okT || !tVal) {
			return true
		}
	}

	// Check container-level securityContext escalations for both regular and init containers
	for _, sliceKey := range []string{"containers", "initContainers"} {
		pContainers, _, _ := unstructured.NestedSlice(podSpec, sliceKey)
		tContainers, _, _ := unstructured.NestedSlice(tmplSpec, sliceKey)
		for _, pC := range pContainers {
			pCMap, okP := pC.(map[string]any)
			if !okP {
				continue
			}
			pName, _ := pCMap["name"].(string)
			var tCMap map[string]any
			for _, tC := range tContainers {
				if m, ok := tC.(map[string]any); ok && m["name"] == pName {
					tCMap = m
					break
				}
			}
			if tCMap == nil {
				continue
			}

			// privileged: true in Pod but not in template
			pPriv, okPPriv, _ := unstructured.NestedBool(pCMap, "securityContext", "privileged")
			tPriv, okTPriv, _ := unstructured.NestedBool(tCMap, "securityContext", "privileged")
			if okPPriv && pPriv && (!okTPriv || !tPriv) {
				return true
			}

			// allowPrivilegeEscalation: true in Pod but not in template
			pEsc, okPEsc, _ := unstructured.NestedBool(pCMap, "securityContext", "allowPrivilegeEscalation")
			tEsc, okTEsc, _ := unstructured.NestedBool(tCMap, "securityContext", "allowPrivilegeEscalation")
			if okPEsc && pEsc && (!okTEsc || !tEsc) {
				return true
			}

			// capabilities.add: any capability in Pod that is not in template
			pCaps, okPCaps, _ := unstructured.NestedSlice(pCMap, "securityContext", "capabilities", "add")
			tCaps, okTCaps, _ := unstructured.NestedSlice(tCMap, "securityContext", "capabilities", "add")
			if okPCaps && len(pCaps) > 0 {
				tCapSet := make(map[string]bool)
				if okTCaps {
					for _, c := range tCaps {
						if s, ok := c.(string); ok {
							tCapSet[s] = true
						}
					}
				}
				for _, c := range pCaps {
					if s, ok := c.(string); ok && !tCapSet[s] {
						return true
					}
				}
			}
		}
	}
	return false
}

func matchesPodTemplate(podObj, tmplObj map[string]any) bool {
	if podObj == nil || tmplObj == nil {
		return false
	}
	// 1. Compare spec template-owned fields (structural subset matching that accommodates
	// Kubernetes API-server defaulting, e.g. restartPolicy, dnsPolicy, serviceAccountName, tokens,
	// while strictly failing closed on un-templated containers, ports, or untrusted volumes).
	tmplSpec, okTmplSpec, _ := unstructured.NestedFieldCopy(tmplObj, "spec")
	podSpec, okPodSpec, _ := unstructured.NestedFieldCopy(podObj, "spec")
	if !okTmplSpec || !okPodSpec {
		return false
	}
	tmplSpecMap, okTMap := tmplSpec.(map[string]any)
	podSpecMap, okPMap := podSpec.(map[string]any)
	if !okTMap || !okPMap {
		return false
	}

	// 1a. Strict container count and provenance: Pod cannot add un-templated containers
	podContainers, _, _ := unstructured.NestedSlice(podSpecMap, "containers")
	tmplContainers, _, _ := unstructured.NestedSlice(tmplSpecMap, "containers")
	if len(podContainers) != len(tmplContainers) {
		return false
	}
	// 1b. Strict init container count
	podInit, _, _ := unstructured.NestedSlice(podSpecMap, "initContainers")
	tmplInit, _, _ := unstructured.NestedSlice(tmplSpecMap, "initContainers")
	if len(podInit) != len(tmplInit) {
		return false
	}
	// 1c. Strict ephemeral container count
	podEph, _, _ := unstructured.NestedSlice(podSpecMap, "ephemeralContainers")
	tmplEph, _, _ := unstructured.NestedSlice(tmplSpecMap, "ephemeralContainers")
	if len(podEph) != len(tmplEph) {
		return false
	}

	// 1d. Volumes: every template volume must exist in pod. Any additional volume in pod
	// must strictly be a Kubernetes admission-injected service account token volume.
	if !matchesVolumes(podSpecMap, tmplSpecMap) {
		return false
	}

	// 1e. Reject un-templated security privilege escalations injected by mutating webhooks
	// (e.g. hostNetwork, hostPID, hostIPC, privileged, or additional capabilities).
	if hasUntemplatedSecurityEscalations(podSpecMap, tmplSpecMap) {
		return false
	}

	// 1f. General template fields subset match (including containers, commands, args, resources)
	if !matchesTemplateFields(podSpec, tmplSpec) {
		return false
	}

	// 2. Compare metadata labels (excluding controller-generated labels)
	// Verify that all labels defined in the controller template are present and match in the Pod,
	// while accommodating labels added by mutating admission webhooks or injectors.
	podLabels, _, _ := unstructured.NestedStringMap(podObj, "metadata", "labels")
	tmplLabels, _, _ := unstructured.NestedStringMap(tmplObj, "metadata", "labels")
	cleanPodLabels := filterGeneratedLabels(podLabels)
	cleanTmplLabels := filterGeneratedLabels(tmplLabels)
	for k, v := range cleanTmplLabels {
		if cleanPodLabels[k] != v {
			return false
		}
	}

	// 3. Compare metadata annotations
	// Verify that all annotations defined in the controller template are present and match in the Pod,
	// while accommodating annotations added by mutating admission webhooks or injectors (e.g. sidecar.istio.io/status).
	podAnnotations, _, _ := unstructured.NestedStringMap(podObj, "metadata", "annotations")
	tmplAnnotations, _, _ := unstructured.NestedStringMap(tmplObj, "metadata", "annotations")
	for k, v := range tmplAnnotations {
		if podAnnotations[k] != v {
			return false
		}
	}

	return true
}

// matchesCronJobJobTemplate performs a semantic exact comparison of a Job's spec.template against
// a parent CronJob's spec.jobTemplate.spec.template. While matchesPodTemplate allows structural
// subset matching for mutating admission webhooks on Pods, a Job's template is copied directly
// from the CronJob by the controller and must not contain un-templated additions (such as runAsUser,
// extra containers, volumes, env vars, or labels/annotations), normalizing only standard Kubernetes
// API-server defaulting.
func matchesCronJobJobTemplate(jobTmpl, cronTmpl map[string]any) bool {
	if jobTmpl == nil || cronTmpl == nil {
		return false
	}
	if !matchesPodTemplate(jobTmpl, cronTmpl) {
		return false
	}

	jobSpec, okJobSpec, _ := unstructured.NestedMap(jobTmpl, "spec")
	cronSpec, okCronSpec, _ := unstructured.NestedMap(cronTmpl, "spec")
	if !okJobSpec || !okCronSpec {
		return false
	}

	podSpecAllowedDefaults := map[string]bool{
		"restartPolicy":                 true,
		"dnsPolicy":                     true,
		"terminationGracePeriodSeconds": true,
		"schedulerName":                 true,
	}
	for k := range jobSpec {
		if !podSpecAllowedDefaults[k] {
			if _, exists := cronSpec[k]; !exists {
				return false
			}
		}
	}

	jobPodSec, okJPSec, _ := unstructured.NestedMap(jobSpec, "securityContext")
	cronPodSec, okCPSec, _ := unstructured.NestedMap(cronSpec, "securityContext")
	if okJPSec && jobPodSec != nil {
		for k, v := range jobPodSec {
			if !okCPSec || cronPodSec == nil {
				return false
			}
			cV, exists := cronPodSec[k]
			if !exists || !equalValues(v, cV) {
				return false
			}
		}
	}

	for _, sliceKey := range []string{"containers", "initContainers"} {
		jContainers, _, _ := unstructured.NestedSlice(jobSpec, sliceKey)
		cContainers, _, _ := unstructured.NestedSlice(cronSpec, sliceKey)
		if len(jContainers) != len(cContainers) {
			return false
		}
		cMap := make(map[string]map[string]any, len(cContainers))
		for _, c := range cContainers {
			if m, ok := c.(map[string]any); ok {
				if name, okName := m["name"].(string); okName && name != "" {
					cMap[name] = m
				}
			}
		}
		containerAllowedDefaults := map[string]bool{
			"imagePullPolicy":          true,
			"terminationMessagePath":   true,
			"terminationMessagePolicy": true,
		}
		for _, jC := range jContainers {
			jCMap, ok := jC.(map[string]any)
			if !ok {
				return false
			}
			name, _ := jCMap["name"].(string)
			cCMap, exists := cMap[name]
			if !exists {
				return false
			}
			for k := range jCMap {
				if !containerAllowedDefaults[k] {
					if _, existsInCron := cCMap[k]; !existsInCron {
						return false
					}
				}
			}

			jSec, okJSec, _ := unstructured.NestedMap(jCMap, "securityContext")
			cSec, okCSec, _ := unstructured.NestedMap(cCMap, "securityContext")
			if okJSec && jSec != nil {
				for k, v := range jSec {
					if !okCSec || cSec == nil {
						return false
					}
					cV, exists := cSec[k]
					if !exists || !equalValues(v, cV) {
						return false
					}
				}
			}

			jEnv, _, _ := unstructured.NestedSlice(jCMap, "env")
			cEnv, _, _ := unstructured.NestedSlice(cCMap, "env")
			if len(jEnv) != len(cEnv) {
				return false
			}

			jMounts, _, _ := unstructured.NestedSlice(jCMap, "volumeMounts")
			cMounts, _, _ := unstructured.NestedSlice(cCMap, "volumeMounts")
			if len(jMounts) != len(cMounts) {
				return false
			}
		}
	}

	jVols, _, _ := unstructured.NestedSlice(jobSpec, "volumes")
	cVols, _, _ := unstructured.NestedSlice(cronSpec, "volumes")
	if len(jVols) != len(cVols) {
		return false
	}

	jLabels, _, _ := unstructured.NestedStringMap(jobTmpl, "metadata", "labels")
	cLabels, _, _ := unstructured.NestedStringMap(cronTmpl, "metadata", "labels")
	cleanJLabels := filterGeneratedLabels(jLabels)
	cleanCLabels := filterGeneratedLabels(cLabels)
	for k := range cleanJLabels {
		if _, exists := cleanCLabels[k]; !exists {
			return false
		}
	}
	jAnnotations, _, _ := unstructured.NestedStringMap(jobTmpl, "metadata", "annotations")
	cAnnotations, _, _ := unstructured.NestedStringMap(cronTmpl, "metadata", "annotations")
	for k := range jAnnotations {
		if _, exists := cAnnotations[k]; !exists {
			return false
		}
	}

	return true
}

func matchesVolumes(podSpec, tmplSpec map[string]any) bool {
	podVols, _, _ := unstructured.NestedSlice(podSpec, "volumes")
	tmplVols, _, _ := unstructured.NestedSlice(tmplSpec, "volumes")

	tmplVolMap := make(map[string]map[string]any)
	for _, v := range tmplVols {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok && name != "" {
				tmplVolMap[name] = vm
			}
		}
	}

	for _, v := range podVols {
		vm, ok := v.(map[string]any)
		if !ok {
			return false
		}
		name, _ := vm["name"].(string)
		if tmplV, exists := tmplVolMap[name]; exists {
			if !matchesTemplateFields(vm, tmplV) {
				return false
			}
		} else {
			// Untemplated volume added to pod: must strictly be a trusted service account token volume or controller-generated PVC volume
			if !isServiceAccountTokenVolume(vm) && !isPersistentVolumeClaimVolume(vm) {
				return false
			}
		}
	}
	return true
}

func isServiceAccountTokenVolume(v map[string]any) bool {
	if v == nil {
		return false
	}
	// Check projected token volume (kube-api-access-*)
	if proj, ok := v["projected"].(map[string]any); ok && proj != nil {
		if sources, ok := proj["sources"].([]any); ok {
			for _, s := range sources {
				if sm, ok := s.(map[string]any); ok {
					if _, hasSAToken := sm["serviceAccountToken"]; hasSAToken {
						return true
					}
				}
			}
		}
	}
	// Legacy token secret volume (default-token-*): secret must be the only source.
	if sec, ok := v["secret"].(map[string]any); ok && sec != nil {
		secName, _ := sec["secretName"].(string)
		name, _ := v["name"].(string)
		return strings.Contains(secName, "-token-") && name == secName
	}
	return false
}

func isPersistentVolumeClaimVolume(v map[string]any) bool {
	if v == nil {
		return false
	}
	pvc, ok := v["persistentVolumeClaim"].(map[string]any)
	if !ok || pvc == nil {
		return false
	}
	claimName, _ := pvc["claimName"].(string)
	return claimName != ""
}

func isVolumeMountsSlice(s []any) bool {
	if len(s) == 0 {
		return false
	}
	m, ok := s[0].(map[string]any)
	if !ok {
		return false
	}
	_, hasMount := m["mountPath"]
	return hasMount
}

func matchesTemplateFields(podVal, tmplVal any) bool {
	if tmplVal == nil {
		return true
	}
	if podVal == nil {
		return false
	}

	switch tVal := tmplVal.(type) {
	case map[string]any:
		pMap, ok := podVal.(map[string]any)
		if !ok {
			return false
		}
		for k, vTmpl := range tVal {
			vPod, exists := pMap[k]
			if !exists {
				return false
			}
			if !matchesTemplateFields(vPod, vTmpl) {
				return false
			}
		}
		return true

	case []any:
		pSlice, ok := podVal.([]any)
		if !ok {
			return false
		}
		if len(tVal) == 0 {
			return true
		}
		// Match slices of named objects (containers, volumes, env vars, volume mounts, etc.)
		if isNamedMapSlice(tVal) {
			for _, tElem := range tVal {
				tMap, ok := tElem.(map[string]any)
				if !ok {
					return false
				}
				tName, _ := tMap["name"].(string)
				tMountPath, _ := tMap["mountPath"].(string)
				var matchedPodElem any
				var found bool
				for _, pElem := range pSlice {
					if pM, ok := pElem.(map[string]any); ok {
						if pName, ok := pM["name"].(string); ok && pName == tName {
							if tMountPath != "" {
								pMountPath, _ := pM["mountPath"].(string)
								if pMountPath != tMountPath {
									continue
								}
							}
							matchedPodElem = pElem
							found = true
							break
						}
					}
				}
				if !found || !matchesTemplateFields(matchedPodElem, tMap) {
					return false
				}
			}
			// For volumeMounts, verify any extra mount in pod is the service account token mount
			if isVolumeMountsSlice(tVal) {
				tmplMountNames := make(map[string]bool)
				for _, tElem := range tVal {
					if tMap, ok := tElem.(map[string]any); ok {
						if n, ok := tMap["name"].(string); ok {
							tmplMountNames[n] = true
						}
					}
				}
				for _, pElem := range pSlice {
					if pM, ok := pElem.(map[string]any); ok {
						pName, _ := pM["name"].(string)
						if !tmplMountNames[pName] {
							mountPath, _ := pM["mountPath"].(string)
							if mountPath != "/var/run/secrets/kubernetes.io/serviceaccount" {
								return false
							}
						}
					}
				}
			}
			return true
		}
		// Match slices of port objects (by containerPort if not named)
		if isPortMapSlice(tVal) {
			// Container ports must strictly match count and values (no extra ports allowed)
			if len(pSlice) != len(tVal) {
				return false
			}
			for _, tElem := range tVal {
				tMap, ok := tElem.(map[string]any)
				if !ok {
					return false
				}
				tPort := tMap["containerPort"]
				var matchedPodElem any
				var found bool
				for _, pElem := range pSlice {
					if pM, ok := pElem.(map[string]any); ok {
						if equalValues(pM["containerPort"], tPort) {
							matchedPodElem = pElem
							found = true
							break
						}
					}
				}
				if !found || !matchesTemplateFields(matchedPodElem, tMap) {
					return false
				}
			}
			return true
		}
		// Match slices of unnamed maps (e.g. tolerations)
		if isMapSlice(tVal) {
			for _, tElem := range tVal {
				tMap, ok := tElem.(map[string]any)
				if !ok {
					return false
				}
				var found bool
				for _, pElem := range pSlice {
					if matchesTemplateFields(pElem, tMap) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
		// Slices of primitives (e.g. command, args) require exact ordered match
		if len(pSlice) != len(tVal) {
			return false
		}
		for i := range tVal {
			if !equalValues(pSlice[i], tVal[i]) {
				return false
			}
		}
		return true

	default:
		return equalValues(podVal, tmplVal)
	}
}

func isNamedMapSlice(s []any) bool {
	if len(s) == 0 {
		return false
	}
	m, ok := s[0].(map[string]any)
	if !ok {
		return false
	}
	_, hasName := m["name"]
	return hasName
}

func isPortMapSlice(s []any) bool {
	if len(s) == 0 {
		return false
	}
	m, ok := s[0].(map[string]any)
	if !ok {
		return false
	}
	_, hasPort := m["containerPort"]
	return hasPort
}

func isMapSlice(s []any) bool {
	if len(s) == 0 {
		return false
	}
	_, ok := s[0].(map[string]any)
	return ok
}

func equalValues(podVal, tmplVal any) bool {
	if reflect.DeepEqual(podVal, tmplVal) {
		return true
	}
	sPod := fmt.Sprintf("%v", podVal)
	sTmpl := fmt.Sprintf("%v", tmplVal)
	if sPod == sTmpl {
		return true
	}
	// Try Kubernetes quantity parsing (e.g., "0.2" == "200m", "30Mi" == "30Mi")
	qPod, errPod := resource.ParseQuantity(sPod)
	qTmpl, errTmpl := resource.ParseQuantity(sTmpl)
	if errPod == nil && errTmpl == nil && qPod.Equal(qTmpl) {
		return true
	}
	return false
}

func hasDaemonSetNodeSelector(podObj map[string]any) bool {
	terms, ok, _ := unstructured.NestedSlice(podObj, "spec", "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution", "nodeSelectorTerms")
	if !ok {
		return false
	}
	for _, term := range terms {
		if termMap, ok := term.(map[string]any); ok {
			if fields, ok, _ := unstructured.NestedSlice(termMap, "matchFields"); ok {
				for _, f := range fields {
					if fMap, ok := f.(map[string]any); ok {
						if k, _ := fMap["key"].(string); k == "metadata.name" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp, _, _ := unstructured.NestedFieldCopy(m)
	if res, ok := cp.(map[string]any); ok {
		return res
	}
	return m
}

func matchesRevisionTemplate(podObj, revObj map[string]any) bool {
	var revTmpl map[string]any
	if t, ok, _ := unstructured.NestedMap(revObj, "data", "spec", "template"); ok {
		revTmpl = t
	} else if t, ok, _ := unstructured.NestedMap(revObj, "data", "template"); ok {
		revTmpl = t
	}
	if revTmpl == nil {
		return false
	}
	if matchesPodTemplate(podObj, revTmpl) {
		return true
	}
	// For DaemonSets, the controller injects a node affinity term selecting the target node
	// (matchFields: metadata.name == node.Name) into requiredDuringSchedulingIgnoredDuringExecution.
	// If the template has required node affinity, the controller transformation modifies/replaces terms.
	if hasDaemonSetNodeSelector(podObj) {
		podCopy := deepCopyMap(podObj)
		revCopy := deepCopyMap(revTmpl)
		unstructured.RemoveNestedField(podCopy, "spec", "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution")
		unstructured.RemoveNestedField(revCopy, "spec", "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution")
		return matchesPodTemplate(podCopy, revCopy)
	}
	return false
}

var spewHasher = spew.ConfigState{
	Indent:         " ",
	SortKeys:       true,
	DisableMethods: true,
	SpewKeys:       true,
}

func ComputePodTemplateHash(template *corev1.PodTemplateSpec, collisionCount *int32) string {
	hasher := fnv.New32a()
	hasher.Reset()
	spewHasher.Fprintf(hasher, "%#v", *template)
	if collisionCount != nil && *collisionCount >= 0 {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(collisionCountBytes, uint32(*collisionCount))
		hasher.Write(collisionCountBytes)
	}
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

func matchesReplicaSetTemplate(podObj, rsObj map[string]any) bool {
	rsTmpl, okRS, _ := unstructured.NestedMap(rsObj, "spec", "template")
	if !okRS || rsTmpl == nil {
		return false
	}
	return matchesPodTemplate(podObj, rsTmpl)
}

func newestOwnerCreationTimestamp(
	ctx context.Context,
	client engineapi.Client,
	resource unstructured.Unstructured,
	depth int,
) (*metav1.Time, error) {
	const maxDepth = 10
	if depth > maxDepth {
		return nil, fmt.Errorf("exceeded max ownerReference traversal depth %d", maxDepth)
	}
	owners := resource.GetOwnerReferences()
	var controllerOwner *metav1.OwnerReference
	for i := range owners {
		if owners[i].Controller != nil && *owners[i].Controller {
			controllerOwner = &owners[i]
			break
		}
	}
	if controllerOwner == nil {
		// Root reached; return its creation timestamp
		ts := resource.GetCreationTimestamp()
		if ts.IsZero() {
			return nil, nil
		}
		return &ts, nil
	}
	if client == nil {
		return nil, nil
	}

	// Validate allowed ancestor controller relationships
	switch resource.GetKind() {
	case "ReplicaSet":
		if resource.GetAPIVersion() != "apps/v1" || controllerOwner.APIVersion != "apps/v1" || controllerOwner.Kind != "Deployment" {
			return nil, fmt.Errorf("unsupported controller owner %s/%s for %s/%s; expected apps/v1 Deployment",
				controllerOwner.APIVersion, controllerOwner.Kind, resource.GetAPIVersion(), resource.GetKind())
		}
	case "Job":
		if resource.GetAPIVersion() != "batch/v1" || controllerOwner.APIVersion != "batch/v1" || controllerOwner.Kind != "CronJob" {
			return nil, fmt.Errorf("unsupported controller owner %s/%s for %s/%s; expected batch/v1 CronJob",
				controllerOwner.APIVersion, controllerOwner.Kind, resource.GetAPIVersion(), resource.GetKind())
		}
	default:
		return nil, fmt.Errorf("unsupported intermediate resource kind %s for owner traversal", resource.GetKind())
	}

	ns := resource.GetNamespace()
	obj, err := getCachedResource(ctx, client, controllerOwner.APIVersion, controllerOwner.Kind, ns, controllerOwner.Name)
	if err != nil {
		return nil, err
	}
	if obj == nil || obj.Object == nil {
		return nil, fmt.Errorf("GetResource returned nil object for %s/%s", controllerOwner.Kind, controllerOwner.Name)
	}
	if controllerOwner.UID == "" || obj.GetUID() != controllerOwner.UID {
		return nil, fmt.Errorf("invalid or mismatched controller owner UID for %s/%s", controllerOwner.Kind, controllerOwner.Name)
	}

	ownerTs := obj.GetCreationTimestamp()
	if ownerTs.IsZero() {
		return nil, fmt.Errorf("%s %s has zero creationTimestamp", controllerOwner.Kind, controllerOwner.Name)
	}

	if controllerOwner.Kind == "CronJob" {
		hasJobTemplateManagedFields := false
		for _, mf := range obj.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:jobTemplate") {
				hasJobTemplateManagedFields = true
				if mf.Time == nil || mf.Time.IsZero() {
					return nil, fmt.Errorf("CronJob %s has jobTemplate modification without timestamp", controllerOwner.Name)
				}
				if mf.Time.Time.After(ownerTs.Time) {
					ownerTs = *mf.Time
				}
			}
		}
		if !hasJobTemplateManagedFields {
			return nil, fmt.Errorf("CronJob %s has no verifiable jobTemplate managedFields", controllerOwner.Name)
		}
		if resource.GetKind() == "Job" {
			cronTmpl, okCron, _ := unstructured.NestedMap(obj.Object, "spec", "jobTemplate", "spec", "template")
			jobTmpl, okJob, _ := unstructured.NestedMap(resource.Object, "spec", "template")
			if !okCron || !okJob || !matchesCronJobJobTemplate(jobTmpl, cronTmpl) {
				return nil, fmt.Errorf("Job %s template does not match CronJob %s jobTemplate", resource.GetName(), controllerOwner.Name)
			}
		}
	}

	var parentHasController bool
	for _, o := range obj.GetOwnerReferences() {
		if o.Controller != nil && *o.Controller {
			parentHasController = true
			break
		}
	}
	if !parentHasController {
		return &ownerTs, nil
	}

	parentTs, err := newestOwnerCreationTimestamp(ctx, client, *obj, depth+1)
	if err != nil {
		return nil, err
	}
	if parentTs != nil && !parentTs.IsZero() && parentTs.Time.After(ownerTs.Time) {
		return parentTs, nil
	}
	return &ownerTs, nil
}
