# KEP-6060 receiver proof

This manual conformance case requires a disposable Kubernetes v1.37 cluster made
with Kind v0.33.0 or later and [kind.yaml](kind.yaml), this checkout's Kyverno images, and
receiver authentication initially disabled. Always set `KUBECONFIG` to that
cluster's separate kubeconfig for every `kubectl` and Helm command. The probe
script expects the cluster name `kyverno-17399-20261002` to guard other contexts.

For example, create the cluster with:

```sh
kind create cluster --name kyverno-17399-20261002 \
  --image kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5 \
  --config test/conformance/manual/webhook-auth/kind.yaml \
  --kubeconfig /tmp/kyverno-17399-20261002-kubeconfig
```

Use the repo's `kind-load-all` and `kind-install-kyverno` targets with
`KUBECONFIG=/tmp/kyverno-17399-20261002-kubeconfig` and
`KIND_NAME=kyverno-17399-20261002`; pass the installed Kind v0.33 binary as
`KIND` if the repo's bundled Kind is older. Keep the feature off during the
baseline installation and set `features.blockLegacyPolicyAPIs.enabled=false`.

Apply [resources.yaml](resources.yaml) before enabling receiver authentication.
Wait until Kyverno's webhook configurations contain their `/validate`
and `/mutate` routes. Confirm unauthenticated validation and mutation behavior
using direct AdmissionReviews. Then enable
`features.webhookAuthentication.enabled=true` for both admission and cleanup
controllers.

Run the probe before enablement, after the Helm upgrade, and after disabling
the flag, with phases `baseline`, `enabled`, and `rollback` respectively:

```sh
python3 test/conformance/manual/webhook-auth/probe.py \
  --kubeconfig /tmp/kyverno-17399-20261002-kubeconfig --phase baseline
```

The probe keeps issued token values in process memory.

Use the dedicated `webhook-auth-proof` ServiceAccount to request a ten-minute
TokenRequest bound to the current ValidatingWebhookConfiguration or
MutatingWebhookConfiguration. Set exactly one `spec.audiences` value to the
service or URL audience for the route being tested. Set
`spec.attestations.admissionReviewAPIGroups` to `["apps"]` or `["*"]`.
The service account has only the required `create` permission for its own
TokenRequest and `attest` permission for those values. Obtain a fresh caller
credential before it expires, and keep both credentials in memory or a private
temporary directory outside the repository.

Send a matching AdmissionReview over verified TLS to each server with
`Authorization: Bearer <token>`. For validation, test a Deployment review with
and without the required `webhook-auth-proof: "true"` label. For mutation,
confirm the response patch adds `webhook-auth-mutated: "true"`. Check that the
response UID matches the request. Repeat without a token and with wrong
audience, API group, and signature. Request another valid token to prove
reacquisition. Check both public probes while authentication is enabled.

Finally disable the feature and confirm that the same AdmissionReview is
processed without a token. Run the chart's `make helm-test` only with the
disposable cluster's `KUBECONFIG`. This proves the receiver; normal stock
kube-apiserver admission calls do not yet attach KEP-6060 credentials.
