# Reviewing a PR in this repo

Kyverno-specific things to check before approving a PR, for a human or an agent doing review. This is not a
restatement of general good-review practice, and not a duplicate of [`.github/pr_documentation.md`](.github/pr_documentation.md)
(which is about what a PR *author* must document) — it's the project-specific checks that are easy to miss because
they aren't visible from the diff alone, plus what CI actually runs and doesn't.

## How review actually works here

Most of the line-by-line nitpicking on PRs in this repo comes from two bots — GitHub Copilot's reviewer and
CodeRabbit — not from human maintainers; a maintainer's own review is frequently a same-day approval with little or
no comment body, especially once CI and the bots are green. That makes the bot findings worth taking seriously, but
not at face value: authors here routinely push back on a bot's specific claim with evidence (a code citation, a
live-cluster reproduction) rather than applying the suggested diff automatically, and just as often the bot is
right and the fix lands as suggested. Do the same in reverse when reviewing — a bot comment (or an AI reviewer's
own comment) is a lead to verify against the actual code, not a verdict to rubber-stamp either way.

## Always

- **DCO**: every commit needs a `Signed-off-by` trailer matching its own author, not just *a* trailer somewhere in
  the range. `git log --format='%h %s%n%b' origin/main..HEAD | grep -n "Signed-off-by"` (from `AGENTS.md`'s own
  checklist) only shows which lines matched — it can't tell you which specific commit is missing one, or whether a
  trailer belongs to that commit's actual author. Check per commit instead:
  ```bash
  for c in $(git log --format=%H origin/main..HEAD); do
    author=$(git log -1 --format='%an <%ae>' "$c")
    git log -1 --format=%B "$c" | grep -qF "Signed-off-by: $author" \
      && echo "OK      $c  $author" \
      || echo "MISSING $c  $author"
  done
  ```
  This is a required, separate CI check (`DCO`) — a missing or mismatched trailer fails it regardless of how good
  the change is.
- **Generated/no-edit zones**: is the diff hand-editing anything in `ARCHITECTURE.md`'s "Generated / no-edit zones"
  table — `pkg/client/**`, `api/**/zz_generated.*.go`, `config/crds/**/*.yaml`, the CLI's own CRD copies under
  `cmd/cli/kubectl-kyverno/{config,data}/crds/**`, the Helm chart's CRD templates under
  `charts/kyverno/charts/crds/templates/{kyverno.io,reports.kyverno.io,wgpolicyk8s.io}/**`,
  `config/install-latest-testing.yaml`, `docs/user/crd/**`, any of the 4 `helm-docs`-generated `charts/**/README.md`
  (2 top-level, 2 nested subchart), `pkg/clients/**/*.generated.go`, `pkg/clients/**/interface.generated.go`? These
  should come from the corresponding `make codegen-*` target, not a hand edit — several of them (`pkg/clients/**`)
  carry no `// DO NOT EDIT` header, so this isn't always obvious from the file itself.
- **Codegen freshness, code *and* docs**: CI runs this as two separate required checks —
  `.github/workflows/check-codegen.yaml`'s `verify-codegen-code` job (`make codegen-all-code` then
  `make verify-codegen`) and `verify-codegen-docs` (`make codegen-all-docs` then the same check, which is just
  `git diff --exit-code`). Running `make codegen-all` locally covers both before pushing; running only
  `codegen-all-code` can pass one job and still fail the other.
- **CODEOWNERS coverage**: does the changed path have its listed owner as a reviewer, or at least aware?

## If the PR touches `api/`

- New types go to `v2alpha1`, never directly to `v1`/`v2`. At beta/stable tiers, attributes are additive-only
  within a version (never deleted or modified in place — deprecate and remove after the tier's notice period
  instead); alpha tiers have no such guarantee. See
  [docs/context/shared/api-versioning.md](docs/context/shared/api-versioning.md) for the full rules and a worked
  example of a version's actual lifecycle being messier than the general rule suggests in isolation.
- **A CRD schema change touches more files than `config/crds/`.** The generated CRD YAML for a given kind is
  duplicated into `config/crds/`, `cmd/cli/kubectl-kyverno/data/crds/` (the CLI's offline copy), the Helm chart
  under `charts/kyverno/charts/crds/templates/`, and the install manifest `config/install-latest-testing.yaml` —
  all four move together under `make codegen-all-code` (its `codegen-crds-all`, `codegen-cli-all`,
  `codegen-helm-all`, and `codegen-manifest-all` sub-targets each cover one). If a reviewer only checks `config/crds/`
  for a schema change, the other three can silently drift — `make verify-codegen` is the actual guarantee, not a
  manual diff of one directory.
- **A new/changed `+kubebuilder:deprecatedversion:warning="..."` marker has a hard 256-character limit** —
  it's a real Kubernetes API-server constraint on the CRD's `deprecationWarning` field, not a style guideline.
  `pkg/deprecations/deprecations_test.go`'s `TestWarningLength` asserts every generated warning stays under it;
  a marker that's too long fails that unit test, not just review.

## If the PR adds client/Kubernetes-API access

- New code should use `pkg/clients/dclient.Interface` for generic resource access, or a typed `pkg/clients/*`
  wrapper when the type is known — never construct a raw `pkg/client`/`k8s.io/client-go` client directly (accepting
  one of those packages' interface types as a parameter is fine). See
  [docs/context/shared/client-access.md](docs/context/shared/client-access.md).

## If the PR adds a feature flag

- `pkg/toggle` toggles are per-binary opt-in: the interface method existing doesn't mean the flag is reachable — it
  also needs `flagset.Func(...)` registered in every `cmd/*/main.go` that should expose it on the CLI (env-var
  access works independent of that registration). `AutogenV2` in `pkg/toggle/toggle.go` is a real example of a
  toggle that compiles and has an interface method but is registered and consumed nowhere — check the new flag
  isn't in the same state. See [pkg/toggle/AGENTS.md](pkg/toggle/AGENTS.md).

## Tests and CI checks to expect

- **Unit tests, CLI tests, `golangci-lint`, `go vet`/fmt/imports/unused-package checks, `Verify codegen (code)`,
  `Verify codegen (docs)`, `Chart-testing (lint)`, `Artifact Hub (lint)`, and `Ensure SHA pinned actions`** all run
  automatically on **every** PR against `main`/`release-*` — none of them are path-filtered, so they show up in the
  checks list regardless of whether the PR actually touches `charts/` or a workflow file (verified against each
  workflow's `on:` block in `.github/workflows/`; there's no `paths:` filter on any of them).
- **`Framework tests`** (Go integration tests per CEL policy kind: vpol/mpol/gpol/dpol/ivpol,
  `test/integration/<kind>/...`) is the one exception — `check-framework.yaml` has
  `paths-ignore: [docs/**, charts/**, **/*.md]`, so a PR that only touches those (like a docs-only PR) never
  triggers it. Its absence from the checks list on such a PR is expected, not a CI gap.
- **The ~1000+ chainsaw conformance suite in `test/conformance/chainsaw/` does *not* run automatically on a PR.**
  `.github/workflows/tests-conformance.yaml` is `workflow_call`-only; it's actually triggered either by
  `comment-conformance.yaml` (posting `/conformance` on the PR, `issue_comment` triggered) or by
  `check-tests.yaml` (only on push to `main`/`release-*`, i.e. after merge). For a PR whose correctness genuinely
  needs e2e verification before merge, a reviewer (or the author) needs to explicitly trigger it with a
  `/conformance` comment — it isn't a gate that runs on its own, and its absence from a PR's checks list doesn't
  mean it was skipped by CI, only that it was never asked to run.
- Logic changes still need unit tests regardless of chainsaw coverage; `kubectl-kyverno test` fixtures under
  `test/cli/` for CLI-observable behavior.
- Proof manifests in the PR description, per the PR template and
  [`.github/pr_documentation.md`](.github/pr_documentation.md), for anything user-facing.

## AI-assisted PRs

The org's [`AI_USAGE_POLICY.md`](https://github.com/kyverno/community/blob/main/AI_USAGE_POLICY.md) applies. AI
assistance should be disclosed via a `Co-authored-by`/`Assisted-by` commit trailer, per the PR template checklist —
its absence on a PR that reads as AI-assisted is worth a comment, not an automatic block.
