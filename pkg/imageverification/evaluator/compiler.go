package eval

import (
	"context"

	"github.com/google/cel-go/cel"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	ivpolvar "github.com/kyverno/kyverno/pkg/imageverification/variables"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
	"github.com/kyverno/sdk/cel/libs/hash"
	"github.com/kyverno/sdk/cel/libs/http"
	"github.com/kyverno/sdk/cel/libs/image"
	"github.com/kyverno/sdk/cel/libs/imagedata"
	"github.com/kyverno/sdk/cel/libs/json"
	"github.com/kyverno/sdk/cel/libs/math"
	"github.com/kyverno/sdk/cel/libs/random"
	"github.com/kyverno/sdk/cel/libs/resource"
	"github.com/kyverno/sdk/cel/libs/time"
	"github.com/kyverno/sdk/cel/libs/transform"
	"github.com/kyverno/sdk/cel/libs/user"
	"github.com/kyverno/sdk/cel/libs/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var ivpolCompilerVersion = version.MajorMinor(1, 0)

type Compiler interface {
	Compile(policiesv1beta1.ImageValidatingPolicyLike, []*policiesv1beta1.PolicyException) (CompiledPolicy, field.ErrorList)
}

func NewCompiler(ictx imagedataloader.ImageContext, lister k8scorev1.SecretInterface, reqGVR *metav1.GroupVersionResource) Compiler {
	return &compilerImpl{
		ictx:   ictx,
		lister: lister,
		reqGVR: reqGVR,
	}
}

type compilerImpl struct {
	ictx   imagedataloader.ImageContext
	lister k8scorev1.SecretInterface
	reqGVR *metav1.GroupVersionResource
}

func (c *compilerImpl) Compile(ivpolicy policiesv1beta1.ImageValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	ivpolEnvSet, variablesProvider, err := c.createBaseIvpolEnv(libs.GetLibsCtx(), ivpolicy)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	env, err := ivpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	spec := ivpolicy.GetSpec()
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := engine.CompileMatchConditions(path, env, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	matchImageEnv, err := engine.NewMatchImageEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	matchImageReferences, errs := engine.CompileMatchImageReferences(path.Child("matchImageReferences"), matchImageEnv, spec.MatchImageReferences...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	imageExtractors, errs := engine.CompileImageExtractors(path.Child("images"), env, c.reqGVR, spec.ImageExtractors...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	variables, errs := engine.CompileVariables(path.Child("variables"), env, variablesProvider, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	var compiledAttestors []*ivpolvar.CompiledAttestor
	{
		path := path.Child("attestors")
		compiledAttestors, errs = ivpolvar.CompileAttestors(path, spec.Attestors, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}

	validations := make([]engine.Validation, 0, len(spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range spec.Validations {
			path := path.Index(i)
			program, errs := engine.CompileValidation(path, env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	auditAnnotations := make(map[string]cel.Program, len(spec.AuditAnnotations))
	{
		path := path.Child("auditAnnotations")
		for i, auditAnnotation := range spec.AuditAnnotations {
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
		failurePolicy:        ivpolicy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()),
		matchConditions:      matchConditions,
		matchImageReferences: matchImageReferences,
		validations:          validations,
		auditAnnotations:     auditAnnotations,
		imageExtractors:      imageExtractors,
		attestors:            compiledAttestors,
		attestationList:      getAttestations(spec.Attestations),
		creds:                spec.Credentials,
		exceptions:           compiledExceptions,
		variables:            variables,
	}, nil
}

func (c *compilerImpl) createBaseIvpolEnv(libsctx libs.Context, ivpol policiesv1beta1.ImageValidatingPolicyLike) (*environment.EnvSet, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(engine.ResourceKey, resource.ContextType),
		cel.Variable(engine.ImagesKey, cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable(engine.AttestorsKey, cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable(engine.AttestationsKey, cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable(engine.ObjectKey, cel.DynType),
	)

	if ivpol.GetSpec().EvaluationMode() == policieskyvernoio.EvaluationModeKubernetes {
		baseOpts = append(baseOpts,
			cel.Variable(engine.RequestKey, engine.RequestType.CelType()),
			cel.Variable(engine.NamespaceObjectKey, engine.NamespaceType.CelType()),
			cel.Variable(engine.OldObjectKey, cel.DynType),
			cel.Variable(engine.VariablesKey, engine.VariablesType),
		)
	}

	base := environment.MustBaseEnvSet(ivpolCompilerVersion)
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, nil, err
	}

	variablesProvider := compiler.NewVariablesProvider(env.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, nil, err
	}

	baseOpts = append(baseOpts, declOptions...)

	extendedBase, err := base.Extend(
		environment.VersionedOptions{
			IntroducedVersion: ivpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libaries
		environment.VersionedOptions{
			IntroducedVersion: ivpolCompilerVersion,
			EnvOptions: []cel.EnvOption{
				globalcontext.Lib(
					globalcontext.Context{ContextInterface: libsctx},
					globalcontext.Latest(),
				),
				http.Lib(
					http.Context{ContextInterface: http.NewHTTP(nil)},
					http.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Context{ContextInterface: libsctx},
					imagedata.Latest(),
				),
				imageverify.Lib(
					imageverify.Latest(), c.ictx, ivpol, c.lister,
				),
				resource.Lib(
					resource.Context{ContextInterface: libsctx},
					ivpol.GetNamespace(),
					resource.Latest(),
				),
				user.Lib(
					user.Latest(),
				),
				math.Lib(
					math.Latest(),
				),
				hash.Lib(
					hash.Latest(),
				),
				json.Lib(
					&json.JsonImpl{},
					json.Latest(),
				),
				yaml.Lib(
					&yaml.YamlImpl{},
					yaml.Latest(),
				),
				random.Lib(
					random.Latest(),
				),
				time.Lib(
					time.Latest(),
				),
				transform.Lib(
					transform.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}

func getAttestations(att []v1beta1.Attestation) map[string]string {
	m := make(map[string]string)
	for _, v := range att {
		m[v.Name] = v.Name
	}
	return m
}
