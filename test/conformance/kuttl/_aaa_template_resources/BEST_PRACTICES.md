# Some Best Practices

* Don't put anything in index `00` so it can be used in the future.
* A final clean-up stage/file is not needed unless a resource was created using a Script. Use scripts sparingly!
* The `*-errors.yaml` file, like an `*-assert.yaml` file only performs an existence check, not a creation check.
* One test can contain both positive and negative tests by extending the test case. No need to write separate.