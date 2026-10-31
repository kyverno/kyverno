package utils

import (
	"context"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type scanner struct {
	logger        logr.Logger
	config        config.Configuration
	client        dclient.Interface
	secretLister  corev1listers.SecretLister
	gctxStore     gctxstore.Store
	mapper        meta.RESTMapper
	typeConverter patch.TypeConverterManager
}

type ScanResult struct {
	EngineResponse *engineapi.EngineResponse
	Error          error
}

type Scanner interface {
	ScanResource(
		context.Context,
		unstructured.Unstructured,
		schema.GroupVersionResource,
		string,
		*corev1.Namespace,
		[]admissionregistrationv1.ValidatingAdmissionPolicyBinding,
		[]admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
		[]*policiesv1beta1.PolicyException,
		...engineapi.GenericPolicy,
	) map[*engineapi.GenericPolicy]ScanResult
}

func NewScanner(
	logger logr.Logger,
	config config.Configuration,
	client dclient.Interface,
	gctxStore gctxstore.Store,
	mapper meta.RESTMapper,
	secretLister corev1listers.SecretLister,
	typeConverter patch.TypeConverterManager,
) Scanner {
	return &scanner{
		logger:        logger,
		config:        config,
		client:        client,
		gctxStore:     gctxStore,
		mapper:        mapper,
		secretLister:  secretLister,
		typeConverter: typeConverter,
	}
}

func (s *scanner) ScanResource(
	ctx context.Context,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	subResource string,
	ns *corev1.Namespace,
	vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	mapBindings []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
	exceptions []*policiesv1beta1.PolicyException,
	policies ...engineapi.GenericPolicy,
) map[*engineapi.GenericPolicy]ScanResult {
	// EnsureBackground, not Ensure: ScanResource runs under the background-controller or
	// reports-controller pod, whose chart memory limit is a third of the admission
	// controller's (pkg/cel/requestbudget's own comment on backgroundBudget) -- one shared
	// CEL cost budget for every policy this resource is scanned against below.
	ctx, _ = requestbudget.EnsureBackground(ctx)
	logger := s.logger.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	results := map[*engineapi.GenericPolicy]ScanResult{}

	if !s.checkResourceFilters(resource, subResource) {
		logger.V(4).Info("resource is filtered out by the configured resourceFilter, skipping scan")

		return results
	}

	var vpols, mpols, ivpols, vaps, maps []engineapi.GenericPolicy
	// split policies per nature
	for _, policy := range policies {
		if pol := policy.AsValidatingPolicyLike(); pol != nil {
			vpols = append(vpols, policy)
		} else if pol := policy.AsImageValidatingPolicyLike(); pol != nil {
			ivpols = append(ivpols, policy)
		} else if pol := policy.AsValidatingAdmissionPolicy(); pol != nil {
			vaps = append(vaps, policy)
		} else if pol := policy.AsMutatingAdmissionPolicy(); pol != nil {
			maps = append(maps, policy)
		} else if pol := policy.AsMutatingPolicyLike(); pol != nil {
			mpols = append(mpols, policy)
		}
	}

	for i, policy := range vpols {
		if pol := policy.AsValidatingPolicyLike(); pol != nil {
			compiler := vpolcompiler.NewCompiler()
			provider, err := vpolengine.NewProvider(compiler, []policiesv1beta1.ValidatingPolicyLike{pol}, exceptions)
			if err != nil {
				logger.Error(err, "failed to create policy provider")
				results[&vpols[i]] = ScanResult{nil, err}
				continue
			}
			engine := vpolengine.NewMetricWrapper(vpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
			), metrics.BackgroundScan)

			request := celengine.Request(
				libs.GetLibsCtx(),
				resource.GroupVersionKind(),
				gvr,
				subResource,
				resource.GetName(),
				resource.GetNamespace(),
				admissionv1.Create,
				authenticationv1.UserInfo{},
				&resource,
				nil,
				false,
				nil,
			)
			engineResponse, err := engine.Handle(ctx, request, nil)
			rules := make([]engineapi.RuleResponse, 0)
			for _, policy := range engineResponse.Policies {
				rules = append(rules, filterMatchConditionSkips(policy.Rules)...)
			}

			response := engineapi.EngineResponse{
				Resource: resource,
				PolicyResponse: engineapi.PolicyResponse{
					Rules: rules,
				},
			}.WithPolicy(vpols[i])
			results[&vpols[i]] = ScanResult{&response, err}
		}
	}

	for i, policy := range mpols {
		if pol := policy.AsMutatingPolicyLike(); pol != nil {
			compiler := mpolcompiler.NewCompiler()
			provider, err := mpolengine.NewProvider(compiler, []policiesv1beta1.MutatingPolicyLike{pol}, exceptions, libs.GetLibsCtx())
			if err != nil {
				logger.Error(err, "failed to create policy provider")
				results[&mpols[i]] = ScanResult{nil, err}
				continue
			}

			engine := mpolengine.NewMetricWrapper(mpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
				s.typeConverter,
				libs.GetLibsCtx(),
			), metrics.BackgroundScan)

			request := celengine.Request(
				libs.GetLibsCtx(),
				resource.GroupVersionKind(),
				gvr,
				subResource,
				resource.GetName(),
				resource.GetNamespace(),
				admissionv1.Create,
				authenticationv1.UserInfo{},
				&resource,
				nil,
				false,
				nil,
			)
			engineResponse, err := engine.Handle(ctx, request, nil)
			patched := engineResponse.PatchedResource
			rules := make([]engineapi.RuleResponse, 0)
			for _, policy := range engineResponse.Policies {
				for j, r := range policy.Rules {
					if r.Status() == engineapi.RuleStatusPass && len(r.Exceptions()) == 0 {
						if !equality.Semantic.DeepEqual(resource.DeepCopyObject(), patched.DeepCopyObject()) {
							policy.Rules[j] = *engineapi.RuleFail("", engineapi.Mutation, "mutation is not applied", nil)
						}
					}
				}
				rules = append(rules, policy.Rules...)
			}

			response := engineapi.EngineResponse{
				Resource: resource,
				PolicyResponse: engineapi.PolicyResponse{
					Rules: rules,
				},
			}.WithPolicy(mpols[i])
			results[&mpols[i]] = ScanResult{&response, err}
		}
	}

	for i, policy := range ivpols {
		if pol := policy.AsImageValidatingPolicyLike(); pol != nil {
			provider, err := ivpolengine.NewProvider(iveval.NewCompiler(s.secretLister), []policiesv1beta1.ImageValidatingPolicyLike{pol}, exceptions)
			if err != nil {
				logger.Error(err, "failed to create image verification policy provider")
				results[&ivpols[i]] = ScanResult{nil, err}
				continue
			}
			engine := ivpolengine.NewMetricWrapper(ivpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
				s.secretLister,
				imageverifycache.DisabledImageVerifyCache(),
				s.config,
			), metrics.BackgroundScan)
			request := celengine.Request(
				libs.GetLibsCtx(),
				resource.GroupVersionKind(),
				gvr,
				subResource,
				resource.GetName(),
				resource.GetNamespace(),
				admissionv1.Create,
				authenticationv1.UserInfo{},
				&resource,
				nil,
				false,
				nil,
			)
			engineResponse, err := engine.HandleValidating(ctx, request, nil)
			response := engineapi.EngineResponse{
				Resource:       resource,
				PolicyResponse: engineapi.PolicyResponse{},
			}.WithPolicy(ivpols[i])

			if len(engineResponse.Policies) >= 1 {
				response.PolicyResponse.Rules = []engineapi.RuleResponse{engineResponse.Policies[0].Result}
			}

			results[&ivpols[i]] = ScanResult{&response, err}
		}
	}

	for i, policy := range vaps {
		if policyData := policy.AsValidatingAdmissionPolicy(); policyData != nil {
			for _, binding := range vapBindings {
				if binding.Spec.PolicyName == policyData.GetDefinition().GetName() {
					policyData.AddBinding(binding)
				}
			}
			res, err := admissionpolicy.Validate(policyData, resource, resource.GroupVersionKind(), gvr, map[string]map[string]string{}, s.client, &authenticationv1.UserInfo{}, false)
			results[&vaps[i]] = ScanResult{&res, err}
		}
	}

	for i, policy := range maps {
		if policyData := policy.AsMutatingAdmissionPolicy(); policyData != nil {
			for _, binding := range mapBindings {
				if binding.Spec.PolicyName == policyData.GetDefinition().GetName() {
					policyData.AddBinding(binding)
				}
			}
			res, err := admissionpolicy.Mutate(policyData, resource, resource.GroupVersionKind(), gvr, map[string]map[string]string{}, s.client, &authenticationv1.UserInfo{}, false, true)
			results[&maps[i]] = ScanResult{&res, err}
		}
	}
	return results
}

func (s *scanner) checkResourceFilters(resource unstructured.Unstructured, subresource string) bool {
	if resource.Object != nil {
		if s.config.ToFilter(resource.GroupVersionKind(), subresource, resource.GetNamespace(), resource.GetName()) {
			return false
		}
	}
	return true
}

// filterMatchConditionSkips removes RuleResponses that were skipped solely
// because matchConditions evaluated to false. These are internal routing
// signals and should not appear in PolicyReports.
func filterMatchConditionSkips(rules []engineapi.RuleResponse) []engineapi.RuleResponse {
	result := make([]engineapi.RuleResponse, 0, len(rules))
	for _, r := range rules {
		if r.Status() == engineapi.RuleStatusSkip && r.SkipReason() == engineapi.SkipReasonMatchConditions {
			continue
		}
		result = append(result, r)
	}
	return result
}
