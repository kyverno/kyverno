The release is automated with GitHub Action. Once a new tag is pushed, it will trigger the job to publish the release page, the docker images, the Helm Release, and CLI in Krew. 

## Generate a Release through Automated Jobs
Follow these steps to generate a release (1.8.0+):

- create a release branch on your forked repo
- run `APP_CHART_VERSION=v1.2.3 KYVERNO_CHART_VERSION=1.2.3 POLICIES_CHART_VERSION=1.2.3 make codegen-helm-update-versions` to update versions in Helm charts
- run `make codegen-helm-all`, it updates Helm charts docs and CRDs
- check-in the changes and create a PR to merge to the upstream release branch
- tag the merged commit with `v1.2.3`, for example, to trigger the release process

## Publish Helm Chart

### Publish Helm Chart Automatically
1. Release the kyverno chart, tag a particular commit with the tag `kyverno-chart-*`, for example, `kyverno-chart-1.2.3`.
1. Release the kyverno-policies chart, tag a particular commit with the tag `kyverno-policies-chart-*`, for example, `kyverno-policies-chart-1.2.3`.

Both tags may be pushed for the same release commit. The `helm-release` workflow serializes `gh-pages` updates via a concurrency group, so parallel tag pushes are queued instead of racing.

```
git tag kyverno-chart-1.2.3 -m "tag kyverno-chart-1.2.3" -a
git tag kyverno-policies-chart-1.2.3 -m "tag kyverno-policies-chart-1.2.3" -a
```

#### Recovering a failed chart publish

If a chart tag exists but the package is missing from the [Helm repo index](https://kyverno.github.io/kyverno/index.yaml) (for example `kyverno-policies-X.Y.Z` returns 404), the `helm-release` workflow likely failed while pushing to `gh-pages`. This can happen when two chart releases race on `gh-pages` before the concurrency guard was added.

To recover:

1. **Re-run the failed workflow** from the GitHub Actions UI (`helm-release` workflow, failed `create-release` job), or:
1. **Use workflow dispatch** (Actions → helm-release → Run workflow):
   - `chart`: `kyverno` or `kyverno-policies`
   - `version`: chart version (e.g. `3.8.1`)
   - `ref`: the release tag (e.g. `kyverno-policies-chart-3.8.1`)
1. **Manual publish** (last resort): see "Publish Helm Release Manually" below.

Verify recovery:

```bash
curl -sI https://kyverno.github.io/kyverno/kyverno-policies-<version>.tgz
helm repo add kyverno https://kyverno.github.io/kyverno
helm repo update
helm search repo kyverno/kyverno-policies --version <version>
```

### Publish Helm Release Manually

On the `main` branch, run:
- `helm package ./charts/kyverno`, this will generate file `kyverno-<version>.tgz`
- create a copy of [index.yaml](https://raw.githubusercontent.com/kyverno/kyverno/gh-pages/index.yaml), add it to the current main branch
- `helm repo index --url https://kyverno.github.io/kyverno/ ./ --merge index.yaml`, this will merge new index to index.yaml

Finally, add `kyverno-<version>.tgz` and `index.yaml` to branch `gh-page`, then push to upstream.