package jmespath

import (
	"regexp"
	"sync"
)

// maxCompiledRegexes bounds the number of compiled patterns kept in memory. Patterns can come
// from resource data, so the cache must not grow without limit.
const maxCompiledRegexes = 1024

var compiledRegexes = struct {
	sync.RWMutex
	byPattern map[string]*regexp.Regexp
}{byPattern: make(map[string]*regexp.Regexp)}

// compileRegex returns the compiled form of pattern, reusing a previous compilation when there is
// one. Patterns that fail to compile are not cached. A *regexp.Regexp is safe for concurrent use.
func compileRegex(pattern string) (*regexp.Regexp, error) {
	compiledRegexes.RLock()
	reg, ok := compiledRegexes.byPattern[pattern]
	compiledRegexes.RUnlock()
	if ok {
		return reg, nil
	}
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	compiledRegexes.Lock()
	if len(compiledRegexes.byPattern) >= maxCompiledRegexes {
		clear(compiledRegexes.byPattern)
	}
	compiledRegexes.byPattern[pattern] = reg
	compiledRegexes.Unlock()
	return reg, nil
}
