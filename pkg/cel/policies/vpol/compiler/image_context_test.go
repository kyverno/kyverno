package compiler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type policyImageContext struct {
	libs.Context
	started chan context.Context
	abort   <-chan struct{}
}

func (p *policyImageContext) GetImageData(string, []remote.Option) (map[string]any, error) {
	return nil, errors.New("image callback is missing evaluation context")
}

func (p *policyImageContext) GetImageDataWithContext(ctx context.Context, image string, _ []remote.Option) (map[string]any, error) {
	if image == "blocked" {
		p.started <- ctx
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.abort:
			return nil, errors.New("test lookup stopped")
		}
	}
	return map[string]any{"resolvedImage": "ok"}, nil
}

func TestPolicyImageMetadataUsesEvaluationContext(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	for _, location := range []string{"match condition", "variable"} {
		t.Run(location, func(t *testing.T) {
			t.Parallel()
			spec := policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
				Validations:             []admissionregistrationv1.Validation{{Expression: "true"}},
			}
			if location == "match condition" {
				spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "metadata", Expression: `image.getMetadata(object.image).resolvedImage == "ok"`}}
			} else {
				spec.Variables = []admissionregistrationv1.Variable{{Name: "metadata", Expression: `image.getMetadata(object.image).resolvedImage`}}
				spec.Validations[0].Expression = `variables.metadata == "ok"`
			}
			policy, errs := NewCompiler().Compile(&policiesv1beta1.ValidatingPolicy{Spec: spec}, nil)
			require.Empty(t, errs)
			abort := make(chan struct{})
			defer close(abort)
			provider := &policyImageContext{Context: libs.NewFakeContextProvider(), started: make(chan context.Context, 1), abort: abort}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				result, err := policy.Evaluate(ctx, map[string]any{"image": "blocked"}, nil, nil, nil, nil, provider)
				if err == nil && result != nil {
					err = result.Error
				}
				finished <- err
			}()
			select {
			case requestCtx := <-provider.started:
				assert.Same(t, ctx, requestCtx)
			case <-time.After(time.Second):
				t.Fatal("policy did not bind the image receiver before evaluation")
			}
			var calls sync.WaitGroup
			for range 4 {
				calls.Go(func() {
					result, err := policy.Evaluate(context.Background(), map[string]any{"image": "allowed"}, nil, nil, nil, nil, provider)
					assert.NoError(t, err)
					if assert.NotNil(t, result) {
						assert.NoError(t, result.Error)
						assert.True(t, result.Result)
					}
				})
			}
			calls.Wait()
			cancel()
			select {
			case err := <-finished:
				require.ErrorContains(t, err, "context canceled")
			case <-time.After(time.Second):
				t.Fatal("policy metadata lookup ignored cancellation")
			}
			result, err := policy.Evaluate(context.Background(), map[string]any{"image": "allowed"}, nil, nil, nil, nil, provider)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NoError(t, result.Error)
			assert.True(t, result.Result)
		})
	}
}
