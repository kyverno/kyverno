package mpol

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policy"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	utilsslices "github.com/kyverno/kyverno/pkg/utils/slices"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type processor struct {
	kyvernoClient versioned.Interface
	client        dclient.Interface

	engine        mpolengine.Engine
	mapper        meta.RESTMapper
	context       libs.Context
	statusControl common.StatusControlInterface

	eventGen event.Interface
}

type gvkItem struct {
	gvk            schema.GroupVersionKind
	parentResource schema.GroupVersionResource
	subresource    string
	resourceNames  []string
}

type resolvedTarget struct {
	object         unstructured.Unstructured
	parentResource schema.GroupVersionResource
	subresource    string
}

func NewProcessor(client dclient.Interface,
	kyvernoClient versioned.Interface,
	mpolEngine mpolengine.Engine,
	mapper meta.RESTMapper,
	context libs.Context,
	statusControl common.StatusControlInterface,
	eventGen event.Interface,
) *processor {
	return &processor{
		client:        client,
		kyvernoClient: kyvernoClient,
		engine:        mpolEngine,
		mapper:        mapper,
		context:       context,
		statusControl: statusControl,
		eventGen:      eventGen,
	}
}

func (p *processor) Process(ur *kyvernov2.UpdateRequest) error {
	var (
		err      error
		failures []error
		targets  []resolvedTarget
	)

	if ur.Spec.Policy == "" {
		return updateURStatus(p.statusControl, *ur, fmt.Errorf("update request %s has empty policy key", ur.GetName()), nil)
	}

	mpol, err := p.GetPolicy(ur)
	if mpol == nil {
		return err
	}

	targetConstraints := mpol.GetMatchConstraints()
	if len(mpol.GetTargetMatchConstraints().ResourceRules) != 0 && mpol.GetTargetMatchConstraints().Expression == "" {
		targetConstraints = mpol.GetTargetMatchConstraints().MatchResources
	}

	if mpol.GetTargetMatchConstraints().Expression == "" {
		results := collectGVK(p.client, p.mapper, targetConstraints, mpol.GetNamespace())
		list := make([]resolvedTarget, 0)
		for ns, gvks := range results {
			for r := range gvks {
				if r.gvk.Kind == "Namespace" || ns == "*" {
					ns = ""
				}

				resources, err := p.client.ListResource(context.TODO(), r.gvk.GroupVersion().String(), r.gvk.Kind, ns, targetConstraints.ObjectSelector)
				if err != nil {
					failures = append(failures, fmt.Errorf("failed to fetch targets %s for mpol %s: %v", r.gvk.String(), ur.Spec.GetPolicyKey(), err))
					continue
				}

				if len(r.resourceNames) > 0 {
					resources.Items = utilsslices.Filter(resources.Items, func(u unstructured.Unstructured) bool {
						return slices.Contains(r.resourceNames, u.GetName())
					})
				}
				for _, resource := range resources.Items {
					list = append(list, resolvedTarget{
						object:         resource,
						parentResource: r.parentResource,
						subresource:    r.subresource,
					})
				}
			}
		}
		targets = list
	} else {
		var objects *unstructured.UnstructuredList
		objects, err = p.getTargetsFromExpression(context.TODO(), ur, mpol)
		if err != nil {
			return updateURStatus(p.statusControl, *ur, err, nil)
		}
		if objects != nil {
			for _, object := range objects.Items {
				parentResource, subresource, err := p.resolveTargetRoute(object, mpol.GetTargetMatchConstraints().MatchResources)
				if err != nil {
					failures = append(failures, fmt.Errorf("failed to resolve target route for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
					continue
				}
				targets = append(targets, resolvedTarget{object: object, parentResource: parentResource, subresource: subresource})
			}
		}
	}

	if len(targets) == 0 {
		return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
	}

	baseAR := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	// Derive policyName and scopePredicate from the resolved mpol object rather than from the
	// UR key. Admission-webhook URs store only the bare policy name (reconciler.MatchesMutateExisting
	// returns GetName(), not namespace/name), so ParsePolicyKey on the UR key would yield an
	// empty namespace for NamespacedMutatingPolicies, causing the wrong scope predicate to be used.
	policyName := mpol.GetName()
	scopePredicate := mpolengine.ClusteredPolicy()
	if ns := mpol.GetNamespace(); ns != "" {
		scopePredicate = mpolengine.NamespacedPolicy(ns)
	}
	for _, target := range targets {
		object := &target.object

		// Build the AdmissionRequest for this target. For background-only scans there is no
		// real admission request, so we construct a synthetic one from the target resource.
		// Operation is Update (background scans mutate already-existing resources).
		// Object.Raw, Kind, Resource, Namespace and Name are populated so that request.*
		// CEL variables (request.object, request.namespace, etc.) reflect the actual target.
		ar := baseAR
		if ar == nil {
			raw, err := json.Marshal(object.Object)
			if err != nil {
				failures = append(failures, fmt.Errorf("failed to marshal target object for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
				continue
			}
			gvk := object.GroupVersionKind()
			ar = &admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Kind: metav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
				Resource: metav1.GroupVersionResource{
					Group:    target.parentResource.Group,
					Version:  target.parentResource.Version,
					Resource: target.parentResource.Resource,
				},
				SubResource: target.subresource,
				Namespace:   object.GetNamespace(),
				Name:        object.GetName(),
				Object:      runtime.RawExtension{Raw: raw},
			}
		}

		attr := admission.NewAttributesRecord(
			object,
			nil,
			object.GroupVersionKind(),
			object.GetNamespace(),
			object.GetName(),
			target.parentResource,
			target.subresource,
			admission.Operation(ar.Operation),
			nil,
			false,
			admissionpolicy.NewUser(ar.UserInfo),
		)

		response, err := p.engine.Evaluate(context.TODO(), attr, *ar, mpolengine.And(mpolengine.MatchNames(policyName), scopePredicate))
		if err != nil {
			failures = append(failures, fmt.Errorf("failed to evaluate mpol %s: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		if response.PatchedResource != nil {
			object, err = p.client.GetResource(context.TODO(), object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())
			if err != nil {
				failures = append(failures, fmt.Errorf("failed to refresh target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
				continue
			}
			new := response.PatchedResource
			new.SetResourceVersion(object.GetResourceVersion())
			subresources := []string{}
			if target.subresource != "" {
				subresources = append(subresources, target.subresource)
			}
			if _, err := p.client.UpdateResource(context.TODO(), new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new.Object, false, subresources...); err != nil {
				failures = append(failures, fmt.Errorf("failed to update target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
				continue
			}

			err := p.audit(object, &response)
			if err != nil {
				logger.Error(err, "failed to create reports for mpol", "mpol", ur.Spec.GetPolicyKey())
			}
		}
	}
	return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}

func (p *processor) resolveTargetRoute(object unstructured.Unstructured, constraints admissionregistrationv1.MatchResources) (schema.GroupVersionResource, string, error) {
	mapping, err := p.mapper.RESTMapping(object.GroupVersionKind().GroupKind(), object.GroupVersionKind().Version)
	if err != nil {
		return schema.GroupVersionResource{}, "", err
	}
	for _, rule := range constraints.ResourceRules {
		for _, group := range rule.APIGroups {
			if !wildcard.Match(group, mapping.Resource.Group) {
				continue
			}
			for _, version := range rule.APIVersions {
				if !wildcard.Match(version, mapping.Resource.Version) {
					continue
				}
				for _, resource := range rule.Resources {
					parts := strings.SplitN(resource, "/", 2)
					if wildcard.Match(parts[0], mapping.Resource.Resource) {
						if len(parts) == 2 {
							return mapping.Resource, parts[1], nil
						}
						return mapping.Resource, "", nil
					}
				}
			}
		}
	}
	return mapping.Resource, "", nil
}

func (p *processor) audit(object *unstructured.Unstructured, response *mpolengine.EngineResponse) error {
	allEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	reportableEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: r.Rules,
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))
		allEngineResponses = append(allEngineResponses, engineResponse)
		if reportutils.IsPolicyReportable(r.Policy) {
			reportableEngineResponses = append(reportableEngineResponses, engineResponse)
		}
	}

	events := webhookutils.GenerateEvents(allEngineResponses, false)
	p.eventGen.Add(events...)

	if !reportutils.ReportingCfg.MutateExistingReportsEnabled() {
		return nil
	}
	if object.GetName() == "" || object.GetUID() == "" {
		return nil
	}

	// Skip report creation for subresources (e.g., pods/exec) as they have empty name/UID.
	// Subresources don't have their own resources in Kubernetes, so reports cannot be created for them.
	if object.GetName() == "" {
		return nil
	}

	report := reportutils.BuildMutateExistingReport(object.GetNamespace(), object.GroupVersionKind(), object.GetName(), object.GetUID(), reportableEngineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(context.TODO(), func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, p.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func collectGVK(client dclient.Interface, mapper meta.RESTMapper, m admissionregistrationv1.MatchResources, ns string) map[string]sets.Set[*gvkItem] {
	result := make(map[string]sets.Set[*gvkItem])

	gvkSet := sets.New[*gvkItem]()
	for _, rule := range m.ResourceRules {
		for _, group := range rule.APIGroups {
			for _, version := range rule.APIVersions {
				for _, resource := range rule.Resources {
					baseResource := resource
					if strings.Contains(resource, "/") {
						baseResource = strings.Split(resource, "/")[0]
					}
					gvr := schema.GroupVersionResource{
						Group:    group,
						Version:  version,
						Resource: baseResource,
					}
					gvk, err := mapper.KindFor(gvr)
					if err != nil {
						continue
					}
					subresource := ""
					if parts := strings.SplitN(resource, "/", 2); len(parts) == 2 {
						subresource = parts[1]
					}
					gvkSet.Insert(&gvkItem{gvk: gvk, parentResource: gvr, subresource: subresource, resourceNames: rule.ResourceNames})
				}
			}
		}
	}

	if ns != "" {
		namespace, err := client.GetResource(context.TODO(), "v1", "Namespace", "", ns)
		if err != nil {
			return result
		}
		result[namespace.GetName()] = gvkSet
		return result
	} else if m.NamespaceSelector != nil {
		namespaces, err := client.ListResource(context.TODO(), "v1", "Namespace", "", m.NamespaceSelector)
		if err != nil {
			return result
		}

		for _, ns := range namespaces.Items {
			result[ns.GetName()] = gvkSet
		}
	} else {
		result["*"] = gvkSet
	}
	return result
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}

func (p *processor) GetPolicy(ur *kyvernov2.UpdateRequest) (v1beta1.MutatingPolicyLike, error) {
	var mpol v1beta1.MutatingPolicyLike
	var err error

	failures := make([]error, 0, 1)

	name, ns := policy.ParsePolicyKey(ur.Spec.Policy)
	if ns != "" {
		// Namespaced policy: go directly to NamespacedMutatingPolicy lookup.
		mpol, err = p.kyvernoClient.PoliciesV1beta1().NamespacedMutatingPolicies(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return mpol, nil
		}
	} else {
		// Cluster-scoped policy: try MutatingPolicy first.
		mpol, err = p.kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return mpol, nil
		}
		// Fallback: CELMutate URs created from admission webhooks use the bare policy name
		// (reconciler.MatchesMutateExisting returns GetName(), not namespace/name).
		// Since NamespacedMutatingPolicies only match resources in their own namespace,
		// the admission request's namespace equals the policy's namespace.
		if ar := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest; ar != nil && ar.Namespace != "" {
			nmpol, nerr := p.kyvernoClient.PoliciesV1beta1().NamespacedMutatingPolicies(ar.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if nerr == nil {
				return nmpol, nil
			}
			err = multierr.Combine(err, nerr)
		}
	}

	failures = append(failures, fmt.Errorf("failed to fetch mpol %s: %v", ur.Spec.GetPolicyKey(), err))
	return nil, updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}

func (p *processor) getTargetsFromExpression(ctx context.Context, ur *kyvernov2.UpdateRequest, mpol v1beta1.MutatingPolicyLike) (*unstructured.UnstructuredList, error) {
	ar := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	if ar == nil {
		return nil, fmt.Errorf("invalid update request passed, the fields needed to extract resource data are nil")
	}

	raw := ar.Object.Raw
	if ar.Operation == admissionv1.Delete {
		raw = ar.OldObject.Raw
	}
	if raw == nil {
		return nil, fmt.Errorf("invalid update request passed, the fields needed to extract resource data are nil")
	}

	var urResource unstructured.Unstructured
	err := json.Unmarshal(raw, &urResource)
	if err != nil {
		return nil, err
	}

	originalObj := &urResource
	if ar.Operation != admissionv1.Delete {
		originalObj, err = p.client.GetResource(ctx, urResource.GetAPIVersion(), urResource.GetKind(), urResource.GetNamespace(), urResource.GetName())
		if err != nil {
			return nil, err
		}
	}
	pol, err := p.engine.GetCompiledPolicy(mpol.GetName())
	if err != nil {
		return nil, err
	}
	// the trigger GVR is informational on the attributes record; tolerate
	// environments where the trigger kind is not registered in the mapper
	var triggerResource schema.GroupVersionResource
	if mapping, err := p.mapper.RESTMapping(originalObj.GroupVersionKind().GroupKind(), originalObj.GroupVersionKind().Version); err == nil {
		triggerResource = mapping.Resource
	}
	attr := admission.NewAttributesRecord(
		originalObj,
		nil,
		originalObj.GroupVersionKind(),
		originalObj.GetNamespace(),
		originalObj.GetName(),
		triggerResource,
		ar.SubResource,
		admission.Operation(ar.Operation),
		nil,
		false,
		admissionpolicy.NewUser(ar.UserInfo),
	)
	if !pol.CompiledPolicy.MatchesConditions(ctx, attr, ar, nil, p.context) {
		return nil, nil
	}
	unstructuredResources, err := pol.CompiledPolicy.EvaluateTargetExpression(ctx, attr, ar, nil)
	if err != nil {
		return nil, err
	}
	if unstructuredResources == nil {
		// fall back to runtime compilation when the compiled policy does not carry
		// a target expression program, preserving the previous evaluation behavior
		if expr := mpol.GetTargetMatchConstraints().Expression; expr != "" {
			data := map[string]any{compiler.ObjectKey: originalObj.Object}
			vars := lazy.NewMapValue(compiler.VariablesType)
			data[compiler.VariablesKey] = vars
			for name, variable := range pol.CompiledPolicy.GetCompiledVariables() {
				vars.Append(name, func(*lazy.MapValue) ref.Val {
					out, _, err := variable.ContextEval(ctx, data)
					if out != nil {
						return out
					}
					if err != nil {
						return types.WrapErr(err)
					}
					return nil
				})
			}
			unstructuredResources, err = p.getResourcesFromExpression(ctx, expr, mpol.GetNamespace(), data)
			if err != nil {
				return nil, err
			}
		}
		if unstructuredResources == nil {
			return nil, nil
		}
	}

	targets := &unstructured.UnstructuredList{}

	if items, ok := unstructuredResources["items"].([]interface{}); ok {
		if len(items) == 0 {
			return nil, nil
		}

		for _, o := range items {
			m, ok := o.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("item is not a valid Kubernetes object: %#v", o)
			}

			targets.Items = append(targets.Items, unstructured.Unstructured{Object: m})
		}
		return targets, nil
	}

	targets.Items = append(targets.Items, unstructured.Unstructured{Object: unstructuredResources})
	return targets, nil
}

func (p *processor) getResourcesFromExpression(ctx context.Context, expr, policyNs string, data map[string]interface{}) (map[string]interface{}, error) {
	e, err := BuildMpolTargetEvalEnv(libs.GetLibsCtx(), policyNs)
	if err != nil {
		return nil, err
	}

	ast, issues := e.Compile(expr)
	if err := issues.Err(); err != nil {
		return nil, field.Invalid(nil, expr, err.Error())
	}
	if !ast.OutputType().IsExactType(types.NewMapType(types.StringType, types.AnyType)) {
		return nil, field.Invalid(nil, expr, "output type of the target selector expression must be a map")
	}
	prog, err := e.Program(ast)
	if err != nil {
		return nil, err
	}
	out, _, err := prog.ContextEval(ctx, data)
	if err != nil {
		return nil, err
	}
	unstructuredResources, err := utils.ConvertToNative[map[string]interface{}](out)
	if err != nil {
		return nil, err
	}
	return unstructuredResources, nil
}
