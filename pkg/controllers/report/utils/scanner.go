package utils

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

type scanner struct {
	logger          logr.Logger
	engine          engineapi.Engine
	config          config.Configuration
	jp              jmespath.Interface
	client          dclient.Interface
	reportingConfig reportutils.ReportingConfiguration
	gctxStore       gctxstore.Store
	typeConverter   patch.TypeConverterManager
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
		[]admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
		[]*policiesv1alpha1.PolicyException,
		...engineapi.GenericPolicy,
	) map[*engineapi.GenericPolicy]ScanResult
}

func NewScanner(
	logger logr.Logger,
	engine engineapi.Engine,
	config config.Configuration,
	jp jmespath.Interface,
	client dclient.Interface,
	reportingConfig reportutils.ReportingConfiguration,
	gctxStore gctxstore.Store,
	typeConverter patch.TypeConverterManager,
) Scanner {
	return &scanner{
		logger:          logger,
		engine:          engine,
		config:          config,
		jp:              jp,
		client:          client,
		reportingConfig: reportingConfig,
		gctxStore:       gctxStore,
		typeConverter:   typeConverter,
	}
}

func (s *scanner) ScanResource(
	ctx context.Context,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	subResource string,
	ns *corev1.Namespace,
	vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	exceptions []*policiesv1alpha1.PolicyException,
	policies ...engineapi.GenericPolicy,
) map[*engineapi.GenericPolicy]ScanResult {
	var kpols, vpols, mpols, ivpols, vaps, maps []engineapi.GenericPolicy
	// split policies per nature
	for _, policy := range policies {
		if pol := policy.AsKyvernoPolicy(); pol != nil {
			kpols = append(kpols, policy)
		} else if pol := policy.AsValidatingPolicy(); pol != nil {
			vpols = append(vpols, policy)
		} else if pol := policy.AsImageValidatingPolicy(); pol != nil {
			ivpols = append(vpols, policy)
		} else if pol := policy.AsValidatingAdmissionPolicy(); pol != nil {
			vaps = append(vaps, policy)
		} else if pol := policy.AsMutatingAdmissionPolicy(); pol != nil {
			maps = append(maps, policy)
		} else if pol := policy.AsMutatingPolicy(); pol != nil {
			mpols = append(mpols, policy)
		}
	}
	logger := s.logger.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	results := map[*engineapi.GenericPolicy]ScanResult{}
	// evaluate kyverno policies
	var nsLabels map[string]string
	if ns != nil {
		nsLabels = ns.Labels
	}
	for i, policy := range kpols {
		if pol := policy.AsKyvernoPolicy(); pol != nil {
			var errors []error
			var response *engineapi.EngineResponse
			var err error
			if s.reportingConfig.ValidateReportsEnabled() {
				response, err = s.validateResource(ctx, resource, nsLabels, pol)
				if err != nil {
					logger.Error(err, "failed to scan resource")
					errors = append(errors, err)
				}
			}
			spec := pol.GetSpec()
			if spec.HasVerifyImages() && len(errors) == 0 && s.reportingConfig.ImageVerificationReportsEnabled() {
				if response != nil {
					// remove responses of verify image rules
					ruleResponses := make([]engineapi.RuleResponse, 0, len(response.PolicyResponse.Rules))
					for _, v := range response.PolicyResponse.Rules {
						if v.RuleType() != engineapi.ImageVerify {
							ruleResponses = append(ruleResponses, v)
						}
					}
					response.PolicyResponse.Rules = ruleResponses
				}
				ivResponse, err := s.validateImages(ctx, resource, nsLabels, pol)
				if err != nil {
					logger.Error(err, "failed to scan images")
					errors = append(errors, err)
				}
				if response == nil {
					response = ivResponse
				} else if ivResponse != nil {
					response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ivResponse.PolicyResponse.Rules...)
				}
			}
			results[&kpols[i]] = ScanResult{response, multierr.Combine(errors...)}
		}
	}

	for i, policy := range vpols {
		if pol := policy.AsValidatingPolicy(); pol != nil {
			compiler := vpolcompiler.NewCompiler()
			provider, err := vpolengine.NewProvider(compiler, []policiesv1alpha1.ValidatingPolicy{*pol}, exceptions)
			if err != nil {
				logger.Error(err, "failed to create policy provider")
				results[&vpols[i]] = ScanResult{nil, err}
				continue
			}
			engine := vpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
			)
			context, err := libs.NewContextProvider(
				s.client,
				nil,
				// TODO
				// []imagedataloader.Option{imagedataloader.WithLocalCredentials(c.RegistryAccess)},
				s.gctxStore,
				false,
			)
			if err != nil {
				logger.Error(err, "failed to create cel context provider")
				results[&vpols[i]] = ScanResult{nil, err}
				continue
			}
			request := celengine.Request(
				context,
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
				rules = append(rules, policy.Rules...)
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
		if pol := policy.AsMutatingPolicy(); pol != nil {
			compiler := mpolcompiler.NewCompiler()
			provider, err := mpolengine.NewProvider(compiler, []policiesv1alpha1.MutatingPolicy{*pol}, exceptions)
			if err != nil {
				logger.Error(err, "failed to create policy provider")
				results[&mpols[i]] = ScanResult{nil, err}
				continue
			}
			context, err := libs.NewContextProvider(
				s.client,
				nil,
				// TODO
				// []imagedataloader.Option{imagedataloader.WithLocalCredentials(c.RegistryAccess)},
				s.gctxStore,
				false,
			)
			engine := mpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
				s.typeConverter,
				context,
			)

			if err != nil {
				logger.Error(err, "failed to create cel context provider")
				results[&vpols[i]] = ScanResult{nil, err}
				continue
			}
			request := celengine.Request(
				context,
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
				for j, r := range policy.Rules {
					if r.Status() == engineapi.RuleStatusPass {
						policy.Rules[j] = *engineapi.RuleFail("", engineapi.Mutation, "mutation is not applied", nil)
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
		if pol := policy.AsImageValidatingPolicy(); pol != nil {
			provider, err := ivpolengine.NewProvider([]policiesv1alpha1.ImageValidatingPolicy{*pol}, exceptions)
			if err != nil {
				logger.Error(err, "failed to create image verification policy provider")
				results[&ivpols[i]] = ScanResult{nil, err}
				continue
			}
			engine := ivpolengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
				s.client.GetKubeClient().CoreV1().Secrets(""),
				nil,
			)
			context, err := libs.NewContextProvider(s.client, nil, gctxstore.New(), false)
			if err != nil {
				logger.Error(err, "failed to create cel context provider")
				results[&ivpols[i]] = ScanResult{nil, err}
				continue
			}
			request := celengine.Request(
				context,
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
			engineResponse, _, err := engine.HandleMutating(ctx, request, nil)
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
			res, err := admissionpolicy.Validate(policyData, resource, resource.GroupVersionKind(), gvr, map[string]map[string]string{}, s.client, false)
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
			res, err := admissionpolicy.Mutate(policyData, resource, resource.GroupVersionKind(), gvr, map[string]map[string]string{}, s.client, false, true)
			results[&maps[i]] = ScanResult{&res, err}
		}
	}
	return results
}

func (s *scanner) validateResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, error) {
	policyCtx, err := engine.NewPolicyContext(s.jp, resource, kyvernov1.Create, nil, s.config)
	if err != nil {
		return nil, err
	}
	policyCtx = policyCtx.
		WithNewResource(resource).
		WithPolicy(policy).
		WithNamespaceLabels(nsLabels)
	response := s.engine.Validate(ctx, policyCtx)
	if len(response.PolicyResponse.Rules) > 0 {
		s.logger.V(6).Info("validateResource", "policy", policy, "response", response)
	}
	return &response, nil
}

func (s *scanner) validateImages(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, error) {
	annotations := resource.GetAnnotations()
	if annotations != nil {
		resource = *resource.DeepCopy()
		delete(annotations, kyverno.AnnotationImageVerify)
		resource.SetAnnotations(annotations)
	}
	policyCtx, err := engine.NewPolicyContext(s.jp, resource, kyvernov1.Create, nil, s.config)
	if err != nil {
		return nil, err
	}
	policyCtx = policyCtx.
		WithNewResource(resource).
		WithPolicy(policy).
		WithNamespaceLabels(nsLabels)
	response, _ := s.engine.VerifyAndPatchImages(ctx, policyCtx)
	if len(response.PolicyResponse.Rules) > 0 {
		s.logger.V(6).Info("validateImages", "policy", policy, "response", response)
	}
	return &response, nil
}
