# Measuring selected GitHub Actions workflow latency

`scripts/ci-timing-report.py` is a read-only collector and offline reporter for
the measurement work in [#18115](https://github.com/kyverno/kyverno/issues/18115).
It does not change required checks, runners, test coverage, or workflow execution.

## Collect an explicit cohort

Requires Python 3.10+ and an authenticated `gh` CLI with permission to read the
repository's Actions metadata. No Python dependencies or workflow secrets are
needed. All GitHub requests use `gh api` GET operations; `report` is offline.

```sh
python3 scripts/ci-timing-report.py collect \
  --repo kyverno/kyverno \
  --workflow check-unit-tests.yaml \
  --workflow check-codegen.yaml \
  --event push --branch main \
  --since 2026-10-21T00:00:00Z --until 2026-10-22T00:00:00Z \
  --output /tmp/kyverno-ci-snapshot.json

python3 scripts/ci-timing-report.py report /tmp/kyverno-ci-snapshot.json
python3 scripts/ci-timing-report.py report /tmp/kyverno-ci-snapshot.json --format json
```

For PR workflows use `--event pull_request` and omit `--branch`, unless filtering
by the PR's **head branch** is intentional. Choose workflow filenames explicitly;
workflow display names are not unique. Repeat `--workflow` for every workflow
needed for the comparison. Do not mix events, branches or workflow sets when
comparing cohorts. The window selects **workflow creation time**, not completion
or latest update time. A workflow created just outside a window can cause a
partial SHA cohort; keep missing cohorts visible and choose the window accordingly.

The collector paginates both runs and jobs. It reads jobs for each run's explicit
`run_attempt`, so older attempts cannot be mistaken for the current attempt. It
refuses run searches reaching GitHub's 1,000-result cap; narrow the window rather
than reporting a silently truncated sample. Collection uses several requests per
run; start with a narrow window and respect GitHub API rate limits. If collection
fails, no new output is written. Keep snapshots outside the repository.

## Metric definitions

- **Cohort:** one commit SHA for the selected event and workflows. This is not
  necessarily a unique PR, nor proof that all required checks were selected.
- **Run selection:** latest created run for each workflow/SHA; higher run ID and
  attempt break ties. A failed or pending newer run never falls back to an older
  successful run. JSON output records selected run IDs, attempts, URLs and outcomes.
- **Observed SHA wall time:** earliest collected workflow creation for the SHA to
  the latest job completion across selected latest attempts. Includes waits before
  reruns or replacement runs; it is not active test execution time. Workflow
  `updated_at` is never used as an execution timestamp.
- **Latest-attempt runner occupancy:** sum of elapsed job durations, including
  parallel jobs. Excludes previous attempts. This is an occupancy proxy, not billed
  minutes, worker CPU time or dollars; it does not account for pricing multipliers.
- **Creation-to-first-job delay:** earliest workflow creation to first job start
  in the selected attempts. GitHub metadata does not reliably separate approval,
  queue and retry waits; this tool does not invent that breakdown.
- **Percentiles:** nearest-rank p50/p90 on complete successful SHA cohorts only.
  Missing workflows, incomplete job metadata, failures, cancellations, non-success
  conclusions and action-required runs are listed but excluded. A workflow with
  only skipped jobs is not considered measured execution. Successful jobs without
  valid timestamps make the cohort incomplete.

JSON also includes per-workflow wall distributions for these same successful
cohorts. For insight into individual failures, inspect the recorded run URLs;
failure and cancelled-run costs are not included in the occupancy distribution.
Job steps remain in the saved snapshot for further analysis.

GitHub collection is not an atomic snapshot: runs can change while it proceeds.
Collect after the window's runs settle for a reproducible comparison. The report
fails closed on missing or inconsistent successful-job metadata, but cannot
detect an entire SHA absent from every selected workflow. Changing workflow
filters across time also changes the population.

## Interpretation

This reports **selected-workflow timing**, not all-required-checks completion or
merge readiness. External/manual statuses, policy approval and branch-protection
requirements are outside this snapshot. Only use a whole-PR label after independently
verifying the complete required-check inventory for that SHA.

The report always displays its cohort and success counts and flags samples below
100 successful cohorts. Do not infer a stable p90, speedup or cost neutrality from
a small successful-only timing population. Compare equivalent before/after windows
and report incomplete, failed and cancelled cohorts alongside latency. Retain cold
cache, dependency and shared-code changes rather than selecting only fast CI-only
changes. No pre/post optimization inference is made automatically.

## Regression tests

```sh
python3 scripts/test-ci-timing-report.py
```

Tests use synthetic snapshots and mocked GitHub responses. They cover parallel
workflows, SHA matching, missing jobs/workflows, reruns, non-success outcomes,
pagination, truncation, and the percentile definition without credentials/network.
