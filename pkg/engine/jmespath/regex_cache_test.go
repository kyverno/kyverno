package jmespath

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetCompiledRegexes() {
	compiledRegexes.Lock()
	clear(compiledRegexes.byPattern)
	compiledRegexes.Unlock()
}

func Test_compileRegex_reusesCompiledPattern(t *testing.T) {
	resetCompiledRegexes()
	first, err := compileRegex(`^a+$`)
	require.NoError(t, err)
	second, err := compileRegex(`^a+$`)
	require.NoError(t, err)
	assert.Same(t, first, second)
}

func Test_compileRegex_doesNotCacheInvalidPattern(t *testing.T) {
	resetCompiledRegexes()
	_, err := compileRegex(`(`)
	assert.Error(t, err)
	compiledRegexes.RLock()
	defer compiledRegexes.RUnlock()
	assert.Empty(t, compiledRegexes.byPattern)
}

func Test_compileRegex_isBounded(t *testing.T) {
	resetCompiledRegexes()
	for i := 0; i < 3*maxCompiledRegexes; i++ {
		_, err := compileRegex(fmt.Sprintf("^pattern-%d$", i))
		require.NoError(t, err)
	}
	compiledRegexes.RLock()
	defer compiledRegexes.RUnlock()
	assert.LessOrEqual(t, len(compiledRegexes.byPattern), maxCompiledRegexes)
}

func Test_compileRegex_concurrentUse(t *testing.T) {
	resetCompiledRegexes()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				reg, err := compileRegex(fmt.Sprintf("^p-%d$", j%8))
				if assert.NoError(t, err) {
					assert.True(t, reg.MatchString(fmt.Sprintf("p-%d", j%8)))
				}
			}
		}()
	}
	wg.Wait()
}

func Test_jpRegexMatch_invalidPattern(t *testing.T) {
	_, err := jpRegexMatch([]any{"(", "abc"})
	assert.Error(t, err)
}
