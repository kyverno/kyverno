# Creating a Kyverno release

This is the **source of truth for the Kyverno release procedure**. Update this
guide alongside changes to the release workflows. Wiki and website release
instructions should link here rather than maintain a separate procedure.

**Status:** application and Helm releases currently have separate triggers.
The [single-tag automation](#planned-single-tag-release) below is **not yet
implemented**. Follow the current procedure until the workflows on the target
release branch implement it. Implementation is tracked in
[kyverno/kyverno#18063](https://github.com/kyverno/kyverno/issues/18063).

## Current release procedure

### Prepare the release commit

1. Prepare a branch from the intended upstream release branch.
2. Update the chart versions using the Makefile target. Application and chart
   versions have different numbering schemes. For example:

   ```bash
   kube_version=$(sed -n 's/^kubeVersion: //p' charts/kyverno/Chart.yaml)
   test -n "$kube_version" || { echo "Missing chart kubeVersion" >&2; exit 1; }
   APP_CHART_VERSION=v1.19.2-rc.3 \
     KYVERNO_CHART_VERSION=3.9.2-rc.3 \
     POLICIES_CHART_VERSION=3.9.2-rc.3 \
     KUBE_CHART_VERSION="$kube_version" \
     make codegen-helm-update-versions
   ```

   This preserves the existing YAML quotes around the Kubernetes compatibility
   constraint. The target applies one constraint to all four charts; first
   verify that both parent charts and the bundled charts share that constraint.
   If they differ intentionally, preserve their individual constraints instead
   of applying this command unchanged. Check their dependency version references.
   For RCs, set `artifacthub.io/prerelease` to `"true"` in both parent charts;
   for stable releases, set it to `"false"`. The version-update target does not
   update this annotation.
3. Run `make codegen-helm-all` to generate chart documentation and CRDs.
4. Review and commit the changes, then merge a PR into the upstream release branch.
5. Verify the intended commit and required test results before tagging. The
   current release workflow does not enforce a full E2E gate.

### Publish the application

Push an application tag such as `v1.19.2-rc.3` for the merged release commit.
Push with a maintainer credential authorized by repository rules, not a
workflow's `GITHUB_TOKEN`: tags pushed using that token do not trigger push
workflows. The same restriction applies to the chart tags below.
[`release.yaml`](../../../.github/workflows/release.yaml) runs on `v*` tag pushes
and publishes images, CLI release assets, signatures/provenance, and install
manifests through separate jobs. Stable tags also trigger the Krew update job;
prereleases do not.

**An application tag does not trigger Helm publishing.** In particular, a tag
does not rewrite chart metadata. If a tag still contains the previous
`Chart.yaml` versions, dispatching Helm publishing cannot correct those versions.

### Publish both Helm charts

[`helm-release.yaml`](../../../.github/workflows/helm-release.yaml) requires
separate chart tags or a manual workflow dispatch. Create the following tags
on the prepared release commit, then push them to upstream:

```bash
git tag -a kyverno-chart-3.9.2-rc.3 <release-commit> -m "kyverno chart 3.9.2-rc.3"
git tag -a kyverno-policies-chart-3.9.2-rc.3 <release-commit> -m "kyverno policies chart 3.9.2-rc.3"
git push upstream kyverno-chart-3.9.2-rc.3 kyverno-policies-chart-3.9.2-rc.3
```

These are examples; use the versions actually prepared in the release commit.
Each chart's `version`, `appVersion`, and rendered Kyverno image references must
match the intended release. Preserve the exact application tag spelling:
`v1.19.2-rc3` and `v1.19.2-rc.3` are distinct image tags.

Both tags may be pushed for the same release commit. The `helm-release` workflow serializes `gh-pages` updates via a concurrency group, so parallel tag pushes are queued instead of racing.

### Verify completion

Check all application and chart workflow jobs, including provenance and signing,
not just the existence of a GitHub release. Verify CLI assets, install manifests,
container images, and both Helm packages in the Helm repository and GHCR.

```bash
helm repo add kyverno https://kyverno.github.io/kyverno
helm repo update kyverno
helm search repo kyverno/kyverno --versions --devel
helm show chart kyverno/kyverno --version 3.9.2-rc.3
helm show chart kyverno/kyverno-policies --version 3.9.2-rc.3
```

## Recovering a failed chart publish

If a chart tag exists but the package is missing from the [Helm repo index](https://kyverno.github.io/kyverno/index.yaml) (for example `kyverno-policies-X.Y.Z` returns 404), the `helm-release` workflow likely failed while pushing to `gh-pages`. This can happen when two chart releases race on `gh-pages` before the concurrency guard was added.

To recover:

1. **Re-run the failed workflow** from the GitHub Actions UI (`helm-release` workflow, failed `create-release` job), or:
1. **Use workflow dispatch** (Actions → helm-release → Run workflow):
   - `chart`: `kyverno` or `kyverno-policies`
   - `version`: chart version (e.g. `3.8.1`)
   - `ref`: the release tag (e.g. `kyverno-policies-chart-3.8.1`)
1. Before dispatching, verify the selected ref already contains the correct
   chart metadata. The dispatch `version` input selects a release identifier;
   it does **not** update `Chart.yaml` or image references.

Verify recovery:

```bash
curl -sI https://kyverno.github.io/kyverno/kyverno-policies-<version>.tgz
helm repo add kyverno https://kyverno.github.io/kyverno
helm repo update
helm search repo kyverno/kyverno-policies --version <version> --devel
```

Do not delete or move a public application tag, or replace an already published
chart version with different content. If the source or release metadata is
wrong, prepare a new version. Prefer workflow recovery over manually editing
the Helm index.

## Planned single-tag release

This section defines the target operational behavior for
[kyverno/kyverno#18063](https://github.com/kyverno/kyverno/issues/18063).
It is not a procedure supported by the current workflows.

The only per-release maintainer action will be pushing an application tag.
There will be no preparatory version-bump PR, separate chart tags, manual chart
publication, or manual Krew update. Normal code review and release-line setup
remain prerequisites.

```text
Application tag
  -> resolve exact source SHA and versions
  -> prepare immutable release artifacts
  -> validate and run full E2E matrix
  -> publish artifacts and signatures/provenance
  -> verify remote artifacts and Helm index
  -> finalize GitHub release and stable-only Krew update
```

### Application-to-chart version mapping

The implementation will add `config/releases.yaml`, read from the **tagged
commit**, as the machine-readable mapping. For example:

```yaml
releaseLines:
  "1.19":
    kyvernoChart: "3.9"
    policiesChart: "3.9"
  "1.20":
    kyvernoChart: "3.10"
    policiesChart: "3.10"
```

Maintainers will add entries when opening release lines, not for each patch or
RC. Missing mappings must fail before publishing. The application patch and
prerelease suffix determine the corresponding chart patch and prerelease:
`v1.19.2-rc.3` produces both charts at `3.9.2-rc.3`. Existing `-rc3` tag spellings
must retain their exact image names; new releases should use dotted `-rc.3`.
Never silently normalize application image tags.

This policy allocates chart patch/prerelease versions in lockstep with application
releases. Independent chart-only releases must not consume versions in that
sequence. A conflicting existing package must fail the workflow rather than be
overwritten. The separate mapping fields allow the two chart minor lines to differ.

### Preparation and validation

Derive versions in an isolated workspace without committing back to the branch.
Update both charts, bundled CRD/Grafana charts, dependency references, and
prerelease annotations. Generate chart docs, CRDs, and release manifests;
preserve Kubernetes compatibility constraints and use locked external
dependencies. Distinguish expected version-stamping changes from unexpected
codegen drift.

Build once and retain the prepared artifacts with checksums, image digests,
source SHA, and tool versions in a release manifest. Validate and publish the
same artifacts, not fresh builds. Testing must use the prepared charts and
release images, including every Kyverno controller and init-container image.

Before public artifact publication, require unit and CLI tests, codegen
freshness, explicit lint of both charts, Artifact Hub lint, the full supported
Kubernetes E2E matrix, policy-library tests, and performance checks. A green
reduced scheduled matrix is insufficient. Quarantined tests must appear in the
report, with owner, issue, expiry, and an explicit non-blocking status if excluded
from the release gate.

A failed release must provide per-test/per-Kubernetes results, first errors,
logs and diagnostic artifacts, reproduction commands, and the exact source and
artifact identities. Related batching and triage work is tracked in
[kyverno/kyverno#18046](https://github.com/kyverno/kyverno/issues/18046).

### Publishing, completion, and recovery

Call reusable publishing jobs directly from the release workflow rather than
relying on secondary tag events. Publish both charts to the Helm repository
and GHCR, plus signed images, CLI archives/checksums, provenance, and install
manifests. Serialize Helm index writes and concurrent runs for the same tag.

Keep the GitHub release draft until every required job succeeds. Before
finalizing, download and verify the published artifacts: chart metadata and
rendered image references, index entries, digests, checksums, signatures,
provenance, CLI assets, and install manifests. Provide one completion summary
linking the results. Prereleases must not become latest stable or update Krew.

Publishing across registries is not atomic. A partially published release must
remain visibly incomplete, with the failed stage and recovery action reported.
Reruns must reuse retained artifacts, verify any existing content, and resume
missing steps without overwriting a version with different bytes.

### Tag timing and rollout

A pushed tag exists publicly before its workflow starts. This path guarantees
validation **before artifact publication**, not before tag creation. For
validation before tagging, an optional single `prepare-release` dispatch can
validate an exact SHA and automatically create the tag after success. It must
invoke the same pipeline; a tag created with `GITHUB_TOKEN` does not itself
trigger another push workflow. A failed public tag must never be moved or reused.

Roll out version preparation first, then unified publishing, full validation,
and completion/recovery checks. Cover stable and RC tags, missing mappings,
stale chart metadata, failure gating, and interrupted retries with automation
tests. Backport the workflows and mapping to supported release branches before
using the new procedure there. Update this guide as each stage ships; remove
the current manual steps only after their replacements are available. Wiki and
website updates should reference this guide after it is updated.