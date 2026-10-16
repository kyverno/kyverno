package evaluator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/cel/libs"
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
	for _, phase := range []string{"validation", "match", "mutation match"} {
		t.Run(phase, func(t *testing.T) {
			for _, cancelDuringFetch := range []bool{false, true} {
				name := "deadline and values"
				if cancelDuringFetch {
					name = "cancel during fetch"
				}
				t.Run(name, func(t *testing.T) {
					policy := ivpol.DeepCopy()
					policy.Spec.ImageExtractors = nil
					policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
					const expression = `getImageData("registry.example/app:latest") != null`
					if phase == "validation" {
						policy.Spec.Validations[0].Expression = expression
					} else {
						policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "image-data", Expression: expression}}
					}
					images := &evaluationImageContext{}
					compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
					require.Empty(t, errs)
					ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), evaluationRequestMarker{}, phase), time.Minute)
					defer cancel()
					if cancelDuringFetch {
						images.cancel = cancel
					}
					var err error
					if phase == "mutation match" {
						_, err = compiled.MutateDigest(ctx, images, nil, map[string]any{}, nil, unstructured.Unstructured{}, nil, nil)
					} else {
						_, err = compiled.Evaluate(ctx, images, nil, map[string]any{}, nil, false, nil, libs.NewFakeContextProvider())
					}
					require.Error(t, err)
					require.Len(t, images.contexts, 1)
					require.Equal(t, ctx, images.contexts[0], "the CEL callback must receive the evaluation context")
					require.Equal(t, phase, images.contexts[0].Value(evaluationRequestMarker{}))
					deadline, _ := ctx.Deadline()
					observed, ok := images.contexts[0].Deadline()
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
