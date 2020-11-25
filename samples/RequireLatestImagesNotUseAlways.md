# Require images using `latest` tag set `imagePullPolicy` to not `Always`

When using the `latest` tag for images, although generally [not a best practice](DisallowLatestTag.md), Kubernetes defaults its `imagePullPolicy` to `Always`. Since Docker Hub has instituted a [rate-limiting policy](https://www.docker.com/blog/what-you-need-to-know-about-upcoming-docker-hub-rate-limiting/), this could result in reaching that limit faster than anticipated, which could mean errors for other Pods in the cluster or across the enterprise. Ensuring those `latest`-tagged images do not use the default of `Always` is one way to ensure pulls are only when needed.

This sample policy checks the `image` value and ensures that if `:latest` is defined that the `imagePullPolicy` must use something other than the value of `Always`. Note that if no tag is defined, Kyverno will not see that as a violation of the policy.

## Policy YAML

[latestimage-notalways.yaml](misc/latestimage-notalways.yaml)

```yaml
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: latestimage-notalways
spec:
  validationFailureAction: audit
  background: false
  rules:
  - name: latestimage-notalways
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "When using the `latest` tag, the `imagePullPolicy` must not use `Always`."  
      pattern:
        spec:
          containers:
          - (image): "*:latest"
            imagePullPolicy: "!Always"
```
