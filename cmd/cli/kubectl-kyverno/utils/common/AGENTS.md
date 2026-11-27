# Common CLI utilities

## Cluster resource loading

`ResourceFetcher.getFromCluster` discovers the policy's matched ordinary resources
and subresource endpoints before loading them. With concurrency greater than one,
ordinary resources still use `loader.LoadResourcesConcurrent`, including when the
same policy also matches subresources. That loader accepts GVKs only: fetch
subresources through `listResources` and merge the results without replacing
ordinary resources. Subresource-only matches must not invoke the concurrent
loader with an empty list of resource types. Sequential requests share a five-minute
deadline; pass that context to every list and get request and propagate cancellation.
Individual API errors still skip the failed endpoint and retain successful results.

## Identity rules

- Ordinary results are keyed by full group/version/kind, namespace, and name.
  Kind alone loses same-named objects from different API groups or versions.
- Discovery may omit a subresource's group or version. Fill each missing field
  from its parent before storing the discovery entry or assigning the result's GVK;
  preserve explicit child fields.
- `subresourceKey` includes the child's GVK, the parent's group/version/resource,
  and the subresource name. Child GVK alone collapses `status` and `resize`, or
  different parents' `scale` endpoints returning the same Scale GVK.
- Loaded subresource keys also include parent identity and subresource name, plus
  namespace and object name. A parent and its subresources can share a GVK and
  object name and must all survive. Keep the object name as the last `::` component
  because resource-path filtering reads it there.

## Regression checks

Run `go test ./cmd/cli/kubectl-kyverno/utils/common -count=1` for discovery,
identity, and mixed ordinary/subresource loading coverage. Run
`make test-cli-local-cluster` for the actual CLI against a local mock Kubernetes
API; it checks resource counts, policy failures, and process exit status at
concurrency 1, 2, and 4. This target is included in `make test-cli-local` and
requires no deployed cluster.
