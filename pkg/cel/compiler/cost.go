package compiler

import (
	"errors"
	"math"

	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
	"k8s.io/apiserver/pkg/cel/library"
)

// f is StringTraversalCostFactor (common.StringTraversalCostFactor in cel-go): 10 bytes
// per cost unit. Kept local so cost.go, guards.go, and costlib.go share one literal.
const f = 0.1

// bytesPerElement: measured ~170 B per decoded []any/map[string]any node, rounded up.
const bytesPerElement uint64 = 192

// programLimit is L, the per-program cost limit: celconfig.PerCallLimit (1,000,000), the
// same default k8s.io/apiserver/pkg/apis/cel uses for --celCostLimit. Hardcoded (the
// minimal cut drops the --celCostLimit flag), but kept behind a function so call sites read
// the same way they would if it were configurable again.
const programLimit uint64 = 1_000_000

func perProgramLimit() uint64 { return programLimit }

// maxArgBytes = maxStepBytes = L / f = 10 MB at the defaults: the cap every non-exempt
// call's arguments and every guard-table quantity derive from.
func maxArgBytes() uint64 {
	return uint64(math.Ceil(float64(perProgramLimit()) / f))
}

// --- measures ---
//
// selecting a types.String out of a DynamicMap wraps the backing bytes without copying,
// which is why arguments are free to construct and must be measured at use (argBytes), and
// why a result that is only referenced (never materialized) is cheap to build but expensive
// to consume.

// shallowSize is traits.Sizer.Size() for strings, bytes, lists, and maps -- the rune count
// for strings (cel-go's own convention) and the element/entry count for aggregates -- and 1
// for everything else.
func shallowSize(v ref.Val) uint64 {
	if sz, ok := v.(traits.Sizer); ok {
		if n, ok := sz.Size().(types.Int); ok && int64(n) >= 0 {
			return uint64(n) //nolint:gosec // int64(n) >= 0 checked immediately above, so this is an exact, non-overflowing widen
		}
	}
	return 1
}

// resultBytes is the shallow weight of a call's result, at the same per-element weight as
// argBytes so the two sides are symmetric: byte length for a string or bytes value;
// bytesPerElement * shallowSize(v) for a list or map (192 per element or entry, no
// descent); 8 for any other scalar. Symmetry is what makes "charged-equivalent" hold: the
// charge is at least the value's real retained bytes for every result kind.
func resultBytes(v ref.Val) uint64 {
	switch t := v.(type) {
	case types.String:
		return uint64(len(string(t)))
	case types.Bytes:
		return uint64(len(t))
	}
	if _, ok := v.(traits.Lister); ok {
		return bytesPerElement * shallowSize(v)
	}
	if _, ok := v.(traits.Mapper); ok {
		return bytesPerElement * shallowSize(v)
	}
	return 8
}

// argBytes is a depth-first walk through traits.Lister and traits.Mapper iterators that
// adds the byte length of every string and bytes value reached, 8 per other scalar, and
// bytesPerElement per list element and map entry -- counting a value every time it is
// reached (references are not deduplicated: a list of 200 references to one 1 MiB string
// measures 200 MiB) -- and stops as soon as the running total exceeds cap. Cost of the walk
// itself is bounded by cap.
func argBytes(vals []ref.Val, cap uint64) uint64 {
	var total uint64
	for _, v := range vals {
		if total > cap {
			return total
		}
		total = measureInto(v, total, cap)
	}
	return total
}

func measureInto(v ref.Val, total, cap uint64) uint64 {
	if total > cap {
		return total
	}
	switch t := v.(type) {
	case types.String:
		return total + uint64(len(string(t)))
	case types.Bytes:
		return total + uint64(len(t))
	case *types.Optional:
		if t.HasValue() {
			return measureInto(t.GetValue(), total, cap)
		}
		return total + 8
	}
	if l, ok := v.(traits.Lister); ok {
		it := l.Iterator()
		for it.HasNext() == types.True {
			if total > cap {
				return total
			}
			total += bytesPerElement
			total = measureInto(it.Next(), total, cap)
		}
		return total
	}
	if m, ok := v.(traits.Mapper); ok {
		it := m.Iterator()
		for it.HasNext() == types.True {
			if total > cap {
				return total
			}
			total += bytesPerElement
			k := it.Next()
			total = measureInto(k, total, cap)
			if val, found := m.Find(k); found {
				total = measureInto(val, total, cap)
			}
		}
		return total
	}
	return total + 8
}

// --- the comprehension fold rule ---
//
// map, filter, and the two-variable transformList expand to a fold whose per-iteration
// step is _+_(@result, [x]) (parser/macro.go); transformMap uses cel.@mapInsert
// (ext/comprehensions.go). Both grow an accumulator in place (concatList / mutableList
// append, common/types/list.go; interpretable.go's fold does not copy it). Walking or
// charging that accumulator on every iteration is quadratic; charging one flat unit per
// append is also wrong because a tracker-priced producer (flatten, sort, ...) can hand the
// fold a multi-megabyte element the estimator never priced. The rule: the appended element
// is measured once, shallowly, by resultBytes, and charged f * that. Never the accumulator
// (args[0] for _+_, or the map being inserted into for @mapInsert), by either the guard
// decorator (guards.go) or the estimator below -- both call isFoldStep to route around
// their normal argBytes/resultBytes handling of the growing structure.

// isFoldStep reports whether this call is a comprehension fold step: _+_ whose first
// argument is already a list (the growing accumulator), or cel.@mapInsert. A user-written
// `a + b` on two list literals is the same case at k = len(b).
func isFoldStep(function string, args []ref.Val) bool {
	if function == mapInsertFunction {
		return true
	}
	if function != operators.Add || len(args) != 2 {
		return false
	}
	_, ok := args[0].(traits.Lister)
	return ok
}

// mapInsertFunction is ext's internal transformMap fold step (ext/comprehensions.go).
const mapInsertFunction = "cel.@mapInsert"

// foldStepCost is the fold rule's charge: for list _+_ it is the sum, over every element
// of the appended operand (args[1]), of ceil(f * resultBytes(element)); for @mapInsert it
// is the same sum over args[1:] (the transformMapEntry form passes a map as its single
// extra argument, which resultBytes still measures shallowly, at bytesPerElement per
// entry). The accumulator (args[0]) is never touched.
func foldStepCost(function string, args []ref.Val) uint64 {
	if function == mapInsertFunction {
		var sum uint64
		for _, v := range args[1:] {
			sum += uint64(math.Ceil(f * float64(resultBytes(v))))
		}
		return sum
	}
	if len(args) < 2 {
		return 0
	}
	appended := args[1]
	if l, ok := appended.(traits.Lister); ok {
		var sum uint64
		it := l.Iterator()
		for it.HasNext() == types.True {
			sum += uint64(math.Ceil(f * float64(resultBytes(it.Next()))))
		}
		return sum
	}
	return uint64(math.Ceil(f * float64(resultBytes(appended))))
}

// --- tier 1: overloads cel-go's own costCall switch prices correctly ---
//
// Copied from the pinned cel-go interpreter/runtimecost.go costCall switch (the explicit,
// non-default cases) so returning nil here defers to exactly those formulas and nothing
// else.
//
// An overload stays in tier 1 only if cel-go's formula (a) measures every argument the
// work or the allocation depends on and (b) the result is no larger than what the formula
// measured. Deferring to a formula that measures the wrong argument, or that measures the
// input while the result expands it, is an under-charge the result term would otherwise
// have corrected: string_format prices "%s".format([x]) by the *template* (args[0], often
// two bytes), not the list argument the formatted result actually depends on; 100 literal
// calls over a 9 MiB value cost 1,314 and retain 943.8 MB with no error if left in tier 1.
//
//	overload                                              | cel-go measures          | result                            | verdict
//	starts_with_string, ends_with_string                  | args[1] (prefix/suffix)  | bool                              | keep
//	string_to_bytes, bytes_to_string                      | args[0]                  | same length as args[0]            | keep
//	strings_quote                                         | args[0]                  | up to 6x args[0] (\u00XX escapes) | remove: generic rule + expander6 guard
//	string_format                                         | args[0] (the template)   | up to 2x argBytes(args[1])        | remove: generic rule + format guard
//	in_list                                               | args[1] (the list)       | bool                              | keep
//	less_*, greater_*, less_equals_*, greater_equals_*,   | min(size)                | bool                              | keep
//	  equals, not_equals                                  |                          |                                   |
//	add_string, add_bytes                                 | args[0] + args[1]        | exactly the sum                   | keep
//	matches, matches_string                               | both arguments           | bool                              | keep
//	contains_string                                       | both arguments           | bool                              | keep
//
// strings_quote and string_format fall to the generic rule (with their own guards below);
// every other row keeps deferring to cel-go's own formula.
var tier1OverloadIDs = map[string]bool{
	overloads.StartsWithString: true,
	overloads.EndsWithString:   true,
	overloads.StringToBytes:    true,
	overloads.BytesToString:    true,
	overloads.InList:           true,
	// in_map: cel-go's runtimecost.go switch has no explicit case for it, so it falls to
	// the `default: cost = 1` branch, which is correct for map membership (a hash lookup,
	// no allocation or work proportional to map size). Keep.
	overloads.InMap:               true,
	overloads.LessString:          true,
	overloads.GreaterString:       true,
	overloads.LessEqualsString:    true,
	overloads.GreaterEqualsString: true,
	overloads.LessBytes:           true,
	overloads.GreaterBytes:        true,
	overloads.LessEqualsBytes:     true,
	overloads.GreaterEqualsBytes:  true,
	overloads.Equals:              true,
	overloads.NotEquals:           true,
	overloads.AddString:           true,
	overloads.AddBytes:            true,
	overloads.Matches:             true,
	overloads.MatchesString:       true,
	overloads.ContainsString:      true,
}

// --- exemption list: O(1) or returns a reference into an existing value ---
//
// _==_, _!=_, _&&_, _||_, and _?_:_ are exempt for a reason beyond size: cel-go's planner
// special-cases them (interpreter/planner.go's planCall switch on
// operators.Equals/NotEquals/LogicalAnd/LogicalOr/Conditional), building a dedicated
// Interpretable (evalEq, evalNe, evalAnd, evalOr, a conditional attribute) whose real logic
// never goes through the Dispatcher/Bindings() path this package's guard decorator resolves
// against. Their registered "singleton binding" (common/stdlib/standard.go:
// "Special-cased in the interpreter") is an intentional non-functional stub that always
// reports "no such overload". Wrapping and re-dispatching through that stub silently breaks
// every typed equality/logical/ternary expression. _[_] (Index) is the same story
// (operators.Index in the same switch) and was already exempt. Cost-wise this changes
// nothing: _==_/_!=_ are already tier 1 (cel-go's own switch prices them); _&&_/_||_/_?_:_
// never reach costCall's InterpretableCall case at all.
var exemptFunctions = map[string]bool{
	"size": true, "type": true, "dyn": true, "_[_]": true,
	"optional.of": true, "optional.ofNonZeroValue": true, "optional.none": true,
	"optional.unwrap": true, "unwrapOpt": true, "hasValue": true, "value": true,
	"or": true, "orValue": true, "_?._": true, "_[?_]": true,
	"first": true, "last": true, "__orValueCompat__": true,
	"_==_": true, "_!=_": true, "_&&_": true, "_||_": true, "_?_:_": true,
}

// Estimator is Kyverno's interpreter.ActualCostEstimator: tier 1 (nil, cel-go's switch
// prices it), tier 1.5 (the apiserver's library.CostEstimator row, floored to at least the
// generic rule), and the generic rule for everything else. Delivered as a cel.ProgramOption
// through CostLib (costlib.go), which is how it reaches every program built from a Kyverno
// env, including ones the apiserver's composited compiler builds.
type Estimator struct {
	apiserver library.CostEstimator
}

var _ interpreter.ActualCostEstimator = (*Estimator)(nil)

// NewEstimator constructs the shared Kyverno cost estimator.
func NewEstimator() *Estimator {
	return &Estimator{}
}

// CallCost implements interpreter.ActualCostEstimator.
func (e *Estimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	if isFoldStep(function, args) {
		c := foldStepCost(function, args)
		return &c
	}
	// size() is exempt because it allocates nothing and returns an int. That holds for
	// lists, maps and bytes, whose Size() is a length read, but not for a string:
	// types.String.Size() is len([]rune(s)), an O(n) scan. Go lowers that to
	// RuneCountInString, so it allocates nothing and every regression test that asserts on
	// TotalAlloc misses it, while a comprehension calling size() on a large string burned
	// seconds of controller CPU for a few hundred charged units. Charge the scan at the
	// generic rule's argument rate, before the exemption.
	//
	// Only the pricing moves. exemptFunctions still covers size() in the guard decorator
	// (guards.go), so size() keeps its freedom from the argument cap and does not start
	// refusing a value larger than maxArgBytes.
	if function == "size" && len(args) == 1 {
		if str, ok := args[0].(types.String); ok {
			c := uint64(math.Ceil(f * float64(len(string(str)))))
			if c < 1 {
				c = 1
			}
			return &c
		}
	}
	if exemptFunctions[function] {
		return nil
	}
	if overloadID != "" && tier1OverloadIDs[overloadID] {
		return nil
	}
	if overloadID != "" {
		if apiCost := e.apiserver.CallCost(function, overloadID, args, result); apiCost != nil {
			generic := genericCost(args, result)
			c := *apiCost
			if generic > c {
				c = generic
			}
			return &c
		}
	}
	c := genericCost(args, result)
	if resultIsGuardRefusal(result) {
		// The guard decorator already refused this call before the real implementation
		// ran (guardTable, or the argument cap in guards.go's wrapped closure): result is
		// its ErrGuardRefused-wrapped error, not a value the call produced. Charging a
		// function-specific floor on top -- sized from the arguments, as if the call had
		// actually run -- would double-charge a call that already cost ~0 real allocation,
		// and can be large enough on its own to trip the per-program cost limit, masking
		// the guard's specific refusal text behind a generic cost-limit cancellation: a
		// test that only checks "an error occurred" cannot tell a masked guard from a
		// working one. Only genericCost's ordinary argument/result charge applies here.
		return &c
	}
	if floor, ok := parserFloor(function, args); ok && floor > c {
		c = floor
	}
	return &c
}

// resultIsGuardRefusal reports whether result is the guard decorator's own refusal for this
// call (guards.go's guardError, or the argument-cap check in its wrapped closure), both of
// which wrap ErrGuardRefused.
func resultIsGuardRefusal(result ref.Val) bool {
	err, ok := result.(error)
	if !ok {
		return false
	}
	return errors.Is(err, ErrGuardRefused)
}

// parserFloor: the generic rule's result term is shallow (a nested parse result -- a
// deeply structured JSON, YAML, or certificate document -- has shallow size 1 regardless of
// how much it actually retains). unmarshal, x509.decode, and yaml's parse are charged
// max(generic, expansion x f x len(input)) instead, so a parse that expands the input is
// priced by what it actually allocates rather than by its shallow result size. The input
// index is per function, matching the guard table's own parserGuard indices: unmarshal's
// and yaml's parse's input is args[1] (args[0] is the Context receiver); x509.decode's is
// args[0].
func parserFloor(function string, args []ref.Val) (uint64, bool) {
	var idx int
	var expansion float64
	switch function {
	case "unmarshal":
		idx, expansion = 1, parseExpansion
	case "x509.decode":
		idx, expansion = 0, parseExpansion
	case "parse": // yaml.parse in policy text: MemberOverload on the yaml Context variable
		idx, expansion = 1, yamlParseExpansion
	default:
		return 0, false
	}
	if idx >= len(args) {
		return 0, false
	}
	var length int
	switch v := args[idx].(type) {
	case types.String:
		length = len(string(v))
	case types.Bytes:
		length = len(v)
	default:
		return 0, false
	}
	floor := uint64(math.Ceil(expansion * f * float64(length)))
	return floor, true
}

// genericCost is the generic rule: cost = max(1, ceil(f*argBytes) + ceil(f*resultBytes)).
// The argument term is the same walk the guard decorator ran before the call; the result
// term prices what the call created and is charged after the call, a rate limiter on the
// next step rather than a bound on this one (every function whose single result can exceed
// maxStepBytes has a pre-execution guard in guards.go).
func genericCost(args []ref.Val, result ref.Val) uint64 {
	cap := maxArgBytes()
	argTerm := math.Ceil(f * float64(argBytes(args, cap)))
	resultTerm := math.Ceil(f * float64(resultBytes(result)))
	cost := uint64(argTerm) + uint64(resultTerm)
	if cost < 1 {
		cost = 1
	}
	return cost
}
