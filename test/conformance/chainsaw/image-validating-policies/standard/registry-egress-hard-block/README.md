# ImageValidatingPolicy registry egress proof

This policy prefetches matching image metadata before evaluating a successful
validation. The metadata and loopback fixtures must fail specifically with
`connection blocked to network host`. The unmatched Pod remains
admissible. No reachable metadata service or private registry is required.

Run against a cluster with this patch:

```sh
chainsaw test --test-dir test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block
```

Run locally from this directory with a CLI built from this patch:

```sh
kubectl-kyverno apply policy.yaml --resource metadata-pod.yaml --policy-report
kubectl-kyverno apply policy.yaml --resource loopback-pod.yaml --policy-report
kubectl-kyverno apply policy.yaml --resource unmatched-pod.yaml --policy-report
```

The first two commands must fail and include the blocked-host error in their policy reports. The last
must succeed or skip the unmatched policy. ImageValidatingPolicy uses the actual
image loader in standalone CLI mode, so these commands exercise the request guard.
