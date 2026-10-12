package compiler

import (
	"fmt"
	"regexp/syntax"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
)

// disabledGuardForTest is a negative-control hook: a pinning test sets this to a function
// name to make newGuardDecorator treat that one function as unguarded, so the test can
// confirm a "refused" row actually depends on the guard rather than on some other mechanism
// (the per-program cost limit, the argBytes cap) that would refuse the same input anyway
// and hide a broken or removed guard behind a passing test. Never set outside a test;
// production code never writes to this.
var disabledGuardForTest atomic.Value // string; unset Load() returns nil, handled by the type assertion at the read site below

// SetDisabledGuardForTest is the only writer, called from test code to install and then
// clear the negative control. name == "" disables no guard.
func SetDisabledGuardForTest(name string) {
	disabledGuardForTest.Store(name)
}

// --- guard-table constants ---

const (
	parseExpansion      = 128    // measured ~85 B allocated per source byte for "[1,1,1,...]" JSON
	yamlParseExpansion  = 2048   // measured ~1,941x for an alias-nested YAML literal, rounded up to a power of two
	maxPatternBytes     = 16384  // pattern length gate, checked before any syntax.Parse
	maxUnrolledRunes    = 20000  // compiled program <= 3*unrolled + 2*nodes instructions
	maxUnboundedRepeat  = 4096   // regen.DefaultMaxUnboundedRepeatCount
	maxQuantityExponent = 100000 // the 1e100000000 hazard
	maxQuantityDigits   = 1000   // len(num) + len(denom)
	interruptEvery      = 100    // celconfig.CheckFrequency
)

// maxParseBytes = maxStepBytes / parseExpansion.
func maxParseBytes() uint64 { return maxArgBytes() / parseExpansion }

// maxYamlParseBytes bounds yaml.parse's input separately from json.unmarshal/x509.decode:
// yaml.parse expands far more per byte (alias expansion), so it needs its own, tighter cap
// rather than sharing maxParseBytes.
func maxYamlParseBytes() uint64 { return maxArgBytes() / yamlParseExpansion }

// maxRandomRunes = maxStepBytes / utf8.UTFMax.
func maxRandomRunes() uint64 { return maxArgBytes() / utf8.UTFMax }

// guardFunc is a hard cap on a quantity computable from a call's arguments in one or two
// lines. It returns a CEL error to refuse the call, or nil to allow it.
type guardFunc func(args []ref.Val) ref.Val

// guardError builds a refusal, wrapping ErrGuardRefused (via %w, last argument) so
// IsResourceError recognizes it -- this is what lets __orValueCompat__ propagate a guard
// refusal instead of silently treating it as an ordinary type error.
func guardError(format string, a ...any) ref.Val {
	args := append(append([]any{}, a...), ErrGuardRefused)
	return types.NewErr("cel: "+format+": %w", args...)
}

// --- binding registry: resolves a call's original implementation the way the planner
// does (env.Functions()[name].Bindings()), so the guard decorator wraps the real function
// instead of reimplementing dispatch. ---

type bindingRegistry struct {
	byOperator map[string]*functions.Overload   // keyed by Overload.Operator: an overload ID for a typed overload, or the function name for a singleton/dynamic-dispatch binding
	byFunction map[string][]*functions.Overload // every overload Bindings() returned for this function name, for the arity fallback below
}

func newBindingRegistry(env *cel.Env) *bindingRegistry {
	reg := &bindingRegistry{byOperator: map[string]*functions.Overload{}, byFunction: map[string][]*functions.Overload{}}
	if env == nil {
		return reg
	}
	for name, fn := range env.Functions() {
		overloads, err := fn.Bindings()
		if err != nil {
			// best-effort: a function with no resolvable binding is left out of the
			// registry; resolve() then reports it as unresolvable, same as if it were
			// never in env.Functions() at all.
			continue
		}
		for _, o := range overloads {
			reg.byOperator[o.Operator] = o
			reg.byFunction[name] = append(reg.byFunction[name], o)
		}
	}
	return reg
}

// overloadArity reports the fixed arity of a binding, when it has one (Unary or Binary);
// Function (variadic) bindings have no fixed arity and are excluded from the fallback below.
func overloadArity(o *functions.Overload) (int, bool) {
	switch {
	case o.Unary != nil:
		return 1, true
	case o.Binary != nil:
		return 2, true
	default:
		return 0, false
	}
}

// resolve looks up a call's original binding: overload ID first, then function name (the
// planner's own order), then -- if exactly one of this function's overloads has a fixed
// arity matching the call's argument count -- that unique match.
func (r *bindingRegistry) resolve(function, overloadID string, arity int) (*functions.Overload, string) {
	if overloadID != "" {
		if o, ok := r.byOperator[overloadID]; ok {
			return o, ""
		}
	}
	if o, ok := r.byOperator[function]; ok {
		return o, ""
	}
	var unique *functions.Overload
	matches := 0
	arities := make([]int, 0, len(r.byFunction[function]))
	for _, o := range r.byFunction[function] {
		a, ok := overloadArity(o)
		if !ok {
			continue
		}
		arities = append(arities, a)
		if a == arity {
			unique = o
			matches++
		}
	}
	if matches == 1 {
		return unique, ""
	}
	return nil, fmt.Sprintf("overload ID %q, available arities %v", overloadID, arities)
}

func callOverload(o *functions.Overload, args []ref.Val) ref.Val {
	switch {
	case o.Function != nil:
		return o.Function(args...)
	case len(args) == 1 && o.Unary != nil:
		return o.Unary(args[0])
	case len(args) == 2 && o.Binary != nil:
		return o.Binary(args[0], args[1])
	case o.Unary != nil && len(args) >= 1:
		return o.Unary(args[0])
	case o.Binary != nil && len(args) >= 2:
		return o.Binary(args[0], args[1])
	default:
		return types.NewErr("cel: no such overload: %s", o.Operator)
	}
}

// --- the argument cap + guard decorator ---

// newGuardDecorator wraps every non-exempt InterpretableCall so that (1) its arguments are
// measured by argBytes and refused above maxArgBytes before the call runs, and (2) the
// function's guard, if any, runs. Fold steps (isFoldStep) are dispatched directly: their
// accumulator must never be walked (the fold rule, cost.go) and has no cap of its own --
// the per-program cost limit is what bounds it, through the estimator's charge.
func newGuardDecorator(reg *bindingRegistry) interpreter.InterpretableDecoratorV2 {
	return func(i interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
		call, ok := i.(interpreter.InterpretableCall)
		if !ok {
			return i, nil
		}
		function := call.Function()
		if exemptFunctions[function] {
			return i, nil
		}
		overloadID := call.OverloadID()
		args := call.Args()
		if len(args) == 0 {
			// A zero-argument call (time.now(), for example) has nothing to measure and
			// no guard can apply to it. interpreter.NewCall below always rebuilds the
			// node as an evalVarArgs regardless of original arity, and cel-go's
			// evalVarArgs indexes argVals[0] unconditionally -- with no arguments that is
			// an out-of-range panic surfaced as "internal error" at evaluation time, not
			// at Compile or NewProgram time. Leaving the call undecorated is safe: no
			// arguments means no argBytes to measure and no guard predictor to run.
			return i, nil
		}
		o, diag := reg.resolve(function, overloadID, len(args))
		if o == nil {
			// Fails construction: an unwrapped node would run with no argument cap and no
			// guard, silently reopening exactly the hazard this decorator exists to close.
			return nil, fmt.Errorf("cel: function %q has no resolvable binding (%s)", function, diag)
		}
		guard := guardTable[function]
		if disabled, _ := disabledGuardForTest.Load().(string); disabled != "" && disabled == function {
			// A test-only negative control (see disabledGuardForTest's doc comment).
			guard = nil
		}

		// Construction-time check for a constant pattern/quantity argument: a bad literal
		// fails policy construction instead of waiting for first eval. Every guard in
		// constGuardArgIndex tolerates the other argument positions being nil.
		if guard != nil {
			if idx, ok := constGuardArgIndex[function]; ok && idx < len(args) {
				if c, ok := args[idx].(interpreter.InterpretableConst); ok {
					probe := make([]ref.Val, len(args))
					probe[idx] = c.Value()
					if errVal := guard(probe); errVal != nil {
						// %w, not %v: preserves the ErrGuardRefused wrapping guardError
						// attaches, so IsResourceError still recognizes a
						// construction-time refusal as a resource error.
						if e, ok := errVal.(error); ok {
							return nil, fmt.Errorf("%w", e)
						}
						return nil, fmt.Errorf("%v", errVal)
					}
				}
			}
		}

		wrapped := func(vals ...ref.Val) ref.Val {
			if isFoldStep(function, vals) {
				return callOverload(o, vals)
			}
			cap := maxArgBytes()
			if b := argBytes(vals, cap); b > cap {
				return guardError("%q refused: arguments measure %d bytes, over the %d-byte cap", function, b, cap)
			}
			if guard != nil {
				if errVal := guard(vals); errVal != nil {
					return errVal
				}
			}
			return callOverload(o, vals)
		}
		return interpreter.NewCall(i.ID(), function, overloadID, args, wrapped), nil
	}
}

// constGuardArgIndex names, for functions whose guard can be evaluated from a single
// constant argument, which argument index that is: the pattern for every pattern-taking
// function, the input string for quantity/isQuantity.
var constGuardArgIndex = map[string]int{
	"matches": 1, "regex.extract": 1, "regex.extractAll": 1, "regex.replace": 1,
	"find": 1, "findAll": 1, "random": 0, "random.random": 0,
	"quantity": 0, "isQuantity": 0,
}

// guardPreservingRegexOptimizations shadows the constant-pattern regex optimizations that
// k8s.io/apiserver's library.Regex installs for find and findAll. Each of those rebuilds the
// call node around its own closure (its Factory calls interpreter.NewCall), which discards
// this package's guard decorator wrapper and with it both the argument cap and the guard --
// so with a constant pattern findAll ran unguarded while a variable pattern was refused.
// decRegexOptimizer keys optimizations by function name in a map and CostLib registers last,
// so these entries replace the apiserver's; returning the call unchanged keeps the guarded
// node. A constant pattern is then recompiled per evaluation: cel.ValidateRegexLiterals in
// the base env still rejects a malformed literal at compile time, and patternGuard still
// bounds the pattern, so only the precompilation is given up.
func guardPreservingRegexOptimizations() []*interpreter.RegexOptimization {
	keepGuardedNode := func(fn string) *interpreter.RegexOptimization {
		return &interpreter.RegexOptimization{
			Function:   fn,
			RegexIndex: 1,
			Factory: func(call interpreter.InterpretableCall, _ string) (interpreter.InterpretableCall, error) {
				return call, nil
			},
		}
	}
	return []*interpreter.RegexOptimization{keepGuardedNode("find"), keepGuardedNode("findAll")}
}

// --- the guard table ---

var guardTable = map[string]guardFunc{
	"matches":          patternGuard(1),
	"regex.extract":    patternGuard(1),
	"regex.extractAll": extractAllGuard,
	"regex.replace":    regexReplaceGuard,
	"find":             patternGuard(1),
	"findAll":          findAllGuard,
	"random":           randomGuard,
	"random.random":    randomGuard,
	"replace":          replaceGuard,
	"sets.contains":    setsGuard,
	"sets.equivalent":  setsGuard,
	"sets.intersects":  setsGuard,
	"split":            splitGuard,
	"quantity":         quantityCallGuard,
	"isQuantity":       quantityCallGuard,
	"unmarshal":        parserGuard(1, maxParseBytes), // json.unmarshal in policy text; see parserGuard's comment
	"x509.decode":      parserGuard(0, maxParseBytes),
	"parse":            parserGuard(1, maxYamlParseBytes), // yaml.parse in policy text; MemberOverload on the yaml Context variable
	"join":             joinGuard,
	"marshal":          expander6Guard(1), // json Context receiver at args[0], the value to marshal at args[1]
	"strings.quote":    expander6Guard(0), // plain global function, args[0] is the input
	"format":           formatGuard,       // string_format left tier 1; args[0] template, args[1] list
	"distinct":         distinctGuard,     // ext/lists.go's distinct is O(n^2) element comparisons
	"json.encode":      expander6Guard(0), // ext.Encoders' cel.Function("json.encode", ...), a literal dotted global name
	"lists.range":      listsRangeGuard,   // allocates one element per requested index, from an int
}

// --- regex pattern gate + unrolled-length walk ---

// patternGate refuses a pattern above maxPatternBytes before any syntax.Parse runs: a
// pattern-length gate alone is not the whole bound (a 3 KB pattern can compile to hundreds
// of MB), but it is the first check, run before any allocation-proportional-to-the-pattern
// happens.
func patternGate(pattern string) ref.Val {
	if len(pattern) > maxPatternBytes {
		return guardError("regex pattern is %d bytes, over the %d-byte cap", len(pattern), maxPatternBytes)
	}
	return nil
}

// unrolledRunes upper-bounds the number of characters a pattern denotes after every
// repeat is expanded to its maximum: literal = rune count; character class or any-char =
// 1; concat and alternate = sum of children; capture, ? = child; *, + = child*starFactor;
// {n,m} = child*max(n,m,1); {n,} = child*max(n,1)*starFactor; saturating at MaxInt64.
func unrolledRunes(re *syntax.Regexp, starFactor int64) int64 {
	if re == nil {
		return 0
	}
	switch re.Op {
	case syntax.OpLiteral:
		return int64(len(re.Rune))
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return 1
	case syntax.OpConcat, syntax.OpAlternate:
		var sum int64
		for _, sub := range re.Sub {
			sum = saturatingAdd(sum, unrolledRunes(sub, starFactor))
		}
		return sum
	case syntax.OpCapture, syntax.OpQuest:
		if len(re.Sub) == 0 {
			return 0
		}
		return unrolledRunes(re.Sub[0], starFactor)
	case syntax.OpStar, syntax.OpPlus:
		if len(re.Sub) == 0 {
			return 0
		}
		return saturatingMul(unrolledRunes(re.Sub[0], starFactor), starFactor)
	case syntax.OpRepeat:
		if len(re.Sub) == 0 {
			return 0
		}
		child := unrolledRunes(re.Sub[0], starFactor)
		if re.Max == -1 {
			mult := maxInt64(int64(re.Min), 1)
			return saturatingMul(saturatingMul(child, mult), starFactor)
		}
		mult := maxInt64(maxInt64(int64(re.Min), int64(re.Max)), 1)
		return saturatingMul(child, mult)
	default:
		return 0 // anchors and boundaries match the empty string
	}
}

const maxInt64Val = int64(1) << 62 // saturation ceiling, well under math.MaxInt64

func saturatingAdd(a, b int64) int64 {
	if a > maxInt64Val-b {
		return maxInt64Val
	}
	return a + b
}

func saturatingMul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > maxInt64Val/b {
		return maxInt64Val
	}
	return a * b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// patternArgString extracts a string pattern from args[idx], tolerating a nil/missing
// argument (the construction-time probe leaves non-pattern positions nil).
func patternArgString(args []ref.Val, idx int) (string, bool) {
	if idx >= len(args) || args[idx] == nil {
		return "", false
	}
	s, ok := args[idx].(types.String)
	return string(s), ok
}

// patternGuard is the shared base guard for matches, regex.extract, find: the pattern gate
// then the unrolled-length walk at starFactor 1 (compilation), refusing above
// maxUnrolledRunes. An invalid regex is left to the real call to report as a compile error.
func patternGuard(patternIdx int) guardFunc {
	return func(args []ref.Val) ref.Val {
		pattern, ok := patternArgString(args, patternIdx)
		if !ok {
			return nil
		}
		if errVal := patternGate(pattern); errVal != nil {
			return errVal
		}
		parsed, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			return nil
		}
		if n := unrolledRunes(parsed, 1); n > maxUnrolledRunes {
			return guardError("regex pattern unrolls to %d characters, over the %d cap", n, maxUnrolledRunes)
		}
		return nil
	}
}

func extractAllGuard(args []ref.Val) ref.Val {
	if errVal := patternGuard(1)(args); errVal != nil {
		return errVal
	}
	t, ok := patternArgString(args, 0)
	if !ok {
		return nil
	}
	limit := int64(maxArgBytes() / 72) //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
	if int64(len(t)) > limit {
		return guardError("regex.extractAll target is %d bytes, over the %d-byte cap", len(t), limit)
	}
	return nil
}

func findAllGuard(args []ref.Val) ref.Val {
	if errVal := patternGuard(1)(args); errVal != nil {
		return errVal
	}
	t, ok := patternArgString(args, 0)
	if !ok {
		return nil
	}
	limit := int64(maxArgBytes() / 16) //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
	if int64(len(t)) > limit {
		return guardError("findAll target is %d bytes, over the %d-byte cap", len(t), limit)
	}
	return nil
}

// randomGuard covers both the 1-arg (pattern) and 0-arg forms of random/random.random: the
// 0-arg form has no pattern argument and is unguarded. goregen parses with Flags(0) (its
// own zero-value GeneratorArgs.Flags), not syntax.Perl, and draws each unbounded repeat
// count in [min, maxUnboundedRepeat], so the unrolled walk uses starFactor =
// maxUnboundedRepeat and the generation-sized cap maxRandomRunes.
func randomGuard(args []ref.Val) ref.Val {
	pattern, ok := patternArgString(args, 0)
	if !ok {
		return nil
	}
	if errVal := patternGate(pattern); errVal != nil {
		return errVal
	}
	parsed, err := syntax.Parse(pattern, syntax.Flags(0))
	if err != nil {
		return nil
	}
	limit := int64(maxRandomRunes()) //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
	if n := unrolledRunes(parsed, maxUnboundedRepeat); n > limit {
		return guardError("random pattern unrolls to %d generated characters, over the %d cap", n, limit)
	}
	return nil
}

// replaceGuard bounds ext.Strings' replace(t, old, new[, limit]): every position can match
// an empty old, so the worst-case output is len(t) + (runes(t)+1) * len(new).
func replaceGuard(args []ref.Val) ref.Val {
	if len(args) < 3 {
		return nil
	}
	t, ok1 := args[0].(types.String)
	newS, ok2 := args[2].(types.String)
	if !ok1 || !ok2 {
		return nil
	}
	ts := string(t)
	bound := int64(len(ts)) + (int64(utf8.RuneCountInString(ts))+1)*int64(len(string(newS)))
	if limit := int64(maxArgBytes()); bound > limit { //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
		return guardError("replace output bound %d bytes, over the %d-byte cap", bound, limit)
	}
	return nil
}

// regexReplaceGuard bounds ext.Regex's regex.replace(t, re, repl[, n]): with a backslash
// digit in repl (a backreference), the worst case is (runes(t)+1) * len(repl) * len(t);
// otherwise it is the same "empty match at every position" bound as replace.
func regexReplaceGuard(args []ref.Val) ref.Val {
	if errVal := patternGuard(1)(args); errVal != nil {
		return errVal
	}
	if len(args) < 3 {
		return nil
	}
	t, ok1 := args[0].(types.String)
	repl, ok2 := args[2].(types.String)
	if !ok1 || !ok2 {
		return nil
	}
	ts := string(t)
	rs := string(repl)
	tRunes := int64(utf8.RuneCountInString(ts))
	var bound int64
	if hasBackreference(rs) {
		bound = (tRunes + 1) * int64(len(rs)) * int64(len(ts))
	} else {
		bound = int64(len(ts)) + (tRunes+1)*int64(len(rs))
	}
	if limit := int64(maxArgBytes()); bound > limit { //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
		return guardError("regex.replace output bound %d bytes, over the %d-byte cap", bound, limit)
	}
	return nil
}

func hasBackreference(repl string) bool {
	for i := 0; i+1 < len(repl); i++ {
		if repl[i] == '\\' && repl[i+1] >= '0' && repl[i+1] <= '9' {
			return true
		}
	}
	return false
}

// setsGuard bounds sets.contains/equivalent/intersects by the number of element
// comparisons, not bytes: element equality is itself bounded by the argument cap.
func setsGuard(args []ref.Val) ref.Val {
	if len(args) < 2 {
		return nil
	}
	prod := shallowSize(args[0]) * shallowSize(args[1])
	limit := perProgramLimit()
	if prod > limit {
		return guardError("sets comparison is %d element pairs, over the %d cap", prod, limit)
	}
	return nil
}

// splitGuard bounds ext.Strings' split(t, sep): strings.Split returns one 16-byte header
// per piece, and the guard bounds that header array to <= maxArgBytes.
//
// The piece count is counted, not assumed. len(t)/len(sep) is the count only if every
// separator-length window is a separator, which no real input reaches: a 640 KiB file split
// on "\n" was refused for 655,360 pieces when it had 64 lines. strings.Count is a single
// O(n) scan that allocates nothing -- far cheaper than the split it is guarding -- and
// strings.Split returns exactly Count+1 pieces for a non-empty separator. An empty
// separator splits into runes instead, so that case counts runes.
func splitGuard(args []ref.Val) ref.Val {
	if len(args) < 2 {
		return nil
	}
	t, ok1 := args[0].(types.String)
	sep, ok2 := args[1].(types.String)
	if !ok1 || !ok2 {
		return nil
	}
	var pieces int64
	if len(string(sep)) == 0 {
		pieces = int64(utf8.RuneCountInString(string(t)))
	} else {
		pieces = int64(strings.Count(string(t), string(sep))) + 1
	}
	limit := int64(maxArgBytes() / 16) //nolint:gosec // maxArgBytes/maxRandomRunes are operator-configured caps, never attacker-controlled and never near 1<<63
	if pieces > limit {
		return guardError("split produces %d pieces, over the %d-piece cap", pieces, limit)
	}
	return nil
}

// joinGuard bounds ext.Strings' list.join(sep): the exact strings.Join predictor --
// sum(len(e) for e in list) + max(len(list)-1, 0) * len(sep) -- rather than an
// over-approximation, since a large separator with many (even empty) elements is the
// hazard. args[0] is the list receiver, args[1] the separator.
func joinGuard(args []ref.Val) ref.Val {
	if len(args) < 2 {
		return nil
	}
	list, ok1 := args[0].(traits.Lister)
	sep, ok2 := args[1].(types.String)
	if !ok1 || !ok2 {
		return nil
	}
	n := int64(shallowSize(list)) //nolint:gosec // element counts are bounded by argBytes long before reaching anywhere near 1<<63
	var sumElems int64
	it := list.Iterator()
	for it.HasNext() == types.True {
		if s, ok := it.Next().(types.String); ok {
			sumElems += int64(len(string(s)))
		}
	}
	seps := n - 1
	if seps < 0 {
		seps = 0
	}
	bound := sumElems + seps*int64(len(string(sep)))
	limit := int64(maxArgBytes()) //nolint:gosec // maxArgBytes is an operator-configured cap, never attacker-controlled and never near 1<<63
	if bound > limit {
		return guardError("join output bound %d bytes, over the %d-byte cap", bound, limit)
	}
	return nil
}

// expander6Guard bounds functions whose output can be up to 6x their input: JSON and CEL
// quoting escape a byte to at most six characters ("\u00XX"). inputIdx differs per entry
// the same way parserGuard's does: marshal is a MemberOverload on the SDK's "json" Context
// variable (receiver at args[0], value to marshal at args[1]); strings.quote is a plain
// global function (args[0] is the input).
func expander6Guard(inputIdx int) guardFunc {
	return func(args []ref.Val) ref.Val {
		if inputIdx >= len(args) {
			return nil
		}
		b := argBytes([]ref.Val{args[inputIdx]}, maxArgBytes())
		limit := maxArgBytes() / 6
		if b > limit {
			return guardError("input to a 6x-expanding function is %d bytes, over the %d-byte cap", b, limit)
		}
		return nil
	}
}

// formatGuard bounds ext.Strings' "%s".format([args]): string_format left tier 1 because
// cel-go prices it by the *template* (args[0]), not the list argument the formatted result
// actually depends on -- refuse when len(template) + 2*argBytes(list) > maxArgBytes; 2x is
// a generous bound on every format verb's own expansion (no verb, not even "%x" on bytes,
// expands an argument by more than 2x). args[0] is the template (receiver), args[1] the
// list of values.
func formatGuard(args []ref.Val) ref.Val {
	if len(args) < 2 {
		return nil
	}
	template, ok := args[0].(types.String)
	if !ok {
		return nil
	}
	listBytes := argBytes([]ref.Val{args[1]}, maxArgBytes())
	limit := maxArgBytes()
	bound := uint64(len(string(template))) + 2*listBytes
	if bound > limit {
		return guardError("format template + 2x argument bytes is %d, over the %d-byte cap", bound, limit)
	}
	return nil
}

// distinctGuard bounds ext.Lists' list.distinct(): the implementation is O(n^2) element
// comparisons, so refusing above 10,000 elements keeps the worst-case wall time bounded.
// args[0] is the list receiver.
func distinctGuard(args []ref.Val) ref.Val {
	if len(args) < 1 {
		return nil
	}
	list, ok := args[0].(traits.Lister)
	if !ok {
		return nil
	}
	const maxDistinctElements = 10_000
	n := shallowSize(list)
	if n > maxDistinctElements {
		return guardError("distinct() over %d elements, over the %d-element cap (O(n^2) comparisons)", n, maxDistinctElements)
	}
	return nil
}

// parserGuard bounds a parser-style function's input against limit(): the charge for an
// admitted call is applied by cost.go's parserFloor, so no separate charge is needed here.
// inputIdx differs per entry: x509.decode is a plain global function (args[0] is the
// input); unmarshal and yaml's parse are MemberOverloads on a Context variable (the
// receiver is always arg0 at the interpreter level, so args[1] is the input).
func parserGuard(inputIdx int, limit func() uint64) guardFunc {
	return func(args []ref.Val) ref.Val {
		if inputIdx >= len(args) {
			return nil
		}
		var length int
		switch v := args[inputIdx].(type) {
		case types.String:
			length = len(string(v))
		case types.Bytes:
			length = len(v)
		default:
			return nil
		}
		cap := limit()
		if uint64(length) > cap {
			return guardError("parser input is %d bytes, over the %d-byte cap", length, cap)
		}
		return nil
	}
}

// listsRangeGuard bounds ext.Lists' lists.range(n): it materializes one element per
// requested index from a single integer argument, so the result -- not the argument -- is
// what grows, and the generic rule's result term is charged only after the allocation has
// already happened. Bound the element count to maxArgBytes at the same per-element weight
// argBytes and resultBytes use.
func listsRangeGuard(args []ref.Val) ref.Val {
	if len(args) < 1 {
		return nil
	}
	n, ok := args[0].(types.Int)
	if !ok {
		return nil
	}
	if n <= 0 {
		return nil
	}
	limit := maxArgBytes() / bytesPerElement
	if uint64(n) > limit {
		return guardError("lists.range(%d) produces more than the %d-element cap", int64(n), limit)
	}
	return nil
}

// --- quantity guard ---

// quantityCallGuard is the guard-table entry for quantity(s) and isQuantity(s).
func quantityCallGuard(args []ref.Val) ref.Val {
	s, ok := patternArgString(args, 0)
	if !ok {
		return nil
	}
	return quantityGuard(s)
}

// quantityGuard parses a quantity string the way resource.ParseQuantity does far enough to
// bound the exponent and mantissa digit count, without constructing the value: refuse when
// abs(exponent) > maxQuantityExponent (the 1e100000000 hazard) or len(num)+len(denom) >
// maxQuantityDigits (the quadratic inf.Dec.SetString path). The digit scan is deliberately
// a looser superset of ParseQuantity's own scanner (it does not strip leading zeros), which
// only makes the guard more conservative, never less.
func quantityGuard(s string) ref.Val {
	num, denom, suffix := scanQuantityParts(s)
	if digits := len(num) + len(denom); digits > maxQuantityDigits {
		return guardError("quantity mantissa is %d digits, over the %d-digit cap", digits, maxQuantityDigits)
	}
	if exp, ok := scientificExponent(suffix); ok {
		if exp < 0 {
			exp = -exp
		}
		if exp > maxQuantityExponent {
			return guardError("quantity exponent %d exceeds the %d cap", exp, maxQuantityExponent)
		}
	}
	return nil
}

func scanQuantityParts(s string) (num, denom, suffix string) {
	i, n := 0, len(s)
	if i < n && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	for i < n && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	num = s[start:i]
	if i < n && s[i] == '.' {
		i++
		start = i
		for i < n && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		denom = s[start:i]
	}
	suffix = s[i:]
	return
}

// scientificExponent extracts the exponent from a quantity's DecimalExponent suffix
// ("e100000000", "E-3"); every other suffix kind (Ki, M, m, ...) denotes a fixed, small
// exponent and is not the hazard this exists for, so it reports ok=false.
func scientificExponent(suffix string) (int64, bool) {
	if len(suffix) == 0 || (suffix[0] != 'e' && suffix[0] != 'E') {
		return 0, false
	}
	rest := suffix[1:]
	neg := false
	if len(rest) > 0 && (rest[0] == '+' || rest[0] == '-') {
		neg = rest[0] == '-'
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return 0, false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	// A digit run this long is certainly over the cap; avoid parsing an arbitrarily large
	// integer just to find that out.
	if len(rest) > 15 {
		if neg {
			return -(maxQuantityExponent + 1), true
		}
		return maxQuantityExponent + 1, true
	}
	v, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		if neg {
			return -(maxQuantityExponent + 1), true
		}
		return maxQuantityExponent + 1, true
	}
	if neg {
		v = -v
	}
	return v, true
}
