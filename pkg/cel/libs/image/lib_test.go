package image_test

import (
	"regexp"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testImageLib(t *testing.T, expr string, expectResult ref.Val, expectRuntimeErrPattern string, expectCompileErrs []string) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		image.Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	if err != nil {
		t.Fatalf("%v", err)
	}
	compiled, issues := env.Compile(expr)

	if len(expectCompileErrs) > 0 {
		missingCompileErrs := []string{}
		matchedCompileErrs := sets.New[int]()
		for _, expectedCompileErr := range expectCompileErrs {
			compiledPattern, err := regexp.Compile(expectedCompileErr)
			if err != nil {
				t.Fatalf("failed to compile expected err regex: %v", err)
			}

			didMatch := false

			for i, compileError := range issues.Errors() {
				if compiledPattern.Match([]byte(compileError.Message)) {
					didMatch = true
					matchedCompileErrs.Insert(i)
				}
			}

			if !didMatch {
				missingCompileErrs = append(missingCompileErrs, expectedCompileErr)
			} else if len(matchedCompileErrs) != len(issues.Errors()) {
				unmatchedErrs := []cel.Error{}
				for i, issue := range issues.Errors() {
					if !matchedCompileErrs.Has(i) {
						unmatchedErrs = append(unmatchedErrs, *issue)
					}
				}
				require.Empty(t, unmatchedErrs, "unexpected compilation errors")
			}
		}

		require.Empty(t, missingCompileErrs, "expected compilation errors")
		return
	} else if len(issues.Errors()) > 0 {
		for _, err := range issues.Errors() {
			t.Errorf("unexpected compile error: %v", err)
		}
		t.FailNow()
	}

	prog, err := env.Program(compiled)
	if err != nil {
		t.Fatalf("%v", err)
	}
	res, _, err := prog.Eval(map[string]any{})
	if len(expectRuntimeErrPattern) > 0 {
		if err == nil {
			t.Fatalf("no runtime error thrown. Expected: %v", expectRuntimeErrPattern)
		} else if matched, regexErr := regexp.MatchString(expectRuntimeErrPattern, err.Error()); regexErr != nil {
			t.Fatalf("failed to compile expected err regex: %v", regexErr)
		} else if !matched {
			t.Fatalf("unexpected err: %v", err)
		}
	} else if err != nil {
		t.Fatalf("%v", err)
	} else if expectResult != nil {
		converted := res.Equal(expectResult).Value().(bool)
		require.True(t, converted, "expectation not equal to output")
	} else {
		t.Fatal("expected result must not be nil")
	}
}

func TestImage(t *testing.T) {
	cases := []struct {
		name               string
		expr               string
		expectValue        ref.Val
		expectedCompileErr []string
		expectedRuntimeErr string
	}{
		{
			name:        "parse",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:latest")`,
			expectValue: image.Image{Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest")},
		},
		{
			name:               "parse_invalid_image",
			expr:               `parseImageReference("registry.k8s.io/kube-apiserver-arm64:@")`,
			expectedRuntimeErr: "could not parse reference: registry.k8s.io/kube-apiserver-arm64:@",
		},
		{
			name:        "isImage",
			expr:        `isImage("registry.k8s.io/kube-apiserver-arm64:latest")`,
			expectValue: types.True,
		},
		{
			name:        "isImage_false",
			expr:        `isImage("registry.k8s.io/kube-apiserver-arm64:@")`,
			expectValue: types.False,
		},
		{
			name:               "isImage_noOverload",
			expr:               `isImage(0)`,
			expectedCompileErr: []string{"found no matching overload for 'isImage' applied to.*"},
		},
		{
			name:        "contains_digest_no_identifier",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").containsDigest()`,
			expectValue: types.False,
		},
		{
			name:        "contains_digest_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:latest").containsDigest()`,
			expectValue: types.False,
		},
		{
			name:        "contains_digest_true",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").containsDigest()`,
			expectValue: types.True,
		},
		{
			name:        "contains_digest_with_tag_true",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:latest@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").containsDigest()`,
			expectValue: types.True,
		},
		{
			name:        "registry",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").registry() == "registry.k8s.io"`,
			expectValue: types.True,
		},
		{
			name:        "registry_matches",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").registry().matches("(registry.k8s.io|ghcr.io)")`,
			expectValue: types.True,
		},
		{
			name:        "repository",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").repository() == "kube-apiserver-arm64"`,
			expectValue: types.True,
		},
		{
			name:        "identifier_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:testtag").identifier()`,
			expectValue: types.String("testtag"),
		},
		{
			name:        "default_identifier",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").identifier()`,
			expectValue: types.String("latest"),
		},
		{
			name:        "identifer_digest",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").identifier()`,
			expectValue: types.String("sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2"),
		},
		{
			name:        "identifer_digest_and_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:latest@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").identifier()`,
			expectValue: types.String("sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2"),
		},
		{
			name:        "tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:testtag").tag()`,
			expectValue: types.String("testtag"),
		},
		{
			name:        "default_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").tag()`,
			expectValue: types.String("latest"),
		},
		{
			name:        "no_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").tag()`,
			expectValue: types.String(""),
		},
		{
			name:        "identifier_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:testtag").identifier()`,
			expectValue: types.String("testtag"),
		},
		{
			name:        "no_digest",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64").digest()`,
			expectValue: types.String(""),
		},
		{
			name:        "digest_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:testtag").digest()`,
			expectValue: types.String(""),
		},
		{
			name:        "digest",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").digest()`,
			expectValue: types.String("sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2"),
		},
		{
			name:        "digest_digest_and_tag",
			expr:        `parseImageReference("registry.k8s.io/kube-apiserver-arm64:latest@sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2").digest() == "sha256:6aefddb645ee6963afd681b1845c661d0ea4c3b20ab9db86d9e753b203d385f2"`,
			expectValue: types.True,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testImageLib(t, c.expr, c.expectValue, c.expectedRuntimeErr, c.expectedCompileErr)
		})
	}
}
