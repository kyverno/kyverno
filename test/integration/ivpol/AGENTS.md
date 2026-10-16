# AGENTS.md — test/integration/ivpol

envtest integration tests for the ImageValidatingPolicy webhook routes. The package has two tiers, split by build
tag, and CI runs each tier in a different workflow. Read this before moving a test between files or changing either
workflow.

## Two tiers, two build tags

- **`handler_test.go`** is `//go:build integration`. It owns `TestMain`, `newIvpol`/`ivpolSpec` and the pod helpers,
  and it never needs network: tests that must reach verification point a pod at `unverifiableImage`
  (`registry.invalid/...`). It runs on every PR in the `Framework tests (ivpol)` leg of `check-framework.yaml`.
- **`registry_test.go`** is `//go:build integration && registry`. It pulls real images from ghcr.io (and Rekor for
  keyless) and runs real cosign and notary verification. It runs only in `check-framework-registry.yaml`: nightly,
  on manual dispatch, and on PRs that touch this directory, `test/integration/framework/` or the workflow.

The registry tests compile together with `handler_test.go` and call the shared framework helpers
(`helpers.go`, `setup.go`), so a change to either can break them. That is why the registry workflow watches this
whole directory and the whole framework package, not just `registry_test.go` and `framework/ivpol.go`.

Run the registry tier locally with `go test -count=1 -tags="integration registry" ./test/integration/ivpol/...`.

## The registry workflow derives its test list from the build tags

`check-framework-registry.yaml` takes the files that build with `integration registry` but not with `integration`,
greps their `func Test...` names, and runs only those. A new registry test needs no workflow change, but it must
live in a file behind the `registry` tag, or the gate below will not count it.

## An unreachable registry skips; the workflow fails on "nothing verified"

`requireImageReachable` (and `pinnedByDigest`, which calls it) calls `t.Skipf` when an image can't be fetched, so a
developer without egress sees a skip, not a failure. The workflow compensates: it emits a warning for every skipped
test and fails when no registry test passed. It uses `-count=1` because a cached pass would report success without
contacting any registry. Keep both properties if you touch the step script.

## Validate tests pin images by digest

`verifyDigest` defaults to true, so the validating route denies an image passed by tag with
`... does not have a digest` before any signature check runs. In a cluster the mutating route pins tags first;
these tests do the same with `pinnedByDigest`. Passing a tag to a validate test makes a deny test pass for the wrong
reason and an admit test fail.

## Deny tests assert why they denied

A deny test must assert the denial message, not just `allowed == false`. Policies built on `newIvpol` deny with
`signatureDenial` (the validation message `ivpolSpec` sets), so the deny tests assert that message is present. Any
other denial, such as the digest check, then fails the test instead of satisfying it.
