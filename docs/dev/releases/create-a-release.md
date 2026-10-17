# Creating a complete Kyverno release

This is the canonical release runbook. Wiki and website instructions should link
here rather than maintain a second procedure. The historical/current-behavior
documentation change is [#18064](https://github.com/kyverno/kyverno/pull/18064);
the unified automation implements
[#18063](https://github.com/kyverno/kyverno/issues/18063).

**This procedure applies only to commits containing the unified workflows and a
mapping for the application line in `config/releases.yaml`.** A release branch
without those changes still requires its existing application and separate chart
release procedure. Do not assume merging the automation on `main` changes older
tagged commits or release branches.

## Version policy

`config/releases.yaml` is read from the exact release commit. It maps an
application `major.minor` to independent `kyverno` and `kyverno-policies` chart
`major.minor` lines. Add the mapping once when opening a new application line;
there is no per-patch version-bump PR.

For example, with `1.20 -> kyverno: 3.10, kyverno-policies: 3.10`:

| Application tag and image tag | Kyverno chart | Policies chart |
| --- | --- | --- |
| `v1.20.2` | `3.10.2` | `3.10.2` |
| `v1.20.2-rc.3` | `3.10.2-rc.3` | `3.10.2-rc.3` |
| `v1.20.2-rc3` (legacy spelling) | `3.10.2-rc3` | `3.10.2-rc3` |

Use dotted prereleases (`-rc.3`) for new releases. Existing undotted tags are
accepted without normalization: `appVersion` and **every Kyverno-owned image
reference** retain the exact application tag. Build metadata (`+...`), malformed
SemVer identifiers, and missing mappings fail closed.

Both chart patches and prereleases belong exclusively to the application release
sequence. Do not allocate chart-only versions in the same sequence or push
`kyverno-chart-*` / `kyverno-policies-chart-*` tags. The unified Helm workflow is
called directly, not triggered by those tags. A conflicting existing chart,
binary, manifest, or image version is an error, never permission to overwrite it.

## Prerequisites

Before the first release from a line:

1. Merge the automation and its tests. After that merge, backport it and the
   appropriate mapping to each supported release branch through reviewed PRs.
   **Backports are a post-merge dependency, not completed by the main PR.**
2. Confirm the mapping, supported Kubernetes matrix, changelog, and source SHA.
3. Ensure Actions can write repository releases, GHCR packages, and `gh-pages`;
   keyless signing and SLSA require `id-token: write`. The workflows use the
   repository `GITHUB_TOKEN`, not a PAT to trigger another workflow.
4. Ensure the existing Krew release-bot service can submit stable releases.
   Acceptance here means a matching Krew index entry or open bot PR; merging the
   external Krew PR remains the Krew maintainers' responsibility.
5. Ensure the repository's Actions retention policy permits the requested 90-day
   retention. Recovery requires the original run's immutable candidate artifacts.

## Choose one entry point

### One application tag

After reviewing the exact commit, push one application tag to upstream. For
example, from an authorized release checkout:

```bash
git tag -a v1.20.2-rc.3 <full-reviewed-commit-sha> -m v1.20.2-rc.3
git push upstream refs/tags/v1.20.2-rc.3
```

The `releaser` workflow validates and publishes the application **and both
charts**. It does not require a chart metadata commit, chart tags, a separate
Helm dispatch, manual upload, or a separate Krew action.

**A pushed tag is already public before Actions starts.** This entry point gates
artifact publication, not tag creation. Never move or delete a failed public tag.

### Optional pre-tag validation

Use `prepare-release` to withhold the public tag until the same mandatory
validation succeeds:

```bash
gh workflow run prepare-release.yaml -R kyverno/kyverno \
  --ref release-1.20 \
  -f tag=v1.20.2-rc.3 \
  -f sha=<full-reviewed-commit-sha>
```

The supplied SHA must equal the dispatch branch HEAD resolved by GitHub. This
check is deliberate: tests, OIDC identity, and SLSA provenance must refer to the
same event commit, not an arbitrary SHA checked out from a newer workflow. If
the branch moved, dispatch on a branch containing the desired reviewed commit;
do not weaken the exact-SHA check.

After validation, the workflow creates the tag with `GITHUB_TOKEN` and calls
publication directly in the **same run**, reusing the tested candidate. A token-
created tag does not produce a second tag workflow. An existing tag pointing to
the same commit is an idempotent retry; a different target fails without moving it.

Both entry points share a per-tag concurrency lock with cancellation disabled.

## What is prepared, tested, and published

### Prepare once in the runner workspace

The runner verifies unit tests and codegen freshness against the original
checkout. It stages:

- CLI archives for the platform matrix in `.goreleaser.yml`, their checksums,
  source archive, and the native CLI used by tests;
- multi-platform OCI image layouts for admission, init, background, cleanup,
  reports, CLI, and readiness-checker images;
- CycloneDX SBOMs and the existing release Trivy scan;
- both charts, local `crds` and `grafana` subcharts, dependency references,
  regenerated Helm documentation, and the checked-in generated CRDs;
- an install manifest rendered from the staged Kyverno chart, plus its
  Flux-compatible OCI content layer and envelope.

Preparation preserves each chart's own `kubeVersion`, rather than imposing one
global constraint. External chart dependencies must match `Chart.lock`; their
exact versions are retained while local dependency versions and the lock digest
are updated in the isolated workspace. The candidate records downloaded
dependency checksums and the policy-library commit.

External archive SHA-256 pins are committed in `config/releases.yaml`.
Preparation verifies the first download against these pins, rejects byte changes
when updating local dependency references, and retains the original archives.
Package verification checks the prepared lock digest, retained archive checksums,
and every expanded dependency file. Helm's lock digest describes dependency
metadata, not archive bytes; it is not used as a substitute for those checksums.
When deliberately updating an external dependency, review and update its pin
together with `Chart.lock`; a missing or stale pin fails closed.

The charts retain their existing registry defaults, including the
`reg.kyverno.io` mirror, and global registry override behavior. Tests load both
canonical and chart-reference aliases of the same staged images; remote
verification also checks mirror references against the published GHCR digests.

`release-candidate` retains the complete `.release` directory, sealed by
`SHA256SUMS`. `release-workspace`, `kyverno.tar`, and `kubectl-kyverno` are test
views of that candidate, not independent builds. Charts are packaged once;
publication never repackages them. The native image archive is extracted from
the staged multi-platform images, not rebuilt for tests. Non-native platforms
are cross-built; the Kubernetes runtime tests run their native `linux/amd64`
members, not an emulated test of every architecture.

### Mandatory validation, before public release artifacts

`release-validate.yaml` is shared by tag and pre-tag runs:

- unit tests;
- code generation freshness;
- CLI tests using the retained native CLI, including negative exit behavior;
- explicit Helm and chart-testing lint for **both** charts;
- Artifact Hub lint;
- the full conformance workflow and the full supported Kubernetes matrices,
  including the appropriate alpha/beta/stable Kubernetes API variants;
- policy-library tests against the pinned policy-library commit;
- K6 scenarios and admission hot-path performance regression checks;
- installation of the exact staged install manifest across supported Kubernetes
  versions, using the same locally loaded staged image tags.

Release conformance does not apply the routine workflow's quarantine list.
Failures, cancellations, and skipped mandatory jobs cannot qualify a release.
No reduced/scheduled test run is substituted for release validation.

PR checks include an unprivileged rehearsal through the same validation
workflow. It stages a fixture prerelease, but cannot create tags, GitHub
releases, or registry artifacts. Offline regression tests exercise stable/RC
parsing, missing mappings, stale metadata, all owned image values, checksums,
failure gates, prereleases, conflicting content, and interrupted retry rules.

### Publish retained bytes, then verify remotely

Only successful validation unlocks publication:

1. Create or reuse a **draft** GitHub release.
2. Copy the retained multi-platform OCI image layouts to their exact image tags;
   check remote digests, attach SBOMs, sign, and generate SLSA provenance for all
   seven images.
3. Call reusable Helm publication directly. Publish the retained packages to
   GHCR and `gh-pages`, sign the OCI charts, and serialize index writes with the
   repository-wide `helm-gh-pages` lock.
4. Upload the retained CLI archives, checksums, source archive, CRDs, install
   manifest, plan, and candidate checksums to the draft. Publish the retained
   manifest OCI object. Sign binary/manifest assets with retained Sigstore
   bundles and sign the manifest OCI digest.
5. Read the Helm index and package URLs back (allowing bounded CDN propagation),
   compare package bytes, versions, `appVersion`, rendered image references,
   image/manifest digests, SBOMs, signatures, provenance, and all GitHub assets.
6. For a stable release only, submit a precomputed-checksum Krew manifest and
   verify its index entry or open PR. Existing identical submissions are reused;
   older supported-line patches do not downgrade a newer stable Krew plugin.
7. Only then undraft the GitHub release. Prereleases are explicitly not latest
   and never update Krew. An older stable patch does not replace a newer stable
   release as latest.

The Krew manifest is generated from staged archive hashes, so this step does not
need public downloads of draft assets. The external Krew PR may run checks
before the final GitHub undraft; acceptance by Krew maintainers is not automated.

## Completion and diagnostics

A public tag, an image, a chart index entry, or even a successful individual job
is **not** completion. The release is complete only when every mandatory
publication/remote-verification job succeeds and the GitHub release is non-draft.

The validation and publication summaries include the exact SHA, application and
chart versions, image digests, policy-library commit, every job/Kubernetes/shard
result, first failed step, log links, and reproduction/retry commands. Retained
reports include per-test JSON for the shared conformance runner, custom Sigstore,
CLI conformance and policy library, and failed-job logs for special suites and publication jobs. K6
reports remain available as their existing report artifacts.

## Recovery and conflict handling

Inspect the detailed report and fix the external/transient cause first. Rerun
**failed jobs of the original run**:

```bash
gh run view <run-id> -R kyverno/kyverno
gh run download <run-id> -R kyverno/kyverno --name release-candidate
gh run rerun <run-id> -R kyverno/kyverno --failed
```

Candidate restoration verifies the tag/SHA and complete checksum seal. Retrying
build preparation reuses the original candidate, not a fresh build. Publishers
skip identical existing versions, fill missing pieces, and reject mismatched
bytes/digests. Index updates are serialized; a successful retry does not churn
an unchanged index. Partial publication keeps the GitHub release draft and the
overall run failed, with explicit partial-publication diagnostics.

Publication rechecks the remote tag target, and every publisher checks out the
validated SHA explicitly. A tag that has moved fails closed. An existing public
release cannot re-enter mutating publication: do not rerun a completed release,
and investigate any manual undraft before attempting recovery. Read-only remote
verification remains available.

Do not use a new workflow run to rebuild an already partially published release.
Do not use `--clobber`, move/delete tags, force-push `gh-pages`, or regenerate
packages to repair a conflicting version. If the retained candidate expired,
automatic recovery fails closed. Maintainers must recover and verify the
original candidate from retained storage or choose a new version; a rebuilt
candidate is not a safe substitute.

For a source defect, fix and review the source, then release a **new tag**. A
failed public tag remains immutable history.

## Backports and external references

After the main implementation merges, prepare reviewed backport PRs to supported
release branches, including the mapping and the branch's supported Kubernetes
matrix. Do not use this procedure on those branches until those backports merge.
Do not merge a backport merely to mark the checklist complete.

The maintainer will update wiki and website release instructions to link here.
Those external reference changes and external Krew PR merges are not performed
by the release workflow.
