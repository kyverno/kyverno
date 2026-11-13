# Contributor Guidelines for Kyverno

[Kyverno and its sub-projects](https://github.com/kyverno#projects) follow the contributor guidelines published at: https://github.com/kyverno/community/blob/main/CODE_OF_CONDUCT.md.

Please review the general guidelines before proceeding further to the project specific information below.

### Fix or Improve Kyverno Documentation

The [Kyverno website](https://kyverno.io), like the main Kyverno codebase, is stored in its own [git repo](https://github.com/kyverno/website). To get started with contributions to the documentation, [follow the guide](https://github.com/kyverno/website#contributing) on that repository.

### Developer Guides

To learn about the code base and developer processes, refer to the [development guide](/DEVELOPMENT.md).

### Good First Issues

Maintainers identify issues that are ideal for new contributors with a `good first issue` label.

View all Kyverno [good first issues](https://github.com/kyverno/kyverno/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

### Pull Request Guidelines

In the process of submitting your PRs, please read and abide by the template provided to ensure the maintainers are able to understand your changes and quickly come up to speed. There are some important pieces that are required outside the code itself. Some of these are up to you, others are up to the maintainers.

1. Provide Proof Manifests allowing the maintainers and other contributors to verify your changes without requiring they understand the nuances of all your code.
2. For new or changed functionality, this typically requires documentation, so raise a corresponding issue (or, better yet, raise a separate PR) on the [documentation repository](https://github.com/kyverno/website).
3. Test your change with the [Kyverno CLI](https://kyverno.io/docs/kyverno-cli/) and provide a test manifest in the proper format. If your feature/fix does not work with the CLI, a separate issue requesting CLI support must be made. For changes that can be tested as an end user, we require conformance/e2e tests by using the `chainsaw` tool. See the [Chainsaw quick-start guide](https://kyverno.github.io/chainsaw/latest/quick-start/) for how and when to write these tests. If your change touches the CEL admission path (`pkg/cel/policies/*` or `pkg/webhooks/resource/{vpol,mpol}`), see [Performance benchmarks](docs/dev/README.md#performance-benchmarks) for how the `make check-perf` gate works (it runs post-merge, not per pull request) and what to do if it fails.
4. Indicate which release this PR is triaged for (maintainers). This step is important, especially for the documentation maintainers, in order to understand when and where the necessary changes should be made.

### The E2E Gate status

The conformance/e2e suite is too slow to run on every pull request, so it runs after merge, on each push to `main` and the release branches (the `Tests` workflow). When it fails there, a tracking issue labelled `e2e-failure` is opened, and every open pull request to that branch shows a red **E2E Gate** status until the branch passes again.

If your pull request shows a red E2E Gate:

- It is not caused by your pull request, and you don't need to do anything. The status reports the state of the branch you are targeting, not your changes.
- "Details" links to the tracking issue, which lists the failing jobs and explains how the gate clears.
- It turns green by itself, without a push, when the `Tests` workflow passes on the branch or the tracking issue is closed.

The pull request that fixes the branch needs to get through while the gate is red. A maintainer adds the `e2e-gate-bypass` label to it, which turns its E2E Gate green.

## Release Processes

Review the Kyverno release process at: https://kyverno.io/docs/releases/
