## Some Best Practices

* Don't put anything in index `00` so it can be used in the future.
* A final clean-up stage/file is not needed unless a resource was created using a Script. Use scripts sparingly!
* The `*-errors.yaml` file, like an `*-assert.yaml` file only performs an existence check, not a creation check.
* One test can contain both positive and negative tests by extending the test case. No need to write separate.

## Kyverno kuttl specifics

Kyverno's fork of kuttl adds several new features not found in the upstream. These features were added to make testing Kyverno's many capabilities easier and more intuitive. Below are some sample TestStep contents which illustrate these features

### Apply, Assert, Errors, Deletes

A TestStep file can declare apply, assert, errors, and deletions by naming the files that should be checked or specifying an object (in the case of delete). These do not all have to be used together.

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
apply:
- policy.yaml
assert:
- policy-ready.yaml
error:
- configmap-rejected.yaml
delete:
- apiVersion: kyverno.io/v1
  kind: ClusterPolicy
  name: podsecurity-subrule-restricted
```

### Checking for creation failures

When the expected behavior for a given manifest's creation should be that it fails (i.e., you want and expect to see it fail), a TestStep can declare this without needing to use a script.

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
apply:
  - file: cleanuppolicy-with-subjects.yaml
    shouldFail: true
```
