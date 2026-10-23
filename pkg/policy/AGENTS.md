# AGENTS.md — `pkg/policy`

## REST mapper

`NewPolicyController` builds the REST mapper once during controller construction, using Kubernetes API discovery.
The `newRESTMapper` helper passes discovery results to `restmapper.GetAPIGroupResources` and builds a
`restmapper.NewDiscoveryRESTMapper` from the results.

When discovery reports an error with partial results, `discoveryErrorCapture` preserves the groups and resources
for mapper construction while capturing the error. The helper logs that error and notes that the mapper was built
from partial discovery results. It does not return the discovery error: the controller constructor keeps the
partial mapper and returns `&pc, nil`, so an unavailable aggregated API does not prevent controller startup.

The mapper is not refreshed after startup; it is assigned in `NewPolicyController` and the package has no later
rebuild path. This describes the current behavior.

The logging-only change is deliberate and tracked in [issue #18236](https://github.com/kyverno/kyverno/issues/18236).
It makes partial discovery visible while preserving startup and partial-mapper behavior.
