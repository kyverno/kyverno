package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/client-go/kubernetes"
)

type engineImpl struct {
	provider   Provider
	client     kubernetes.Interface
	nsResolver engine.NamespaceResolver
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, client kubernetes.Interface) *engineImpl {
	return &engineImpl{
		provider:   provider,
		nsResolver: nsResolver,
		client:     client,
	}
}

func (e *engineImpl) Handle(ctx context.Context, request engine.EngineRequest) (engine.EngineResponse, error) {
	var response engine.EngineResponse
	mpols, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, err
	}

	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	dryRun := false
	if request.Request.DryRun != nil {
		dryRun = *request.Request.DryRun
	}

	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.Request.Kind),
		request.Request.Namespace,
		request.Request.Name,
		schema.GroupVersionResource(request.Request.Resource),
		request.Request.SubResource,
		admission.Operation(request.Request.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)
	typeConverter := patch.NewTypeConverterManager(nil, e.client.Discovery().OpenAPIV3())
	for _, mpol := range mpols {
		mpol.CompiledPolicy.Evaluate(ctx, attr, schema.GroupVersionResource(request.Request.Resource), nil, typeConverter)
	}
	return response, nil
}
