package compiler

import (
	"sync"
	"testing"

	"cel.dev/cel-go/cel"
	"github.com/stretchr/testify/require"
)

// TestWarmUpCheckerConcurrentCompile runs under go test -race in CI: without
// the warm-up, concurrent undeclared-reference and comprehension checks on one
// environment race in cel.dev/cel-go v0.33.2.
func TestWarmUpCheckerConcurrentCompile(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	require.NoError(t, err)
	WarmUpChecker(env)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, iss := env.Compile("missing.field == 1")
			require.Error(t, iss.Err())
		}()
		go func() {
			defer wg.Done()
			_, iss := env.Compile("[1, 2].all(x, x > 0)")
			require.NoError(t, iss.Err())
		}()
	}
	wg.Wait()
}
