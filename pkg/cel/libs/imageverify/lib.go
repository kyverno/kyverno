package imageverify

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/cel/libs/versions"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const libraryName = "kyverno.imageverify"

// RequestContextKey carries the evaluation context in the CEL activation.
const RequestContextKey = "__kyverno_imageverify_context"

var evaluationContextType = cel.ObjectType("imageverify.EvaluationContext")

// EvaluationContext is an opaque receiver for image verification callbacks.
// Its context is supplied at evaluation time, never retained by compiled bindings.
type EvaluationContext struct {
	ctx    context.Context //nolint:containedctx
	images imagedataloader.ImageContext
}

// NewEvaluationContext binds image verification work to the current evaluation.
func NewEvaluationContext(ctx context.Context, images ...imagedataloader.ImageContext) EvaluationContext {
	request := EvaluationContext{ctx: ctx}
	if len(images) > 0 {
		request.images = images[0]
	}
	return request
}

type lib struct {
	logger        logr.Logger
	version       *version.Version
	imgCtx        imagedataloader.ImageContext
	ivpol         policiesv1beta1.ImageValidatingPolicyLike
	lister        corev1listers.SecretLister
	ivCache       imageverifycache.Client
	verifications *ImageVerificationResults
}

func Latest() *version.Version {
	return versions.KyvernoLatest
}

// Lib builds the image verification CEL library. The verification results are shared
// with the caller, which reads them back after evaluation to enforce
// validationConfigurations.required; pass nil when that enforcement is not needed.
func Lib(v *version.Version, imgCtx imagedataloader.ImageContext, ivpol policiesv1beta1.ImageValidatingPolicyLike, lister corev1listers.SecretLister, logger logr.Logger, ivCache imageverifycache.Client, verifications *ImageVerificationResults) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		logger:        logger,
		version:       v,
		imgCtx:        imgCtx,
		ivpol:         ivpol,
		lister:        lister,
		ivCache:       ivCache,
		verifications: verifications,
	})
}

func Types() []*apiservercel.DeclType {
	return []*apiservercel.DeclType{}
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		c.extendEnv,
	}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	// Standalone Lib users can still evaluate existing expressions without
	// providing the internal receiver. Evaluator activation data overrides it.
	return []cel.ProgramOption{
		cel.Globals(map[string]any{RequestContextKey: NewEvaluationContext(context.Background())}),
	}
}

func (c *lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	impl, err := ImageVerifyCELFuncs(c.logger, c.imgCtx, c.ivpol, c.lister, c.ivCache, env.CELTypeAdapter(), c.verifications)
	if err != nil {
		return nil, err
	}
	functions := []struct {
		name   string
		args   []*cel.Type
		result *cel.Type
		call   func(*ivfuncs, []ref.Val) ref.Val
	}{
		{
			"verifyImageSignatures",
			[]*cel.Type{cel.StringType, cel.ListType(cel.DynType)},
			cel.IntType,
			func(f *ivfuncs, args []ref.Val) ref.Val {
				return f.verify_image_signature_string_stringarray(args[0], args[1])
			},
		},
		{
			"verifyAttestationSignatures",
			[]*cel.Type{cel.StringType, cel.StringType, cel.ListType(cel.DynType)},
			cel.IntType,
			func(f *ivfuncs, args []ref.Val) ref.Val {
				return f.verify_image_attestations_string_string_stringarray(args...)
			},
		},
		{
			"getImageData",
			[]*cel.Type{cel.StringType},
			cel.DynType,
			func(f *ivfuncs, args []ref.Val) ref.Val { return f.get_image_data_string(args[0]) },
		},
		{
			"extractPayload",
			[]*cel.Type{cel.StringType, cel.StringType},
			cel.DynType,
			func(f *ivfuncs, args []ref.Val) ref.Val { return f.payload_string_string(args[0], args[1]) },
		},
	}
	options := []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[EvaluationContext]()),
		cel.Variable(RequestContextKey, evaluationContextType),
	}
	for _, fn := range functions {
		internalName := "__" + fn.name
		options = append(options,
			// Preserve public function syntax while passing the current activation's context.
			cel.Macros(cel.GlobalMacro(fn.name, len(fn.args), func(e cel.MacroExprFactory, _ ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
				return e.NewMemberCall(internalName, e.NewIdent(RequestContextKey), args...), nil
			})),
			cel.Function(internalName, cel.MemberOverload(internalName+"_context", append([]*cel.Type{evaluationContextType}, fn.args...), fn.result,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					request, err := utils.ConvertToNative[EvaluationContext](args[0])
					if err != nil {
						return types.WrapErr(err)
					}
					if request.ctx == nil {
						return types.NewErr("missing image verification evaluation context")
					}
					// The release compiler already owns per-admission results and caches.
					// Copy only the callback state to attach context without mutating a
					// compiled closure or losing deferred payloads between function calls.
					callback := *impl
					callback.requestCtx = request.ctx
					if request.images != nil {
						callback.imgCtx = request.images
					}
					return fn.call(&callback, args[1:])
				}))),
		)
	}
	return env.Extend(options...)
}
