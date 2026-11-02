# Generation-label webhook bootstrap exclusion

This exclusive test requires a disposable test cluster with Kyverno installed
and automatic webhook updates initially enabled, cluster-admin credentials, `kubectl`, and `jq`. It temporarily restarts admission
with `excludeBootstrapResources=true` in automatic and manual webhook modes,
then restores the original admission arguments. Set `KYVERNO_NAMESPACE` if the
installation uses a namespace other than `kyverno`.

The test copies the emitted generation-label webhook's rules and object selector
into a temporary webhook with a nonexistent service. An additional selector
limits it to the probe manifests. Node and CertificateSigningRequest requests
use server-side dry-run and retain a generation policy association. Without the
copied match condition, both requests must fail because the service is absent.
With the emitted condition, both requests must succeed without a callback.

Manual mode also refreshes an older reserved entry while preserving a separate
manual webhook entry. Cleanup removes only the temporary webhook and manual
entry created by this test. No Node or CSR is persisted; `csr.yaml` contains only
a public test certificate request, with no private key or issued certificate.

CI runs this suite in its own job, with a fresh KinD cluster for each supported
Kubernetes version. It is separate from the ordinary generating-policy suite
because it changes the admission Deployment and shared webhook configuration.

Run from the repository root against the disposable cluster:

```sh
chainsaw test --config test/conformance/chainsaw/webhook-bootstrap-exclusion/.chainsaw.yaml \
  --test-dir test/conformance/chainsaw/webhook-bootstrap-exclusion/generation-labels
```
