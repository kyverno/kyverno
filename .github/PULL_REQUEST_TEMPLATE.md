## Related issue

<!--
Please link the GitHub issue this pull request resolves in the format of `#1234`. If you discussed this change
with a maintainer, please mention her/him using the `@` syntax (e.g. `@JimBugwadia`).

If this change neither resolves an existing issue nor has sign-off from one of the maintainers, there is a
chance substantial changes will be requested or that the changes will be rejected.

You can discuss changes with maintainers in the [Kyverno Slack Channel](https://kubernetes.slack.com/).
-->

## Milestone of this PR
<!--

Add the milestone label by commenting `/milestone 1.2.3`.

-->
## What type of PR is this

<!--

> Uncomment only one ` /kind <>` line, hit enter to put that in a new line, and remove leading white spaces from that line:
>
> /kind api-change
> /kind bug
> /kind cleanup
> /kind design
> /kind documentation
> /kind failing-test
> /kind feature
-->

## Proposed Changes

<!--
Describe the big picture of your changes here to communicate to the maintainers why we should accept this pull request. 

***NOTE***: If this PR results in new or altered behavior which is user facing, you **MUST** read and follow the steps outlined in the [PR documentation guide](pr_documentation.md) and add Proof Manifests as defined below.
-->

### Proof Manifests

<!--
Read and follow the [PR documentation guide](https://github.com/kyverno/kyverno/blob/main/.github/pr_documentation.md) for more details first. This section is for pasting your YAML manifests (Kubernetes resources and Kyverno policies) and Kyverno CLI test manifests which allow maintainers to prove the intended functionality is achieved by your PR. Please use proper fenced code block formatting, for example:

# Kubernetes resource

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: roles-dictionary
  namespace: default
data:
  allowed-roles: "[\"cluster-admin\", \"cluster-operator\", \"tenant-admin\"]"
```

# Kyverno CLI test manifest (please see docs for latest manifest format at https://kyverno.io/docs/kyverno-cli/). See kyverno/policies for complete examples of all related test files.

```yaml
name: prepend-image-registry
policies:
  - prepend_image_registry.yaml
resources:
  - resource.yaml
variables: values.yaml
results:
  - policy: prepend-registry
    rule: prepend-registry-containers
    resource: mypod
    # if mutate rule
    patchedResource: patchedResource01.yaml
    kind: Pod
    result: pass
```
-->

## Checklist

<!--
Put an `x` in the boxes that apply. You can also fill these out after creating the PR. If you're unsure about any of
them, don't hesitate to ask. We're here to help! This is simply a reminder of what we are going to look for before merging your code.
-->

- [] I have read the [contributing guidelines](https://github.com/kyverno/kyverno/blob/main/CONTRIBUTING.md).
- [] I have added tests that prove my fix is effective or that my feature works.
- [] My PR contains new or altered behavior to Kyverno and
  - [] CLI support should be added my PR doesn't contain that functionality.
  - [] I have added or changed [the documentation](https://github.com/kyverno/website) myself in an existing PR and the link is:
  <!-- Uncomment to link to the PR -->
  <!-- https://github.com/kyverno/website/pull/123 -->
  - [] I have raised an issue in [kyverno/website](https://github.com/kyverno/website) to track the doc update and the link is:
  <!-- Uncomment to link to the issue -->
  <!-- https://github.com/kyverno/website/issues/1 -->
  - [] I have read the [PR documentation guide](https://github.com/kyverno/kyverno/blob/main/.github/pr_documentation.md) and followed the process including adding proof manifests to this PR.

## Further Comments

<!--
If this is a relatively large or complex change, kick off the discussion by explaining why you chose the solution
you did and what alternatives you considered, etc...
-->
