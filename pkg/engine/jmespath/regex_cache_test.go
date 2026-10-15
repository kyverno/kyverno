package jmespath

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/lru"
)

func resetCompiledRegexes() {
	compiledRegexes.Clear()
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
	assert.Zero(t, compiledRegexes.Len())
}

func Test_compileRegex_isBounded(t *testing.T) {
	resetCompiledRegexes()
	for i := 0; i < 3*maxCompiledRegexes; i++ {
		_, err := compileRegex(fmt.Sprintf("^pattern-%d$", i))
		require.NoError(t, err)
	}
	assert.Equal(t, maxCompiledRegexes, compiledRegexes.Len())
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

func Test_compileRegex_largeProgramIsNotRetained(t *testing.T) {
	resetCompiledRegexes()
	pattern := `^(abc|def|ghi){1000}$`
	for range 2 {
		reg, err := compileRegex(pattern)
		require.NoError(t, err)
		assert.True(t, reg.MatchString(strings.Repeat("abc", 1000)))
		assert.False(t, reg.MatchString("abc"))
	}
	v, ok := compiledRegexes.Get(pattern)
	require.True(t, ok, "a marker should be cached")
	entry := v.(cachedRegex)
	assert.True(t, entry.large)
	assert.Nil(t, entry.re)
}

func Test_compileRegex_longPatternIsNotCached(t *testing.T) {
	resetCompiledRegexes()
	pattern := strings.Repeat("a", maxCachedPatternLen+1)
	reg, err := compileRegex(pattern)
	require.NoError(t, err)
	assert.True(t, reg.MatchString(pattern))
	assert.Zero(t, compiledRegexes.Len())
}

func Test_compileRegex_cachesTypicalPatterns(t *testing.T) {
	for _, pattern := range []string{
		`^[A-Za-z]+$`,
		`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
		`foo.*bar`,
		`\d{3}-\d{4}`,
	} {
		assert.False(t, isLargeProgram(pattern), pattern)
	}
}

func Test_compileRegex_largeProgramBounds(t *testing.T) {
	assert.True(t, isLargeProgram(`(abc|def|ghi){1000}`))
	// a class whose runes outgrow the cap although it has few instructions
	assert.True(t, isLargeProgram(`[`+strings.Repeat(`\pL`, 168)+`]a`))
	// anchored: the one-pass copy repeats the rune set once per capture
	assert.True(t, isLargeProgram(`^`+strings.Repeat(`(`, 250)+`\pL`+strings.Repeat(`)`, 250)+`$`))
}

func Test_compileRegex_keyDoesNotRetainCallerString(t *testing.T) {
	var keys []string
	original := compiledRegexes
	compiledRegexes = lru.NewWithEvictionFunc(maxCompiledRegexes, func(key lru.Key, _ any) {
		keys = append(keys, key.(string))
	})
	t.Cleanup(func() { compiledRegexes = original })

	big := "prefix-" + strings.Repeat("x", 1<<20) + "-^abc$-suffix"
	start := strings.Index(big, "^abc$")
	pattern := big[start : start+len("^abc$")]
	_, err := compileRegex(pattern)
	require.NoError(t, err)
	compiledRegexes.Clear()
	require.Len(t, keys, 1)
	assert.Equal(t, pattern, keys[0])
	assert.NotSame(t, unsafe.StringData(pattern), unsafe.StringData(keys[0]))
}

func Test_compileRegex_evictsLeastRecentlyUsed(t *testing.T) {
	resetCompiledRegexes()
	hot := `^hot$`
	_, err := compileRegex(hot)
	require.NoError(t, err)
	for i := 0; i < 2*maxCompiledRegexes; i++ {
		_, err := compileRegex(hot)
		require.NoError(t, err)
		_, err = compileRegex(fmt.Sprintf("^pattern-%d$", i))
		require.NoError(t, err)
	}
	_, ok := compiledRegexes.Get(hot)
	assert.True(t, ok, "a frequently used pattern should survive eviction")
	assert.Equal(t, maxCompiledRegexes, compiledRegexes.Len())
}
