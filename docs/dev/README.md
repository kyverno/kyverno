# Developer Documentation (Advanced Topics)

This folder contains developer documentation.

To get started see: [DEVELOPMENT.md](../../DEVELOPMENT.md).

When you are ready to contribute, you can select issue at [Good First Issues](https://github.com/orgs/kyverno/projects/10). 

For release instructions, see: [create-a-release.md](releases/create-a-release.md).

## Performance benchmarks

Every pull request runs a fast, cluster-free allocation-regression check
against the admission hot path. The check covers three boundaries:

- **Engine** (`pkg/engine`): a single-policy `Validate` pass through
  `BenchmarkEngineValidate`, with metrics recording forced on so the
  benchmark reflects the production admission path.
- **Policy cache** (`pkg/policycache`): `BenchmarkGetPolicies`, which looks
  up policies for an admission request at several cache sizes.
- **Handler** (`pkg/webhooks/resource`): the full webhook `Validate` call in
  `BenchmarkHandlerValidate`, from request unmarshaling through engine
  validation and metrics recording.

Run the benchmarks and gate them against the committed ceilings with:

```shell
make check-perf
```

This runs `go test -bench=. -benchmem` scoped to the three packages above,
then compares each benchmark's `allocs/op` and `B/op` against the ceilings
in `scripts/bench/thresholds.txt`. The check fails if a benchmark exceeds
its ceiling, or if a benchmark listed in the thresholds file is missing from
the test output (for example, because it was renamed or removed).

If your PR legitimately changes allocation counts at one of these
boundaries, re-run `make test-perf` to capture new measurements, update
`scripts/bench/thresholds.txt` in the same PR, and add a one-line
justification to your PR description explaining the change.
