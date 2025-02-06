package utils

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type scanner struct {
	logger          logr.Logger
	engine          engineapi.Engine
	config          config.Configuration
	jp              jmespath.Interface
	client          dclient.Interface
	reportingConfig reportutils.ReportingConfiguration
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
) Scanner {
	return &scanner{
		logger:          logger,
		engine:          engine,
		config:          config,
		jp:              jp,
		client:          client,
		reportingConfig: reportingConfig,
	}
}

func (s *scanner) ScanResource(
	ctx context.Context,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	subResource string,
	ns *corev1.Namespace,
	bindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	policies ...engineapi.GenericPolicy,
) map[*engineapi.GenericPolicy]ScanResult {
	var kpols, vpols, vaps []engineapi.GenericPolicy
	// split policies per nature
	for _, policy := range policies {
		if pol := policy.AsKyvernoPolicy(); pol != nil {
			kpols = append(kpols, policy)
		} else if pol := policy.AsValidatingPolicy(); pol != nil {
			vpols = append(vpols, policy)
		} else if pol := policy.AsValidatingAdmissionPolicy(); pol != nil {
			vaps = append(vaps, policy)
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
	// evaluate validating policies
	for i, policy := range vpols {
		if pol := policy.AsValidatingPolicy(); pol != nil {
			// create compiler
			compiler := celpolicy.NewCompiler()
			// create provider
			provider, err := celengine.NewProvider(compiler, *pol)
			if err != nil {
				logger.Error(err, "failed to create policy provider")
				results[&vpols[i]] = ScanResult{nil, err}
				continue
			}
			// create engine
			engine := celengine.NewEngine(
				provider,
				func(name string) *corev1.Namespace { return ns },
				matching.NewMatcher(),
			)
			// create context provider
			context, err := celpolicy.NewContextProvider(
				s.client.GetKubeClient(),
				nil,
				// TODO
				// []imagedataloader.Option{imagedataloader.WithLocalCredentials(c.RegistryAccess)},
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
				&resource,
				nil,
				false,
				nil,
			)
			engineResponse, err := engine.Handle(ctx, request)
			response := engineapi.EngineResponse{
				Resource: resource,
				PolicyResponse: engineapi.PolicyResponse{
					// TODO: policies at index 0
					Rules: engineResponse.Policies[0].Rules,
				},
			}.WithPolicy(vpols[i])
			results[&vpols[i]] = ScanResult{&response, err}
		}
	}
	// evaluate validating admission policies
	for i, policy := range vaps {
		if pol := policy.AsValidatingAdmissionPolicy(); pol != nil {
			policyData := admissionpolicy.NewPolicyData(*pol)
			for _, binding := range bindings {
				if binding.Spec.PolicyName == pol.Name {
					policyData.AddBinding(binding)
				}
			}
			res, err := admissionpolicy.Validate(policyData, resource, map[string]map[string]string{}, s.client)
			results[&vaps[i]] = ScanResult{&res, err}
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
