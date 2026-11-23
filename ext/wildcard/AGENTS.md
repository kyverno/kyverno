# AGENTS.md — ext/wildcard

Shell-style glob matching (`*`, `?`) for every wildcard-bearing field in the project. `Match` is a shared leaf
helper with call sites across `pkg/engine`, `pkg/utils/match`, `pkg/pss`, `pkg/config` and the
CLI, and it is reached from the CEL side too — `pkg/background/mpol/processor.go` matches a MutatingPolicy's
`matchConstraints` API groups, versions and resources through it. Treat a change here as touching all of them.

## The matcher must stay iterative

`Match` used to delegate to a third-party doubly-recursive backtracking matcher. Its `*` case forked two recursive
calls per non-matching tail, so some shapes were exponential: one call with a pattern of the form `*a*a…*b` against
a 40-rune name of `a`s does not finish in 20 seconds at fourteen stars.

The replacement is an iterative, single-backtrack, two-pointer matcher: each `*` is backtracked to at most once, and
every backtrack advances the string index it resumes from, so runtime is `O(len(name) * len(pattern))` with no
recursion. **Do not reintroduce recursion here, and do not add a second backtrack point.** Either change
reintroduces the exponential class.

## `*` is tested before literal equality — this ordering is load-bearing

```go
case pIdx < len(p) && p[pIdx] == '*':
    // '*' is always a wildcard operator in the pattern, even when the
    // name's current rune is itself a literal '*' or '?'
case pIdx < len(p) && (p[pIdx] == '?' || p[pIdx] == s[sIdx]):
```

Swap those two cases and a pattern `*` matched against a name whose current rune is also `*` or `?` is consumed as a
one-rune literal instead of acting as the wildcard. An earlier version of the rewrite had exactly this bug, and
**four independent checks missed it** — the equivalence argument, the harness spec, the shipped sweep, and an ad-hoc
207k-pair sweep — because every name alphabet in them was metacharacter-free.

So: **any differential test here must put `*` and `?` in both operand alphabets**, not just the pattern's. The sweep
bounds in `match_reference_test.go` are also load-bearing for suite runtime — the reference implementation is the
old exponential one, so widening them "for thoroughness" hangs the suite.

## Budget exhaustion is reported as "no match"

`matchWithSteps` counts main-loop iterations against `maxMatchSteps` and gives up past it. `Match` returns a plain
`bool`, which cannot represent "unknown", so an over-budget pair is reported as not matching.

That makes the margin a contract, not a tuning knob. `maxMatchSteps` is sized at roughly twice the largest
legitimate shape (a 255-rune glob in its worst rewind form against a 256KiB annotation value is about 67M steps,
just under half the budget), so a shift in counting convention or a slightly larger legitimate input cannot silently
cross it. If you change the counting, re-derive the margin.

The worst rewind shape is one leading `*` followed by literals that keep matching until a final mismatch
(`*aaa…ab` against all `a`s), not alternating `*a*a…` — the alternating form bails early and is useless for
probing the bound.

Callers that accept unbounded input should cap it at admission rather than relying on the budget: a caller that
negates `Match`, or treats a non-match as "rule does not apply", turns an over-budget answer into a silent
behavior change rather than an error.
