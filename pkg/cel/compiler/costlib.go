package compiler

import (
	"math"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
)

// costLib is Kyverno's env-level cel.Library: pricing (the Estimator), the per-program
// limit and interrupt frequency, the argument-cap/guard decorator, the replacement
// regex-literal validator, and the 32 replacement overload trackers. Delivered as a single
// library so it reaches every program built from a Kyverno env through cel-go's normal
// env/library merge (cel/library.go, cel/env.go), including programs the apiserver's
// composited compiler builds and checks it cannot see.
type costLib struct {
	mu  sync.Mutex
	env *cel.Env // captured by CompileOptions, read by ProgramOptions; see the comment there
}

// CostLib constructs the cost library. Every Kyverno env constructor (env.go's NewEnv /
// BaseEnvSet) registers exactly one of these, last.
func CostLib() cel.Library {
	return &costLib{}
}

// CostLibOption gives every environment it is applied to its own costLib instance.
//
// costLib captures the environment in CompileOptions and reads it back in ProgramOptions to
// build the guard decorator's binding registry, which looks like it would break when one
// instance is shared across an environment.EnvSet's two environments. It does not:
// cel.Lib applies CompileOptions and then reads ProgramOptions immediately, in the same
// per-environment call (cel-go cel/library.go), so each environment captures itself before
// the next one runs. Verified by measurement on an EnvSet's two environments, which refuse
// identically with and without this option.
//
// It is kept because it removes the dependency on that ordering detail: nothing in the
// Library contract promises ProgramOptions is read eagerly, and if cel-go ever defers it the
// shared-instance form would silently resolve guard bindings against the wrong environment.
func CostLibOption() cel.EnvOption {
	return func(e *cel.Env) (*cel.Env, error) {
		return cel.Lib(CostLib())(e)
	}
}

// LibraryName implements cel.SingletonLibrary, so a second registration (e.g. through a
// chained Extend) is a no-op rather than a duplicate-function panic.
func (l *costLib) LibraryName() string { return "kyverno.cost" }

// CompileOptions registers the replacement regex-literal validator and one env option
// whose only job is to capture the *cel.Env this library ends up attached to. Since
// Kyverno always registers CostLib last (every env constructor's final option; see the
// chokepoint comment in env.go), by the time this runs every other library's functions are
// already on the env, so the capture below is not a snapshot-in-progress -- it is the same
// env, complete, that ProgramOptions() reads a moment later (cel/library.go's Lib() calls
// all CompileOptions then reads ProgramOptions() in the same step). This is load-bearing:
// an incomplete env would build an incomplete binding registry and a guard decorator that
// silently fails to wrap some functions.
func (l *costLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		func(e *cel.Env) (*cel.Env, error) {
			l.mu.Lock()
			l.env = e
			l.mu.Unlock()
			return e, nil
		},
		cel.ASTValidators(regexLiteralValidator{}),
	}
}

// ProgramOptions implements the Library interface: pricing, the per-program limit and
// interrupt frequency, constant-regex optimization, the guard decorator, and the 32
// replacement overload trackers.
func (l *costLib) ProgramOptions() []cel.ProgramOption {
	l.mu.Lock()
	env := l.env
	l.mu.Unlock()

	reg := newBindingRegistry(env)
	trackerOpts := replacementTrackerOptions()
	opts := make([]cel.ProgramOption, 0, 5+len(trackerOpts))
	opts = append(opts,
		cel.CostTracking(NewEstimator()),
		cel.CostLimit(perProgramLimit()),
		cel.InterruptCheckFrequency(interruptEvery),
		cel.EvalOptions(cel.OptOptimize),
		cel.OptimizeRegex(guardPreservingRegexOptimizations()...),
		cel.CustomDecoratorV2(newGuardDecorator(reg)),
	)
	opts = append(opts, trackerOpts...)
	return opts
}

// --- replacement overload trackers ---
//
// ext.Regex v1, ext.Lists v3, ext.Sets, and ext.Encoders install cel.CostTrackerOptions
// for the overload IDs below; a tracker short-circuits both the Estimator and cel-go's own
// switch, so none of these results ever got the generic rule's result term.
// OverloadCostTracker assigns into a map keyed by overload ID and cel-go applies program
// options in registration order, so registering a tracker for the same ID here, after
// CostLib's own env-capture step runs last among Kyverno's own options, replaces the ext
// library's entry. Each replacement below is a small, version-pinned reimplementation of
// that specific ext tracker's formula, used only to decide whether the original was a
// measurement or a sentinel -- never to avoid the generic rule: the replacement always
// returns at least max(original, generic) for a measurement, and the generic rule alone for
// a sentinel (>= sentinelFloor).

// sentinelFloor: a value at or above this is treated as a sentinel (cel-go's own
// json.encode tracker returns math.MaxUint64 unconditionally), not a measurement, so a
// plain max() never lets that sentinel silently determine a real charge.
const sentinelFloor = uint64(1) << 62

// regexStringLengthCostFactor is common.RegexStringLengthCostFactor (0.25): the weight the
// ext.Regex trackers give the pattern's own length, separate from f (StringTraversalCostFactor).
const regexStringLengthCostFactor = 0.25

// callCost is ext's own uint64(1) base dispatch cost (ext/costs.go), added by every
// replica below to match the originals term for term.
const extCallCost = uint64(1)

func satAdd(a, b uint64) uint64 {
	if a > math.MaxUint64-b {
		return math.MaxUint64
	}
	return a + b
}

func satMul(a, b uint64) uint64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxUint64/b {
		return math.MaxUint64
	}
	return a * b
}

// allocatingListCallCost mirrors ext's trackAllocatingListCall: a cost factor times a
// size, plus the call and list-allocation base costs.
func allocatingListCallCost(costFactor float64, size uint64) uint64 {
	if costFactor < 0 {
		costFactor = 1
	}
	return satAdd(satAdd(uint64(float64(size)*costFactor), extCallCost), 10 /* ListCreateBaseCost */)
}

// listSelfCompareCost mirrors ext's trackListSelfCompare: a worst-case O(n^2) self-compare
// (distinct, sort), with a small string-traversal premium when the elements are strings.
func listSelfCompareCost(l traits.Lister) uint64 {
	sz := shallowSize(l)
	costFactor := 2.0
	if sz == 0 {
		return allocatingListCallCost(costFactor, 0)
	}
	if elem, ok := firstElement(l); ok {
		switch elem.(type) {
		case types.String, types.Bytes:
			costFactor += f
		}
	}
	return allocatingListCallCost(costFactor, satMul(sz, sz))
}

func firstElement(l traits.Lister) (ref.Val, bool) {
	it := l.Iterator()
	if it.HasNext() != types.True {
		return nil, false
	}
	return it.Next(), true
}

// regexSearchCost mirrors the shared shape of ext.Regex's extract/extractAll/replace
// trackers: (targetSize+1)*f * (patternSize+1)*regexStringLengthCostFactor.
func regexSearchCost(target, pattern ref.Val) float64 {
	targetCost := float64(shallowSize(target)+1) * f
	regexCost := float64(shallowSize(pattern)+1) * regexStringLengthCostFactor
	return targetCost * regexCost
}

func arg(args []ref.Val, i int) ref.Val {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// replacementTrackerOptions builds the 32 replacement trackers. Deliberately not replaced:
// ext.Math's math_@min_list_*/math_@max_list_* trackers, whose results are scalars and
// retain nothing.
func replacementTrackerOptions() []cel.ProgramOption {
	type replacement struct {
		overloadID string
		original   func(args []ref.Val, result ref.Val) uint64
	}

	replacements := []replacement{
		// ext.Lists v3 (ext/lists.go): output-size trackers.
		{"list_slice", func(args []ref.Val, result ref.Val) uint64 {
			return allocatingListCallCost(1, shallowSize(result))
		}},
		{"lists_range", func(args []ref.Val, result ref.Val) uint64 {
			return allocatingListCallCost(1, shallowSize(result))
		}},
		{"list_reverse", func(args []ref.Val, result ref.Val) uint64 {
			return allocatingListCallCost(1, shallowSize(result))
		}},
		{"list_distinct", func(args []ref.Val, result ref.Val) uint64 {
			l, ok := arg(args, 0).(traits.Lister)
			if !ok {
				return 0
			}
			return listSelfCompareCost(l)
		}},
		// ext.ListsVersion(3) (Kyverno's pinned version) uses the "legacy" flatten
		// tracker, priced on the input's top-level size, not the result's.
		{"list_flatten", func(args []ref.Val, result ref.Val) uint64 {
			depth := 1.0
			if n, ok := arg(args, 1).(types.Int); ok {
				depth = float64(n)
			}
			return allocatingListCallCost(depth, shallowSize(arg(args, 0)))
		}},
		{"list_flatten_int", func(args []ref.Val, result ref.Val) uint64 {
			depth := 1.0
			if n, ok := arg(args, 1).(types.Int); ok {
				depth = float64(n)
			}
			return allocatingListCallCost(depth, shallowSize(arg(args, 0)))
		}},

		// ext.Sets (ext/sets.go): comparison-count trackers.
		{"list_sets_contains_list", func(args []ref.Val, _ ref.Val) uint64 {
			return satAdd(extCallCost, uint64(float64(shallowSize(arg(args, 0))*shallowSize(arg(args, 1)))*1))
		}},
		{"list_sets_intersects_list", func(args []ref.Val, _ ref.Val) uint64 {
			return satAdd(extCallCost, uint64(float64(shallowSize(arg(args, 0))*shallowSize(arg(args, 1)))*1))
		}},
		{"list_sets_equivalent_list", func(args []ref.Val, _ ref.Val) uint64 {
			return satAdd(extCallCost, uint64(float64(shallowSize(arg(args, 0))*shallowSize(arg(args, 1)))*2))
		}},

		// ext.Regex v1 (ext/regex.go): search-cost trackers.
		{"regex_extract_string_string", func(args []ref.Val, result ref.Val) uint64 {
			total := float64(extCallCost) + regexSearchCost(arg(args, 0), arg(args, 1)) + float64(shallowSize(result))
			return uint64(math.Ceil(total))
		}},
		{"regex_extractAll_string_string", func(args []ref.Val, result ref.Val) uint64 {
			total := float64(extCallCost) + regexSearchCost(arg(args, 0), arg(args, 1)) + float64(shallowSize(result)) + 10
			return uint64(math.Ceil(total))
		}},
		{"regex_replace_string_string_string", func(args []ref.Val, result ref.Val) uint64 {
			total := float64(extCallCost) + regexSearchCost(arg(args, 0), arg(args, 1)) + float64(shallowSize(result))
			return uint64(total) // ext's replaceCostTracker truncates, does not ceil
		}},
		{"regex_replace_string_string_string_int", func(args []ref.Val, result ref.Val) uint64 {
			total := float64(extCallCost) + regexSearchCost(arg(args, 0), arg(args, 1)) + float64(shallowSize(result))
			return uint64(total)
		}},

		// ext.Encoders (ext/encoders.go): byte-scan trackers.
		{"base64_decode_string", func(args []ref.Val, _ ref.Val) uint64 {
			return satAdd(uint64(math.Ceil(float64(shallowSize(arg(args, 0)))*f)), extCallCost)
		}},
		{"base64_encode_bytes", func(args []ref.Val, _ ref.Val) uint64 {
			return satAdd(uint64(math.Ceil(float64(shallowSize(arg(args, 0)))*f)), extCallCost)
		}},
		// json_encode_dyn's original always returns math.MaxUint64: mirrored exactly, so
		// the sentinel branch below is what actually prices it.
		{"json_encode_dyn", func(args []ref.Val, _ ref.Val) uint64 {
			return math.MaxUint64
		}},
	}

	// list_<T>_sort / list_<T>_sortByAssociatedKeys: 16 IDs generated from the same
	// comparableTypes list ext/lists.go uses, via each type's own TypeName(), so a cel-go
	// type-name change is reflected automatically rather than hardcoded.
	typeNames := comparableTypeNames()
	grown := make([]replacement, len(replacements), len(replacements)+2*len(typeNames))
	copy(grown, replacements)
	replacements = grown
	for _, t := range typeNames {
		typeName := t
		replacements = append(replacements,
			replacement{"list_" + typeName + "_sort", func(args []ref.Val, _ ref.Val) uint64 {
				l, ok := arg(args, 0).(traits.Lister)
				if !ok {
					return 0
				}
				return listSelfCompareCost(l)
			}},
			replacement{"list_" + typeName + "_sortByAssociatedKeys", func(args []ref.Val, _ ref.Val) uint64 {
				l, ok := arg(args, 1).(traits.Lister)
				if !ok {
					return 0
				}
				return listSelfCompareCost(l)
			}},
		)
	}

	trackerOpts := make([]interpreter.CostTrackerOption, 0, len(replacements))
	for _, r := range replacements {
		original := r.original
		trackerOpts = append(trackerOpts, interpreter.OverloadCostTracker(r.overloadID, func(args []ref.Val, result ref.Val) *uint64 {
			orig := original(args, result)
			generic := genericCost(args, result)
			var c uint64
			if orig >= sentinelFloor {
				c = generic
			} else if orig > generic {
				c = orig
			} else {
				c = generic
			}
			return &c
		}))
	}
	return []cel.ProgramOption{cel.CostTrackerOptions(trackerOpts...)}
}

// --- legacy composited-compiler fallback ---
//
// k8s.io/apiserver's own admission plugin (plugin/cel/compile.go's createEnvForOpts, used by
// cel.NewCompositedCompiler / plugincel.NewCompiler) re-Extends whatever EnvSet it is given
// with environment.StrictCostOpt, whose ProgramOptions include cel.CostTracking(&library.
// CostEstimator{}). cel.CostTracking is a plain field assignment (interpreter.CostTracker.
// Estimator = ...), last-registration-wins, so that later Extend silently replaces CostLib's
// Estimator -- taking the fold rule, the generic rule, and the parser floor with it -- for
// every call whose OverloadID() is empty (every dyn call: exactly the class this whole
// design exists to price). This is the legacy kyverno.io/v1 validate.cel/celPreconditions
// path and the VAP/MAP emulation (pkg/admissionpolicy), reached through
// cel.NewCompositedCompiler; not the ordinary Kyverno chokepoints, which never call it.
//
// cel.CostTrackerOptions is not last-wins: it populates interpreter.CostTracker.
// overloadTrackers, a map, and costCall (interpreter/runtimecost.go) checks that map BEFORE
// ever consulting Estimator. A tracker registered at the empty overload ID therefore
// survives a later CostTracking swap regardless of registration order, because the two
// mechanisms are different fields on the same struct.
//
// LegacyDynCostFallback is that tracker, registered only by pkg/admissionpolicy/compiler.go
// (never added to costLib's own ProgramOptions, which would apply it to every env, not just
// the legacy one -- see below). FunctionTracker's signature carries no function name, so it
// cannot replicate the fold rule's isFoldStep dispatch or the parser floor, which are keyed
// by function; it applies the generic rule (argBytes + resultBytes, both cap-bounded)
// unconditionally to every empty-overload-ID call. That over-charges the small number of
// shapes the fold rule would otherwise price more precisely (walking a growing accumulator
// under this fallback, rather than charging only the appended element) -- a coarser refusal,
// not an under-charge, and the legacy path is admin-authored text (Trust tiers), where a
// coarser refusal is the accepted trade-off (U4). Registering it on every env instead of
// only here would be the opposite mistake: it would preempt CostLib's own fold-aware
// Estimator for every ordinary dyn call on vpol/mpol/dpol/gpol too, since overloadTrackers
// beats Estimator regardless of which one is "correct" -- silently reintroducing the
// quadratic accumulator walk the fold rule exists to avoid on every real Kyverno policy.
func LegacyDynCostFallback() cel.ProgramOption {
	return cel.CostTrackerOptions(interpreter.OverloadCostTracker("", func(args []ref.Val, result ref.Val) *uint64 {
		c := genericCost(args, result)
		return &c
	}))
}

// comparableTypeNames returns the runtime type names cel-go's ext.Lists uses to build its
// 16 sort/sortByAssociatedKeys overload IDs, read from the pinned cel-go's own type values
// rather than hardcoded strings.
func comparableTypeNames() []string {
	types := []*cel.Type{
		cel.IntType, cel.UintType, cel.DoubleType, cel.BoolType,
		cel.DurationType, cel.TimestampType, cel.StringType, cel.BytesType,
	}
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.TypeName()
	}
	return names
}
