package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

type Compiler interface {
	Compile(policy *policiesv1alpha1.DeletingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.DeletingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	if policy == nil {
		return nil, field.ErrorList{field.Required(field.NewPath("policy"), "policy must not be nil")}
	}
	var allErrs field.ErrorList
	base, err := compiler.NewBaseEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, compiler.NamespaceType, compiler.RequestType)
	options := []cel.EnvOption{
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
		cel.Variable(compiler.ImagesKey, image.ImageType),
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	}
	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	variablesProvider := compiler.NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(declTypes...)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		// TODO: proper error handling
		panic(err)
	}
	options = append(options, declOptions...)
	options = append(options, globalcontext.Lib(), http.Lib(), image.Lib(), imagedata.Lib(), resource.Lib())
	// TODO: params, authorizer, authorizer.requestResource ?
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	conditions := make([]cel.Program, 0, len(policy.Spec.Conditions))
	{
		path := path.Child("conditions")
		programs, errs := compiler.CompileMatchConditions(path, env, policy.Spec.Conditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		conditions = append(conditions, programs...)
	}
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, policy.Spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	// exceptions' match conditions
	compiledExceptions := make([]compiler.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), env, polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, compiler.Exception{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}
	return &Policy{
		deletionPropagationPolicy: policy.Spec.DeletionPropagationPolicy,
		schedule:                  policy.Spec.Schedule,
		conditions:                conditions,
		variables:                 variables,
		exceptions:                compiledExceptions,
	}, nil
}
