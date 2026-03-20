package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	authzengine "github.com/kyverno/kyverno-authz/pkg/engine"
	authzcompiler "github.com/kyverno/kyverno-authz/pkg/engine/compiler"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/sdk/core"
	"github.com/kyverno/sdk/core/dispatchers"
	"github.com/kyverno/sdk/core/handlers"
	"github.com/kyverno/sdk/core/resulters"
	sdkpolicy "github.com/kyverno/sdk/extensions/policy"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type AuthzProcessor struct {
	rc            *ResultCounts
	dclient       dclient.Interface
	httpPolicies  []*policiesv1beta1.ValidatingPolicy
	envoyPolicies []*policiesv1beta1.ValidatingPolicy
}

func (p *AuthzProcessor) ApplyHTTPPolicies(resources []*authzhttp.CheckRequest) ([]engineapi.EngineResponse, error) {
	responses := make([]engineapi.EngineResponse, 0)
	for _, req := range resources {
		for _, vpol := range p.httpPolicies {
			resp, err := processHTTPPolicy(vpol, req, p.dclient)
			if err != nil {
				return nil, err
			}
			p.rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}
	return responses, nil
}

func (p *AuthzProcessor) ApplyEnvoyPolicies(resources []*authv3.CheckRequest) ([]engineapi.EngineResponse, error) {
	responses := make([]engineapi.EngineResponse, 0)
	for _, req := range resources {
		for _, vpol := range p.envoyPolicies {
			resp, err := processEnvoyPolicy(vpol, req, p.dclient)
			if err != nil {
				return nil, err
			}
			p.rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}
	return responses, nil
}

func NewAuthzProcessor(
	rc *ResultCounts,
	dclient dclient.Interface,
	httpPolicies []*policiesv1beta1.ValidatingPolicy,
	envoyPolicies []*policiesv1beta1.ValidatingPolicy,
) *AuthzProcessor {
	return &AuthzProcessor{
		rc:            rc,
		dclient:       dclient,
		httpPolicies:  httpPolicies,
		envoyPolicies: envoyPolicies,
	}
}

func processEnvoyPolicy(vpol *policiesv1beta1.ValidatingPolicy, request *authv3.CheckRequest, dClient dclient.Interface) (engineapi.EngineResponse, error) {
	var dynClient dynamic.Interface
	if dClient != nil {
		dynClient = dClient.GetDynamicInterface()
	}
	compiler := authzcompiler.NewCompiler[dynamic.Interface, *authv3.CheckRequest, *authv3.CheckResponse](dynClient)
	compiled, errs := compiler.Compile(vpol)
	if len(errs) > 0 {
		return engineapi.EngineResponse{}, fmt.Errorf("failed to compile envoy policy %s: %v", vpol.Name, errs.ToAggregate())
	}

	eng := core.NewEngine(
		core.MakeSource(compiled),
		handlers.Handler(
			dispatchers.Sequential(
				sdkpolicy.EvaluatorFactory[authzengine.EnvoyPolicy](),
				func(ctx context.Context, fc core.FactoryContext[authzengine.EnvoyPolicy, dynamic.Interface, *authv3.CheckRequest]) core.Breaker[authzengine.EnvoyPolicy, *authv3.CheckRequest, sdkpolicy.Evaluation[*authv3.CheckResponse]] {
					return core.MakeBreakerFunc(func(_ context.Context, _ authzengine.EnvoyPolicy, _ *authv3.CheckRequest, out sdkpolicy.Evaluation[*authv3.CheckResponse]) bool {
						return out.Result != nil
					})
				},
			),
			func(ctx context.Context, fc core.FactoryContext[authzengine.EnvoyPolicy, dynamic.Interface, *authv3.CheckRequest]) core.Resulter[authzengine.EnvoyPolicy, *authv3.CheckRequest, sdkpolicy.Evaluation[*authv3.CheckResponse], sdkpolicy.Evaluation[*authv3.CheckResponse]] {
				return resulters.NewFirst[authzengine.EnvoyPolicy, *authv3.CheckRequest](func(out sdkpolicy.Evaluation[*authv3.CheckResponse]) bool {
					return out.Result != nil || out.Error != nil
				})
			},
		),
	)

	evaluation := eng.Handle(context.TODO(), dynClient, request)

	var status engineapi.RuleStatus
	var message string

	if evaluation.Result == nil && evaluation.Error == nil {
		status = engineapi.RuleStatusSkip
		message = "request does not match"
	} else if evaluation.Result != nil {
		if ok := evaluation.Result.GetOkResponse(); ok != nil {
			status = engineapi.RuleStatusPass
			message = "request allowed"
		} else if denied := evaluation.Result.GetDeniedResponse(); denied != nil {
			status = engineapi.RuleStatusFail
			message = fmt.Sprintf("request denied with status code %d", denied.Status.Code)
		}
	} else if evaluation.Error != nil {
		status = engineapi.RuleStatusError
		message = evaluation.Error.Error()
	}

	resource := unstructured.Unstructured{Object: make(map[string]interface{})}
	response := engineapi.EngineResponse{
		Resource: resource,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.NewRuleResponse(vpol.Name, engineapi.Validation, message, status, nil),
			},
		},
	}
	response = response.WithPolicy(engineapi.NewValidatingPolicy(vpol))

	return response, nil
}

func processHTTPPolicy(vpol *policiesv1beta1.ValidatingPolicy, request *authzhttp.CheckRequest, dClient dclient.Interface) (engineapi.EngineResponse, error) {
	var dynClient dynamic.Interface
	if dClient != nil {
		dynClient = dClient.GetDynamicInterface()
	}
	compiler := authzcompiler.NewCompiler[dynamic.Interface, *authzhttp.CheckRequest, *authzhttp.CheckResponse](dynClient)
	compiled, errs := compiler.Compile(vpol)
	if len(errs) > 0 {
		return engineapi.EngineResponse{}, fmt.Errorf("failed to compile HTTP policy %s: %v", vpol.Name, errs.ToAggregate())
	}

	eng := core.NewEngine(
		core.MakeSource(compiled),
		handlers.Handler(
			dispatchers.Sequential(
				sdkpolicy.EvaluatorFactory[authzengine.HTTPPolicy](),
				func(ctx context.Context, fc core.FactoryContext[authzengine.HTTPPolicy, dynamic.Interface, *authzhttp.CheckRequest]) core.Breaker[authzengine.HTTPPolicy, *authzhttp.CheckRequest, sdkpolicy.Evaluation[*authzhttp.CheckResponse]] {
					return core.MakeBreakerFunc(func(_ context.Context, _ authzengine.HTTPPolicy, _ *authzhttp.CheckRequest, out sdkpolicy.Evaluation[*authzhttp.CheckResponse]) bool {
						return out.Result != nil
					})
				},
			),
			func(ctx context.Context, fc core.FactoryContext[authzengine.HTTPPolicy, dynamic.Interface, *authzhttp.CheckRequest]) core.Resulter[authzengine.HTTPPolicy, *authzhttp.CheckRequest, sdkpolicy.Evaluation[*authzhttp.CheckResponse], sdkpolicy.Evaluation[*authzhttp.CheckResponse]] {
				return resulters.NewFirst[authzengine.HTTPPolicy, *authzhttp.CheckRequest](func(out sdkpolicy.Evaluation[*authzhttp.CheckResponse]) bool {
					return out.Result != nil || out.Error != nil
				})
			},
		),
	)

	evaluation := eng.Handle(context.TODO(), dynClient, request)

	var status engineapi.RuleStatus
	var message string

	if evaluation.Result == nil && evaluation.Error == nil {
		status = engineapi.RuleStatusSkip
		message = "request does not match"
	} else if evaluation.Result != nil {
		if ok := evaluation.Result.Ok; ok != nil {
			status = engineapi.RuleStatusPass
			message = "request allowed"
		} else if denied := evaluation.Result.Denied; denied != nil {
			status = engineapi.RuleStatusFail
			message = fmt.Sprintf("request denied, reason: %s", denied.Reason)
		}
	} else if evaluation.Error != nil {
		status = engineapi.RuleStatusError
		message = evaluation.Error.Error()
	}

	resource := unstructured.Unstructured{Object: make(map[string]interface{})}
	response := engineapi.EngineResponse{
		Resource: resource,
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.NewRuleResponse(vpol.Name, engineapi.Validation, message, status, nil),
			},
		},
	}
	response = response.WithPolicy(engineapi.NewValidatingPolicy(vpol))

	return response, nil
}

func LoadEnvoyRequests(path string) (*authv3.CheckRequest, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read envoy payload file %s: %w", path, err)
	}

	var payload authv3.CheckRequest
	if err := protojson.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func LoadHTTPRequests(path string) (*authzhttp.CheckRequest, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP payload file %s: %w", path, err)
	}

	var p authzhttp.CheckRequest
	if err := json.Unmarshal(content, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
