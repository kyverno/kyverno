package imageverify

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/sdk/extensions/cel/libs/versions"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

const libraryName = "kyverno.imageverify"

// RuntimeKey is supplied by the evaluator, never stored in a reusable program.
const RuntimeKey = "__kyverno_imageverify"

var runtimeType = cel.ObjectType("imageverify.Runtime")

type lib struct{}

func Latest() *version.Version { return versions.KyvernoLatest }

// Lib declares stateless bindings. Public function syntax is preserved by macros
// which pass the current activation's runtime as an internal receiver.
func Lib() cel.EnvOption { return cel.Lib(&lib{}) }

func Types() []*apiservercel.DeclType            { return nil }
func (*lib) LibraryName() string                 { return libraryName }
func (*lib) ProgramOptions() []cel.ProgramOption { return nil }

func (*lib) CompileOptions() []cel.EnvOption {
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
