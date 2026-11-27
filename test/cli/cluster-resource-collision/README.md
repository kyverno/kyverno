# Cluster resource collision regression

Run `make test-cli-local-cluster` from the repository root. This builds the CLI
and runs `kyverno apply --cluster` against an in-process mock Kubernetes API;
no existing cluster or user kubeconfig is used.

The cross-group fixture serves two Widgets with identical names and namespaces.
The alpha resource passes and the beta resource fails. At concurrency settings
1, 2, and 4, both must be evaluated: one pass, one failure, and exit status 1.

The subresource fixture selects the alpha Widget together with its status and
resize endpoints. Both return the parent Kind and omit group/version in discovery.
The mock status response violates the policy while the parent and resize pass.
All three inputs must survive at each concurrency setting, with two passes,
one failure, and exit status 1.

The Go test is part of `make test-cli` through `test-cli-local-cluster`. When
running it directly, set `KYVERNO_CLI_BIN` to the absolute path of a built CLI.
