package imageverify

import (
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

// RuntimeKey selects the evaluation runtime. The evaluator overrides the
// standalone library default with a private instance bound to its request.
const RuntimeKey = "__kyverno_imageverify"

var runtimeType = cel.ObjectType("imageverify.Runtime")

type lib struct {
	logger         logr.Logger
	version        *version.Version
	imgCtx         imagedataloader.ImageContext
	ivpol          policiesv1beta1.ImageValidatingPolicyLike
	lister         corev1listers.SecretLister
	ivCache        imageverifycache.Client
	verifications  *ImageVerificationResults
	defaultRuntime Runtime
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

func Types() []*apiservercel.DeclType { return nil }
func (*lib) LibraryName() string      { return libraryName }

func (c *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{cel.Globals(map[string]any{RuntimeKey: c.defaultRuntime})}
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{func(env *cel.Env) (*cel.Env, error) {
		functions, err := ImageVerifyCELFuncs(c.logger, c.imgCtx, c.ivpol, c.lister, c.ivCache, env.CELTypeAdapter(), c.verifications)
		if err != nil {
			return nil, err
		}
		c.defaultRuntime = NewRuntimeForPolicy(functions, c.imgCtx, c.ivCache, c.verifications)
		return env.Extend(runtimeOptions()...)
	}}
}

func runtimeOptions() []cel.EnvOption {
	// Preserve public function syntax while resolving the receiver from activation.
	functions := []struct {
		name   string
		args   []*cel.Type
		result *cel.Type
		call   func(*IvFuncs, []ref.Val) ref.Val
	}{
		{
			"verifyImageSignatures",
			[]*cel.Type{cel.StringType, cel.ListType(cel.DynType)},
			cel.IntType,
			func(f *IvFuncs, args []ref.Val) ref.Val {
				return f.verify_image_signature_string_stringarray(args[0], args[1])
			},
		},
		{
			"verifyAttestationSignatures",
			[]*cel.Type{cel.StringType, cel.StringType, cel.ListType(cel.DynType)},
			cel.IntType,
			func(f *IvFuncs, args []ref.Val) ref.Val {
				return f.verify_image_attestations_string_string_stringarray(args...)
			},
		},
		{
			"getImageData",
			[]*cel.Type{cel.StringType},
			cel.DynType,
			func(f *IvFuncs, args []ref.Val) ref.Val { return f.get_image_data_string(args[0]) },
		},
		{
			"extractPayload",
			[]*cel.Type{cel.StringType, cel.StringType},
			cel.DynType,
			func(f *IvFuncs, args []ref.Val) ref.Val { return f.payload_string_string(args[0], args[1]) },
		},
	}
	options := make([]cel.EnvOption, 0, 2+2*len(functions))
	options = append(options,
		ext.NativeTypes(reflect.TypeFor[Runtime]()),
		cel.Variable(RuntimeKey, runtimeType),
	)
	for _, fn := range functions {
		internalName := "__" + fn.name
		options = append(options,
			cel.Macros(cel.GlobalMacro(fn.name, len(fn.args), func(e cel.MacroExprFactory, _ ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
				return e.NewMemberCall(internalName, e.NewIdent(RuntimeKey), args...), nil
			})),
			cel.Function(internalName, cel.MemberOverload(internalName+"_runtime", append([]*cel.Type{runtimeType}, fn.args...), fn.result,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					r, err := utils.ConvertToNative[Runtime](args[0])
					if err != nil {
						return types.WrapErr(err)
					}
					if r.functions == nil {
						return types.NewErr("missing image verification runtime")
					}
					return fn.call(r.functions, args[1:])
				}))),
		)
	}
	return options
}
