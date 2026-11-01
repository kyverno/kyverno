# Querying a null input file

The `--input` flag selects the input file even when it contains JSON or YAML
`null`. Such a file must not cause the command to read an input object from stdin.
Stdin remains available for the expression when no query argument is supplied.

This test exercises both null formats and a query supplied through stdin. An
unrelated object is piped into the first two cases to catch accidental fallback
to stdin, which would change the result of `type(@)` from `"null"` to `"object"`.

Run against a locally built `kyverno` binary on `PATH` without a Kubernetes cluster:

```sh
chainsaw test --no-cluster --test-dir test/conformance/chainsaw/cli/jp/null-input-file
```
