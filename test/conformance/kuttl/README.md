# Testing with `kuttl`

This document explains conformance and end-to-end (e2e) tests using the `kuttl` tool, when test coverage is required or beneficial, and how contributors may write these tests.

## Overview

Kyverno uses [`kuttl`](https://github.com/kudobuilder/kuttl) for performing tests on a live Kubernetes environment with the current code of Kyverno running inside it. The official documentation for this tool is located [here](https://kuttl.dev/). `kuttl` is a Kubernetes testing tool that is capable of submitting resources to a cluster and checking the state of those resources. By comparing that state with declarations defined in other files, `kuttl` can determine whether the observed state is "correct" and either pass or fail based upon this. It also has abilities to run commands or whole scripts. `kuttl` tests work by defining a number of different YAML files with a numerical prefix and co-locating these files in a single directory. Each directory represents a "test case". Files within this directory are evaluated/executed in numerical order. If a failure is encountered at any step in the process, the test is halted and a failure reported. The benefit of `kuttl` is that test cases may be easily and quickly written with no knowledge of a programming language required.

## How Tests Are Conducted

Kyverno uses `kuttl` tests to check behavior against incoming code in the form of PRs. Upon every PR, the following automated actions occur in GitHub Actions:

1. A KinD cluster is built.
2. Kyverno is built from source incorporating the changes in your PR.
3. Kyverno is installed into the KinD cluster.
4. Kuttl executes all test cases against the live environment.

## When Tests Are Required

Tests are required for any PR which:

1. Introduces a new capability
2. Enhances an existing capability
3. Fixes an issue
4. Makes a behavioral change

Test cases are required for any of the above which can be tested and verified from an end-user (black box) perspective. Tests are also required _at the same time_ as when a PR is proposed. Unless there are special circumstances, tests may not follow a PR which introduces any of the following items in the list. This is because it is too easy to forget to write a test and then it never happens. Tests should always be considered a part of a responsible development process and not an after thought or "extra".

## Organizing Tests

Organization of tests is critical to ensure we have an accounting of what exists. With the eventuality of hundreds of test cases, they must be organized to be useful. Please look at the [existing directory structure](https://github.com/kyverno/kyverno/tree/main/test/conformance/kuttl) to identify a suitable location for your tests. Tests are typically organized with the following structure, though this is subject to change.

```
.
├── generate
│   └── clusterpolicy
│       ├── cornercases
│       │   ├── test_case_01
│       │   │   ├── <files>.yaml
│       │   └── test_case_02
│       │       ├── <files>.yaml
│       └── standard
│           ├── clone
│           │   ├── nosync
│           │   │   ├── test_case_03
```

PRs which address issues will typically go into the `cornercases` directory separated by `clusterpolicy` or `policy` depending on which it addresses. If both, it can go under `cornercases`. PRs which add net new functionality such as a new rule type or significant capability should have basic tests under the `standard` directory. Standard tests test for generic behavior and NOT an esoteric combination of inputs/events to expose a problem. For example, an example of a standard test is to ensure that a ClusterPolicy with a single validate rule can successfully be created. Unless the contents are highly specific, this is a standard test which should be organized under the `standard` directory.

## Writing Tests

To make writing test cases even easier, we have provided an example [here](https://github.com/kyverno/kyverno/tree/main/test/conformance/kuttl/_aaa_template_resources) under the `scaffold` directory which may be copied-and-pasted to a new test case (directory) based upon the organizational structure outlined above. Additional `kuttl` test files may be found in either `commands` or `scripts` with some common test files for Kyverno.

It is imperative you modify `README.md` for each test case and follow the template provided. The template looks like the following:

```markdown
## Description

This is a description of what my test does and why it needs to do it.

## Expected Behavior

This is the expected behavior of my test. Although it's assumed the test, overall, should pass/succeed, be specific about what the internal behavior is which leads to that result.

## Reference Issue(s)

1234
```

For some best practices we have identified, see the best practices document [here](https://github.com/kyverno/kyverno/blob/main/test/conformance/kuttl/_aaa_template_resources/BEST_PRACTICES.md).
