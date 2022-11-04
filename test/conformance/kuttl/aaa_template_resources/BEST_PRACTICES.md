# Some Best Practices

* Don't put anything in index `00` so it can be used in the future.
* Put clean-up as index `99` so it's always last no matter how many steps.
* The `*-errors.yaml` file, like an `*-assert.yaml` file only performs an existence check, not a creation check.
* One test can contain both positive and negative tests by extending the test case. No need to write separate.