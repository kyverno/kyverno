package compiler

import (
	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/sdk/extensions/cel/libs/image"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
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
func EnvOptionsForVersion(v *version.Version, options ...VersionedEnvOptions) []cel.EnvOption {
	if v == nil {
		return nil
	}
	result := make([]cel.EnvOption, 0)
	for _, option := range options {
		if option.IntroducedVersion == nil {
			continue
		}
		if !v.AtLeast(option.IntroducedVersion) {
			continue
		}
		if option.RemovedVersion != nil && !v.LessThan(option.RemovedVersion) {
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
		ext.Regex(ext.RegexVersion(1)),
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
						// Propagate a resource error (cost limit exceeded, guard
						// refusal) instead of absorbing it into the compat default --
						// otherwise a hazardous value could select the default branch
						// by being expensive rather than by being genuinely absent.
						// Type and presence errors, the idiom this macro exists for,
						// keep the v0.27-compatible absorb-to-default behavior.
						if IsResourceError(v) {
							return v
						}
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

// --- chokepoints ---
//
// NewEnv and BaseEnvSet are the two places CostLib gets attached to a *cel.Env or
// *environment.EnvSet. CostLib must be the LAST function-registering option: its
// env-capture (costlib.go's CompileOptions) only sees a complete env if nothing adds more
// functions afterward, and its binding registry (guards.go) needs every function on the
// env to wrap it. A caller that builds through NewEnv/BaseEnvSet and then calls a second
// env.Extend / EnvSet.Extend with more function-registering options breaks this silently --
// no error, just a function left unwrapped or a replacement tracker that never wins.

// NewEnv is the chokepoint for bare (non-apiserver) *cel.Env construction. opts is the
// caller's whole option list; CostLib is appended last.
func NewEnv(opts ...cel.EnvOption) (*cel.Env, error) {
	all := make([]cel.EnvOption, 0, len(opts)+1)
	all = append(all, opts...)
	all = append(all, CostLibOption()) // must stay last; see the chokepoint comment above
	return cel.NewEnv(all...)
}

// BaseEnvSet is the chokepoint for apiserver-based *environment.EnvSet construction. opts
// is the caller's whole VersionedOptions list, passed to a single Extend call alongside a
// final VersionedOptions carrying CostLib at IntroducedVersion 1.0, so it applies under
// environment.NewExpressions too (webhook matchConditions, exception validation) as well as
// environment.StoredExpressions.
func BaseEnvSet(v *version.Version, opts ...environment.VersionedOptions) (*environment.EnvSet, error) {
	base := environment.MustBaseEnvSet(v)
	all := make([]environment.VersionedOptions, 0, len(opts)+1)
	all = append(all, opts...)
	all = append(all, environment.VersionedOptions{
		IntroducedVersion: version.MajorMinor(1, 0),
		EnvOptions:        []cel.EnvOption{CostLibOption()},
	})
	return base.Extend(all...)
}

func NewBaseEnv() (*cel.Env, error) {
	return NewEnv(DefaultEnvOptions()...)
}

func NewMatchImageEnv() (*cel.Env, error) {
	opts := append(append([]cel.EnvOption{}, DefaultEnvOptions()...),
		cel.Variable(ImageRefKey, cel.StringType),
		image.Lib(KyvernoVersion),
	)
	return NewEnv(opts...)
}
