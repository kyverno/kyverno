package evaluator

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type evaluationRequestMarker struct{}

type evaluationImageContext struct {
	imagedataloader.ImageContext
	contexts []context.Context
	cancel   context.CancelFunc
}

func (c *evaluationImageContext) Get(ctx context.Context, _ string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	c.contexts = append(c.contexts, ctx)
	if c.cancel != nil {
		c.cancel()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("fetch stopped by test")
}

func TestImageFunctionsUseEvaluationContext(t *testing.T) {
	for _, testCase := range []struct {
		phase    string
		workload bool
		custom   bool
	}{
		{phase: "validation"},
		{phase: "match"},
		{phase: "mutation match"},
		{phase: "validation", workload: true},
		{phase: "match", workload: true},
		{phase: "mutation match", workload: true},
		{phase: "validation", workload: true, custom: true},
		{phase: "match", workload: true, custom: true},
		{phase: "mutation match", workload: true, custom: true},
	} {
		t.Run(testCase.phase+"/workload="+strconv.FormatBool(testCase.workload)+"/custom="+strconv.FormatBool(testCase.custom), func(t *testing.T) {
			for _, cancelDuringFetch := range []bool{false, true} {
				name := "deadline and values"
				if cancelDuringFetch {
					name = "cancel during fetch"
				}
				t.Run(name, func(t *testing.T) {
					policy := ivpol.DeepCopy()
					policy.Spec.ImageExtractors = nil
					policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
					const image = "93.184.216.34/app:latest"
					resource := map[string]any{}
					expression := "getImageData(" + strconv.Quote(image) + ") != null"
					if testCase.workload {
						resource["spec"] = map[string]any{"containers": []any{map[string]any{"image": image}}}
					}
					if testCase.custom {
						resource = obj(image)
						policy.Spec.ImageExtractors = []policiesv1beta1.ImageExtractor{{Name: "custom", Expression: "[object.foo.bar]"}}
					}
					if testCase.phase == "validation" {
						policy.Spec.Validations[0].Expression = expression
					} else {
						policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "image-data", Expression: expression}}
					}
					author := &evaluationImageContext{}
					workload := &evaluationImageContext{}
					images := imageverify.WithResourceImageContextFactory(author, func() (imagedataloader.ImageContext, error) {
						return workload, nil
					})
					selected, unused := author, workload
					if testCase.workload {
						selected, unused = workload, author
					}
					compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
					require.Empty(t, errs)
					ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), evaluationRequestMarker{}, testCase.phase), time.Minute)
					defer cancel()
					if cancelDuringFetch {
						selected.cancel = cancel
					}
					var err error
					if testCase.phase == "mutation match" {
						_, err = compiled.MutateDigest(ctx, images, nil, resource, nil, unstructured.Unstructured{}, nil, nil, libs.NewFakeContextProvider())
					} else {
						_, err = compiled.Evaluate(ctx, images, nil, resource, nil, false, nil, libs.NewFakeContextProvider())
					}
					require.Error(t, err)
					require.Empty(t, unused.contexts)
					require.Len(t, selected.contexts, 1)
					require.Equal(t, ctx, selected.contexts[0], "the CEL callback must receive the evaluation context")
					require.Equal(t, testCase.phase, selected.contexts[0].Value(evaluationRequestMarker{}))
					deadline, _ := ctx.Deadline()
					observed, ok := selected.contexts[0].Deadline()
					require.True(t, ok)
					require.Equal(t, deadline, observed)
					if cancelDuringFetch {
						require.ErrorContains(t, err, context.Canceled.Error())
					}
				})
			}
		})
	}
}

func TestMutateDigestUsesRuntimeMetadataProvider(test *testing.T) {
	previous := libs.LibraryContext
	test.Cleanup(func() { libs.LibraryContext = previous })
	compileProvider := &metadataScopeContext{Context: libs.NewFakeContextProvider()}
	libs.LibraryContext = compileProvider
	const image = "169.254.169.254/author:tag"
	policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		MatchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "runtime-provider",
			Expression: "image.GetMetadata(" + strconv.Quote(image) + ") == null",
		}},
		Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
	}}
	images := &proofImageContext{}
	compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
	require.Empty(test, errs)
	first := &metadataScopeContext{Context: libs.NewFakeContextProvider()}
	second := &metadataScopeContext{Context: libs.NewFakeContextProvider()}
	for _, provider := range []*metadataScopeContext{first, second} {
		patches, err := compiled.MutateDigest(context.Background(), images, nil, obj(image), nil, unstructured.Unstructured{}, nil, nil, provider)
		require.NoError(test, err)
		require.Empty(test, patches)
	}
	require.Empty(test, compileProvider.lookups)
	require.Equal(test, []string{image}, first.lookups)
	require.Equal(test, []string{image}, second.lookups)
	require.Empty(test, images.images)
}
