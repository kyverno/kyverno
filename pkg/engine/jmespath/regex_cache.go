package jmespath

import (
	"regexp"
	"regexp/syntax"
	"strings"
	"sync"
)

const (
	// maxCompiledRegexes bounds the number of compiled patterns kept in memory. Patterns can come
	// from resource data, so the cache must not grow without limit.
	maxCompiledRegexes = 1024

	// The entry count alone does not bound the bytes, because a short pattern can compile to a
	// very large program, e.g. `(abc|def|ghi){1000}` retains megabytes. A pattern whose program
	// is above these bounds is compiled on every call, as it was before the cache existed.
	maxCachedPatternLen = 512
	maxProgramInsts     = 1024
	maxProgramRunes     = 8192
	// regexp builds a second, one-pass program for an anchored pattern, and that copy repeats a
	// rune set once per alternation, capture or empty-width instruction in front of it.
	maxOnePassCost = 4096
)

// cachedRegex is a cache entry. Only one of the two fields is used: re is the compiled pattern,
// and large marks a pattern that is too big to retain, so later calls skip the size check.
type cachedRegex struct {
	re    *regexp.Regexp
	large bool
}

var compiledRegexes = struct {
	sync.RWMutex
	byPattern map[string]cachedRegex
}{byPattern: make(map[string]cachedRegex)}

// compileRegex returns the compiled form of pattern, reusing a previous compilation when there is
// one. Patterns that fail to compile are not cached, and neither is the compiled form of a
// pattern that is long or whose program is large. A *regexp.Regexp is safe for concurrent use.
func compileRegex(pattern string) (*regexp.Regexp, error) {
	compiledRegexes.RLock()
	entry, ok := compiledRegexes.byPattern[pattern]
	compiledRegexes.RUnlock()
	if ok {
		if entry.large {
			return regexp.Compile(pattern)
		}
		return entry.re, nil
	}
	if len(pattern) > maxCachedPatternLen {
		return regexp.Compile(pattern)
	}
	// pattern can be a piece of a larger string, such as a field of a resource. Copy it so that
	// neither the cache key nor the compiled regexp retains that string.
	pattern = strings.Clone(pattern)
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	entry = cachedRegex{re: reg}
	if isLargeProgram(pattern) {
		entry = cachedRegex{large: true}
	}
	compiledRegexes.Lock()
	if len(compiledRegexes.byPattern) >= maxCompiledRegexes {
		clear(compiledRegexes.byPattern)
	}
	compiledRegexes.byPattern[pattern] = entry
	compiledRegexes.Unlock()
	return reg, nil
}

// isLargeProgram reports whether the program compiled from pattern is too large to be retained
// by the cache. The pattern is known to compile.
func isLargeProgram(pattern string) bool {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return true
	}
	prog, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return true
	}
	if len(prog.Inst) > maxProgramInsts {
		return true
	}
	var runes, ranges, copies int
	for i := range prog.Inst {
		inst := &prog.Inst[i]
		switch inst.Op {
		case syntax.InstRune:
			// The parser trims the unused capacity of a class only when the class ends its
			// branch, so count the capacity, which is what is retained.
			runes += cap(inst.Rune)
			ranges += len(inst.Rune) / 2
		case syntax.InstAlt, syntax.InstAltMatch, syntax.InstCapture, syntax.InstEmptyWidth:
			copies++
		}
	}
	if runes > maxProgramRunes {
		return true
	}
	anchored := prog.StartCond()&syntax.EmptyBeginText != 0
	return anchored && len(prog.Inst) < 1000 && (copies+1)*ranges > maxOnePassCost
}
