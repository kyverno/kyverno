package eval

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/match"
	"github.com/kyverno/kyverno/pkg/imageverification/variables"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	AttestationKey     = "attestations"
	AttestorKey        = "attestors"
	GlobalContextKey   = "globalcontext"
	HttpKey            = "http"
	ImagesKey          = "images"
	NamespaceObjectKey = "namespaceObject"
	ObjectKey          = "object"
	OldObjectKey       = "oldObject"
	RequestKey         = "request"
	ResourceKey        = "resource"
)

type Compiler interface {
	Compile(*policiesv1alpha1.ImageValidatingPolicy) (CompiledPolicy, field.ErrorList)
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

func (c *compiler) Compile(ivpolicy *policiesv1alpha1.ImageValidatingPolicy) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, imageverify.Types()...)
	options := []cel.EnvOption{
		cel.Variable(ResourceKey, resource.ContextType),
		cel.Variable(GlobalContextKey, globalcontext.ContextType),
		cel.Variable(HttpKey, http.HTTPType),
		cel.Variable(ImagesKey, cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable(AttestorKey, cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable(AttestationKey, cel.MapType(cel.StringType, cel.StringType)),
	}

	if ivpolicy.Spec.EvaluationMode() == policiesv1alpha1.EvaluationModeKubernetes {
		options = append(options, cel.Variable(RequestKey, policy.RequestType.CelType()))
		options = append(options, cel.Variable(NamespaceObjectKey, policy.NamespaceType.CelType()))
		options = append(options, cel.Variable(ObjectKey, cel.DynType))
		options = append(options, cel.Variable(OldObjectKey, cel.DynType))
	} else {
		options = append(options, cel.Variable(ObjectKey, cel.DynType))
	}

	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	options = append(options, globalcontext.Lib(), http.Lib(), imagedata.Lib(), imageverify.Lib(c.ictx, ivpolicy, c.lister), resource.Lib(), user.Lib())
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(ivpolicy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := policy.CompileMatchConditions(path, ivpolicy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	imageRules, errs := match.CompileMatches(path.Child("imageRules"), ivpolicy.Spec.ImageRules)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	imageExtractors, errs := variables.CompileImageExtractors(path.Child("images"), ivpolicy.Spec.Images, c.reqGVR, options)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	verifications := make([]policy.CompiledValidation, 0, len(ivpolicy.Spec.Validations))
	{
		path := path.Child("verifications")
		for i, rule := range ivpolicy.Spec.Validations {
			path := path.Index(i)
			program, errs := policy.CompileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			verifications = append(verifications, program)
		}
	}

	return &compiledPolicy{
		failurePolicy:   ivpolicy.GetFailurePolicy(),
		matchConditions: matchConditions,
		imageRules:      imageRules,
		verifications:   verifications,
		imageExtractors: imageExtractors,
		attestorList:    variables.GetAttestors(ivpolicy.Spec.Attestors),
		attestationList: variables.GetAttestations(ivpolicy.Spec.Attestations),
		creds:           ivpolicy.Spec.Credentials,
	}, nil
}
