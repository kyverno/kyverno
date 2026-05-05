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

```
git tag kyverno-chart-1.2.3 -m "tag kyverno-chart-1.2.3" -a
```

### Publish Helm Release Manually

On the `main` branch, run:
- `helm package ./charts/kyverno`, this will generate file `kyverno-<version>.tgz`
- create a copy of [index.yaml](https://raw.githubusercontent.com/kyverno/kyverno/gh-pages/index.yaml), add it to the current main branch
- `helm repo index --url https://kyverno.github.io/kyverno/ ./ --merge index.yaml`, this will merge new index to index.yaml

Finally, add `kyverno-<version>.tgz` and `index.yaml` to branch `gh-page`, then push to upstream.