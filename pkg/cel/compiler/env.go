package compiler

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/sdk/cel/libs/image"
	"k8s.io/apiserver/pkg/cel/library"
)

func DefaultEnvOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.HomogeneousAggregateLiterals(),
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
		cel.CrossTypeNumericComparisons(true),
		// register common libs
		cel.OptionalTypes(),
		ext.Bindings(),
		ext.Encoders(),
		ext.Lists(),
		ext.Math(),
		ext.Protos(),
		ext.Sets(),
		ext.Strings(),
		// register kubernetes libs
		library.CIDR(),
		library.Format(),
		library.IP(),
		library.Lists(),
		library.Regex(),
		library.URLs(),
		library.Quantity(),
		library.SemverLib(),
	}
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
		image.Lib(image.Latest()),
	)
}

// NewIdentityExprEnv creates a CEL environment suitable for evaluating
// identity field expressions (subject, subjectRegExp). The environment
// includes the "image" variable (string) so expressions can reference
// the image being verified.
//
// Example expression:
//
//	'"https://github.com/" + image.split("/")[1] + "/.github/workflows/release.yml@refs/heads/main"'
func NewIdentityExprEnv() (*cel.Env, error) {
	base, err := NewBaseEnv()
	if err != nil {
		return nil, err
	}
	return base.Extend(
		cel.Variable(ImageKey, cel.StringType),
	)
}
