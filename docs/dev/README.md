# Developer Documentation (Advanced Topics)

This folder contains developer documentation.

To get started see: [DEVELOPMENT.md](../../DEVELOPMENT.md).

When you are ready to contribute, you can select issue at [Good First Issues](https://github.com/orgs/kyverno/projects/10). 

For release instructions, see: [create-a-release.md](releases/create-a-release.md).

## Performance benchmarks

A `perf` job in the post-merge `check-tests` workflow
(`.github/workflows/check-tests.yaml`, triggered on push to `main` and
`release-*`) runs a fast, cluster-free allocation-regression check against
the CEL admission path. It doesn't run per pull request: if a regression
lands on `main`, the `perf` job files or updates a workflow-failure issue
for a maintainer to triage, rather than blocking a PR, which is what lets
the check use tight headroom (see the ratchet process below).

The check covers four boundaries, all under `pkg/cel/policies/*` and
`pkg/webhooks/resource/{vpol,mpol}`:

- **vpol engine** (`pkg/cel/policies/vpol/engine`): `BenchmarkEngineHandleVpol`,
  the validating-policy engine's `Handle` call against N always-true compiled
  policies (subbenchmarks for N=1, 16, 64) and a realistically sized (~50 KB)
  synthetic pod.
- **mpol engine** (`pkg/cel/policies/mpol/engine`): `BenchmarkEngineHandleMpol`,
  the mutating-policy engine's `Handle` call for a single mutation policy.
- **vpol webhook handler** (`pkg/webhooks/resource/vpol`):
  `BenchmarkVpolHandlerValidate`, the full `ValidateClustered` path -
  `RequestFromAdmission`, engine `Handle`, the async audit fan-out, and
  response construction.
- **mpol webhook handler** (`pkg/webhooks/resource/mpol`):
  `BenchmarkMpolHandlerMutate`, the full `MutateClustered` path, including
  JSON-patch generation.

`gpol`, `dpol`, and `ivpol` are deliberately excluded:

- **gpol** only enqueues an `UpdateRequest` at admission - it has no
  synchronous per-request loop to gate.
- **dpol** is a background/scheduled cleanup engine that never runs at
  admission.
- **ivpol** is I/O-bound: signature and attestation fetches dominate its
  cost, so its allocs/op scales with image count rather than policy logic,
  making it a low-signal, flaky gate target.

Run the benchmarks and gate them against the committed ceilings with:

```shell
make check-perf
```

This runs `go test -bench=. -benchmem` scoped to the four packages above,
then compares each benchmark's `allocs/op` and `B/op` against the ceilings
in `scripts/bench/thresholds.txt`. The check fails if a benchmark exceeds
its ceiling, or if a benchmark listed in the thresholds file is missing from
the test output (for example, because it was renamed or removed).

### Ratcheting the ceilings

The ceilings are fixed numbers committed to `scripts/bench/thresholds.txt`,
not an auto-adjusting baseline, so they need a deliberate ratchet at three
points:

1. **Perf PRs.** If your PR intentionally reduces allocations at one of the
   four boundaries, run `make bench-baseline` and commit the tightened
   ceilings in the same PR.
2. **Allocation-changing PRs.** If your PR legitimately increases allocation
   counts at one of these boundaries, run `make bench-baseline`, commit the
   regenerated ceilings in the same PR, and add a one-line justification to
   your PR description explaining the change.
3. **Release-branch cuts.** At each release-branch cut, the release owner
   re-runs `make bench-baseline` on linux/amd64 and commits the result. This
   bounds ceiling staleness to one release cycle even if individual ratchets
   are missed.

`make bench-baseline` reruns the benchmarks and regenerates
`scripts/bench/thresholds.txt` by applying the headroom formula (+5% on
allocs/op with a minimum of +2, +15% on bytes/op, rounded up) to the
measured values. Both webhook handler benchmarks are deterministic, but for
different reasons:

- The vpol handler already awaits its own audit work in production - it
  runs under a `wait.Group` with a deferred `Wait()` (see
  `pkg/webhooks/resource/vpol/handler.go`) - so `ValidateClustered` doesn't
  return until that work is done, and `BenchmarkVpolHandlerValidate` needs
  no benchmark-side synchronization at all.
- The mpol handler fires its audit (and, for non-dry-run requests, a
  mutate-existing update-request check) in unwaited goroutines instead, so
  `BenchmarkMpolHandlerMutate` adds its own synchronization: a dry-run
  request to skip the update-request goroutine entirely, plus a fake event
  sink that signals when the audit goroutine's last observable side effect
  completes (see that benchmark's doc comment for the full mechanism).

Because both are deterministic, a single headroom applies uniformly across
all six gated benchmarks. `make bench-baseline` only re-measures benchmarks
already gated in `scripts/bench/thresholds.txt` - it never promotes a new,
exploratory benchmark into the gate as a side effect of ratcheting the
existing rows. Because allocation counts differ across GOOS/GOARCH, the
committed ceilings must come from linux/amd64: run `make bench-baseline`
inside a `golang` Docker container if you're on another platform, so your
numbers match what CI will measure.
