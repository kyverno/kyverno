package compiler

import (
	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/sdk/extensions/cel/libs/image"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/library"
)

// breaking change history is stored inside the library structure. each policy compiler can pass a kyverno
// version from which it wants its libraries to be at. during a backport, you can set this to the version
// you are building.
var KyvernoVersion = version.MajorMinor(1, 18)

// VersionedEnvOptions groups CEL environment options by compiler compatibility versions.
// Options are enabled when `version` is >= IntroducedVersion and < RemovedVersion (if set).
type VersionedEnvOptions struct {
	IntroducedVersion *version.Version
	RemovedVersion    *version.Version
	EnvOptions        []cel.EnvOption
}

// EnvOptionsForVersion returns all CEL environment options applicable for the given
// compiler compatibility version.
func EnvOptionsForVersion(version *version.Version, options ...VersionedEnvOptions) []cel.EnvOption {
	result := make([]cel.EnvOption, 0)
	for _, option := range options {
		if option.IntroducedVersion == nil {
			continue
		}
		if !version.AtLeast(option.IntroducedVersion) {
			continue
		}
		if option.RemovedVersion != nil && !version.LessThan(option.RemovedVersion) {
			continue
		}
		result = append(result, option.EnvOptions...)
	}
	return result
}

func DefaultEnvOptions() []cel.EnvOption {
	return defaultEnvOptionsWithHomogeneousAggregateEnforcement(true)
}

// defaultEnvOptionsWithHomogeneousAggregateEnforcement returns CEL environment
// options with optional homogeneous aggregate enforcement.
func defaultEnvOptionsWithHomogeneousAggregateEnforcement(enforce bool) []cel.EnvOption {
	opts := []cel.EnvOption{
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
		cel.CrossTypeNumericComparisons(true),
		// register common libs
		cel.OptionalTypes(),
		ext.Bindings(),
		ext.Encoders(),
		// versions below match the kubernetes base env set behavior (k8s.io/apiserver/pkg/cel/environment).
		// we moved away from using it directly, but we want to preserve the same library versions.
		ext.Lists(ext.ListsVersion(3)),
		ext.Math(),
		ext.Protos(),
		ext.Sets(),
		ext.Strings(ext.StringsVersion(2)),
		// register kubernetes libs
		library.CIDR(),
		library.Format(),
		library.IP(),
		library.Lists(),
		library.Regex(),
		library.URLs(),
		library.Quantity(),
		library.SemverLib(library.SemverVersion(1)),
	}

	if enforce {
		opts = append([]cel.EnvOption{cel.HomogeneousAggregateLiterals()}, opts...)
	}
	return opts
}

// DynamicResourceEnvOptions returns CEL environment options suitable for
// dynamic resource generation where mixed-type aggregate literals are allowed.
func DynamicResourceEnvOptions() []cel.EnvOption {
	return defaultEnvOptionsWithHomogeneousAggregateEnforcement(false)
}

// DefaultEnvOptionsWithCompat returns DefaultEnvOptions() extended with backward-compat
// options for cel-go v0.28 behaviour changes (see OrValueCompatEnvOptions).
func DefaultEnvOptionsWithCompat() []cel.EnvOption {
	return append(DefaultEnvOptions(), OrValueCompatEnvOptions()...)
}

// DynamicResourceEnvOptionsWithCompat returns DynamicResourceEnvOptions()
// extended with backward-compat options for cel-go v0.28 behaviour changes.
func DynamicResourceEnvOptionsWithCompat() []cel.EnvOption {
	return append(DynamicResourceEnvOptions(), OrValueCompatEnvOptions()...)
}

// OrValueCompatEnvOptions returns the EnvOptions that restore cel-go v0.27 behaviour
// for orValue called on non-optional (concrete) LHS values. In v0.28 this now returns
// NoSuchOverloadErr; the compat macro transparently routes such calls through
// __orValueCompat__ which returns the concrete value as-is.
func OrValueCompatEnvOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Backward-compat macro: cel-go v0.28 changed orValue to return
		// NoSuchOverloadErr for non-optional LHS values, whereas v0.27 silently
		// returned the concrete value unchanged. Policies that call .orValue() on a
		// non-optional field access (e.g. container.volumeMounts.orValue([])) would
		// start producing evaluation errors after the cel-go upgrade. This macro
		// transforms x.orValue(y) at parse time to __orValueCompat__(x, y), which
		// is backed by a function that restores the pre-0.28 behaviour.
		cel.Macros(cel.ReceiverMacro("orValue", 1, orValueCompatMacro)),
		cel.Function("__orValueCompat__",
			cel.Overload("__orValueCompat___dyn_dyn",
				[]*cel.Type{cel.DynType, cel.DynType},
				cel.DynType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					switch v := lhs.(type) {
					case *types.Optional:
						if v.HasValue() {
							return v.GetValue()
						}
						return rhs
					case *types.Err:
						return rhs
					default:
						// Concrete non-optional value: return it (v0.27 compatible behaviour).
						return lhs
					}
				}),
			),
		),
	}
}

// orValueCompatMacro rewrites x.orValue(y) → __orValueCompat__(x, y) at parse time.
// This routes all orValue calls through our compat function instead of cel-go's
// evalOptionalOrValue, which in v0.28 returns NoSuchOverloadErr for non-optional LHS.
func orValueCompatMacro(eh cel.MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *cel.Error) {
	return eh.NewCall("__orValueCompat__", target, args[0]), nil
}

func NewBaseEnv() (*cel.Env, error) {
	// create new cel env
	return cel.NewEnv(
		DefaultEnvOptions()...,
	)
}

func NewMatchImageEnv() (*cel.Env, error) {
	base, err := NewBaseEnv()
	if err != nil {
		return nil, err
	}
	return base.Extend(
		cel.Variable(ImageRefKey, cel.StringType),
		image.Lib(KyvernoVersion),
	)
}
