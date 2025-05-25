package eval

import (
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	ivpolvar "github.com/kyverno/kyverno/pkg/imageverification/variables"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type Compiler interface {
	Compile(*policiesv1alpha1.ImageValidatingPolicy, []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList)
}

func NewCompiler(ictx imagedataloader.ImageContext, lister k8scorev1.SecretInterface, reqGVR *metav1.GroupVersionResource) Compiler {
	return &compiler{
		ictx:   ictx,
		lister: lister,
		reqGVR: reqGVR,
	}
}

type compiler struct {
	ictx   imagedataloader.ImageContext
	lister k8scorev1.SecretInterface
	reqGVR *metav1.GroupVersionResource
}

func (c *compiler) Compile(ivpolicy *policiesv1alpha1.ImageValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewBaseEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, imageverify.Types()...)
	options := []cel.EnvOption{
		cel.Variable(engine.ResourceKey, resource.ContextType),
		cel.Variable(engine.HttpKey, http.ContextType),
		cel.Variable(engine.ImagesKey, cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable(engine.AttestorsKey, cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable(engine.AttestationsKey, cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable(engine.ImageDataKey, imagedata.ContextType),
	}

	if ivpolicy.Spec.EvaluationMode() == policiesv1alpha1.EvaluationModeKubernetes {
		options = append(options, cel.Variable(engine.RequestKey, engine.RequestType.CelType()))
		options = append(options, cel.Variable(engine.NamespaceObjectKey, engine.NamespaceType.CelType()))
		options = append(options, cel.Variable(engine.ObjectKey, cel.DynType))
		options = append(options, cel.Variable(engine.OldObjectKey, cel.DynType))
		options = append(options, cel.Variable(engine.VariablesKey, engine.VariablesType))
		options = append(options, cel.Variable(engine.GlobalContextKey, globalcontext.ContextType))
	} else {
		options = append(options, cel.Variable(engine.ObjectKey, cel.DynType))
	}

	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	variablesProvider := engine.NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(declTypes...)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	options = append(options, declOptions...)
	options = append(options, globalcontext.Lib(), http.Lib(), image.Lib(), imagedata.Lib(), imageverify.Lib(c.ictx, ivpolicy, c.lister), resource.Lib(), user.Lib())
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(ivpolicy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := engine.CompileMatchConditions(path, env, ivpolicy.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	matchImageEnv, err := engine.NewMatchImageEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	matchImageReferences, errs := engine.CompileMatchImageReferences(path.Child("matchImageReferences"), matchImageEnv, ivpolicy.Spec.MatchImageReferences...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	imageExtractors, errs := engine.CompileImageExtractors(path.Child("images"), env, c.reqGVR, ivpolicy.Spec.ImageExtractors...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	variables, errs := engine.CompileVariables(path.Child("variables"), env, variablesProvider, ivpolicy.Spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	var compiledAttestors []*ivpolvar.CompiledAttestor
	{
		path := path.Child("attestors")
		compiledAttestors, errs = ivpolvar.CompileAttestors(path, ivpolicy.Spec.Attestors, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}

	validations := make([]engine.Validation, 0, len(ivpolicy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range ivpolicy.Spec.Validations {
			path := path.Index(i)
			program, errs := engine.CompileValidation(path, env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	auditAnnotations := make(map[string]cel.Program, len(ivpolicy.Spec.AuditAnnotations))
	{
		path := path.Child("auditAnnotations")
		for i, auditAnnotation := range ivpolicy.Spec.AuditAnnotations {
			path := path.Index(i)
			program, errs := engine.CompileAuditAnnotation(path, env, auditAnnotation)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			auditAnnotations[auditAnnotation.Key] = program
		}
	}

	compiledExceptions := make([]engine.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := engine.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), env, polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, engine.Exception{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}

	if len(allErrs) > 0 {
		return nil, allErrs
	}

	return &compiledPolicy{
		failurePolicy:        ivpolicy.GetFailurePolicy(),
		matchConditions:      matchConditions,
		matchImageReferences: matchImageReferences,
		validations:          validations,
		auditAnnotations:     auditAnnotations,
		imageExtractors:      imageExtractors,
		attestors:            compiledAttestors,
		attestationList:      getAttestations(ivpolicy.Spec.Attestations),
		creds:                ivpolicy.Spec.Credentials,
		exceptions:           compiledExceptions,
		variables:            variables,
	}, nil
}

func getAttestations(att []v1alpha1.Attestation) map[string]string {
	m := make(map[string]string)
	for _, v := range att {
		m[v.Name] = v.Name
	}
	return m
}
