# Namespaced policy admission scope

Create policies through a patched Kyverno admission webhook to verify that:

- An unknown kind alongside Pod cannot hide foreign context or verifyImages credentials.
- Bare and explicit local credential references still work with an unknown-kind warning.
- Cluster-scoped CEL parameter resources are refused through both their preferred v1 and served v1beta1 versions.
- A namespaced parameter resource with the same Kind in another API group remains allowed.
- ClusterPolicy parameter behavior remains unchanged.

The test creates two temporary CRDs and policy objects; it does not read secrets or contact an image registry. Chainsaw cleans up the created resources. No parameter objects are needed because only policy admission is tested.

Run against a patched deployment:

```sh
chainsaw test --test-dir test/conformance/chainsaw/validate/namespaced-admission-scope
```
