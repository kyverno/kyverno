The Pod fixtures in `foreach`, `nested-foreach`, and `simple` test admission-time
array mutation only. Their unmatched `kyverno.io/test-node: nonexistent` node
selector deliberately prevents scheduling, so cleanup does not depend on image
pulls or container termination. The Pod assertions check that the selector is
preserved alongside the original mutation expectations.

All of the following tests in this directory have been copied (for now) to kyverno/policies where they rightfully belong.
Once we are executing tests from that repo in this one, we can remove all the cases.

- add-emptydirsizelimit
- add-external-secret-prefix
- add-image-as-env-var
- add-node-affinity
- add-pod-priorityclassname
- add-tolerations
- add-volume-deployment
- annotate-base-images
- inject-env-var-from-image-label
- k10-minimum-retention
- replace-ingress-hosts