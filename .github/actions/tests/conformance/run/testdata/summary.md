## Conformance test summary

**3 tests failed**: 1 on all versions, 1 on all versions that ran it, 1 on some versions (possibly flaky).

| Test | Failing Kubernetes versions | Scope | First error | Logs |
| --- | --- | --- | --- | --- |
| <code>ttl/delete-twice</code> | v1.34.11, v1.35.8, v1.36.4, v1.37.0 | all versions | <code>step-02 / error: v1/Pod/chainsaw-bright-loon/test-pod - resource matches expectation</code> | [v1.34.11](https://github.com/kyverno/kyverno/actions/runs/42/job/101) [v1.35.8](https://github.com/kyverno/kyverno/actions/runs/42/job/102) [v1.36.4](https://github.com/kyverno/kyverno/actions/runs/42/job/103) [v1.37.0](https://github.com/kyverno/kyverno/actions/runs/42/job/104) |
| <code>policy-exceptions-disabled/image-validating-policies</code> | v1.34.11, v1.35.8, v1.37.0 | all versions that ran it<br>no result on v1.36.4 | <code>create good pod / create: Post "https://127.0.0.1:41435/api/v1/namespaces/default/pods?fieldValidation=Strict": context deadline exceeded</code> | [v1.34.11](https://github.com/kyverno/kyverno/actions/runs/42/job/201) [v1.35.8](https://github.com/kyverno/kyverno/actions/runs/42/job/202) [v1.37.0](https://github.com/kyverno/kyverno/actions/runs/42/job/204) |
| <code>ttl/past-timestamp</code> | v1.35.8 | some versions (possibly flaky)<br>passed on v1.34.11, v1.36.4, v1.37.0 | <code>step-02 / error: v1/Pod/chainsaw-valid-rattler/test-pod - resource matches expectation</code> | [v1.35.8](https://github.com/kyverno/kyverno/actions/runs/42/job/102) |

The first error is the one from the lowest failing version.

### Failed jobs without a failed test

These jobs failed outside a chainsaw test, for example in setup or on a timeout, or they aren't conformance jobs.

| Job | Result | Failed step | Test report |
| --- | --- | --- | --- |
| [Conformance tests / policy-exceptions-disabled (v1.36.4)](https://github.com/kyverno/kyverno/actions/runs/42/job/203) | failure | Run ./.github/actions/tests/conformance/run | none |
| [K6 tests / load (v1.37.0)](https://github.com/kyverno/kyverno/actions/runs/42/job/301) | failure | Run k6 | none |

### Totals

- Tests: 3 failed, 1 passed, 1 skipped. A skipped test is quarantined, or chainsaw stopped at an earlier failure in the same job.
- Reports: 8, on Kubernetes v1.34.11, v1.35.8, v1.36.4, v1.37.0. 1 job uploaded no test results.
