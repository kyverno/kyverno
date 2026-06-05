package compiler

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/sdk/extensions/cel/libs/image"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/library"
)

// breaking change history is stored inside the library structure. each policy compiler can pass a kyverno
// version from which it wants its libraries to be at. during a backport, you can set this to the version
// you are building.
var KyvernoVersion = version.MajorMinor(1, 18)

func DefaultEnvOptions() []cel.EnvOption {
	return defaultEnvOptionsWithHomogeneousAggregateEnforcement(true)
}

// defaultEnvOptionsWithHomogeneousAggregateEnforcement returns CEL environment
// options with optional homogeneous aggregate enforcement.
//
// Homogeneous aggregate enforcement is enabled for standard policy evaluation
// to preserve stricter CEL type-checking behavior.
//
// Dynamic resource generation environments (for example, GeneratingPolicy)
// may disable this enforcement to allow mixed-type aggregate literals.
func defaultEnvOptionsWithHomogeneousAggregateEnforcement(enforce bool) []cel.EnvOption {
	opts := []cel.EnvOption{
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
		cel.CrossTypeNumericComparisons(true),

		// register common libs
		cel.OptionalTypes(),
		ext.Bindings(),
		ext.Encoders(),

		// versions below match the kubernetes base env set behavior
		// (k8s.io/apiserver/pkg/cel/environment).
		// we moved away from using it directly, but we want to preserve
		// the same library versions.
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
// dynamic resource generation.
//
// This disables homogeneous aggregate enforcement to allow mixed-type
// aggregate literals used in dynamically generated resources.
func DynamicResourceEnvOptions() []cel.EnvOption {
	return defaultEnvOptionsWithHomogeneousAggregateEnforcement(false)
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
