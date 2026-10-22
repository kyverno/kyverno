# pre-existing-no-effect

Verifies that a legacy `ClusterPolicy` already stored before admission-time legacy execution
existed (simulated here by deleting the policy webhook configuration, creating the `ClusterPolicy`
while no webhook denies it, then letting the running controller recreate the webhook
configuration) has no effect on admission: a `Pod` that violates the stored policy's rule is
still allowed, because 1.20 never executes legacy `kyverno.io` policy content at admission time,
regardless of how the object got into the cluster.

Note: this test is written against the described behavior but has not been run against a live
cluster as part of this change.
