# Flux example: deploying a bundle with `OCIRepository`

This is a worked, digest-pinned example of consuming a Kyverno CEL policy bundle with
[Flux](https://fluxcd.io)'s `OCIRepository` and `Kustomization` controllers. It's developer documentation: a
reference for the `kyverno/website` GitOps guide to build on, not a replacement for it. See
[bundle-spec.md section 10](./bundle-spec.md#10-consumption-by-generic-tooling) for the normative claims about
what Flux's `OCIRepository` controller requires of an artifact.

## Prerequisites

- Kyverno 1.20 or later, with the `policies.kyverno.io` CRDs installed in the cluster before the policy
  `Kustomization` below reconciles. Make the policy `Kustomization` depend on the `Kustomization` that installs
  Kyverno, so Flux orders the reconciliation correctly:

  ```yaml
  spec:
    dependsOn:
      - name: kyverno
        namespace: flux-system
  ```

- Flux 2.6 or later. Confirmed on Flux v2.10.0 (source-controller v1.10.1, kustomize-controller v1.10.1):
  `OCIRepository` is `source.toolkit.fluxcd.io/v1` and `Kustomization` is `kustomize.toolkit.fluxcd.io/v1`, each
  with no other served version. The 2.6 floor comes from Flux's own release notes and wasn't independently
  re-verified here; on an older release, check your own cluster first:
  `kubectl get crd ocirepositories.source.toolkit.fluxcd.io -o jsonpath='{range .spec.versions[*]}{.name}
  served={.served} storage={.storage}{"\n"}{end}'`.

- The `team-a` namespace, created before the policy `Kustomization` reconciles: `kubectl create namespace
  team-a`. The bundle format can't carry a `Namespace` object (see
  [3. Supported resources](./bundle-spec.md#3-supported-resources)), and the `PolicyException` below targets
  `team-a` explicitly, so the namespace must already exist on the cluster.

## 1. Push the bundle

Push a directory of policies with a descriptor, so the bundle carries a name, a version, and named sets:

```text
baseline/
├── kyverno-bundle.yaml
├── policies/
│   └── require-labels.yaml          # ValidatingPolicy (cluster-scoped)
└── exceptions/
    └── team-a-exceptions.yaml       # PolicyException, metadata.namespace: team-a
```

```yaml
# kyverno-bundle.yaml
apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  version: 1.0.0
  description: Baseline policies
  sets:
    - name: policies
      type: policies
      paths: ["policies/**"]
    - name: exceptions
      type: exceptions
      paths: ["exceptions/**"]
```

```yaml
# policies/require-labels.yaml — matchConstraints and validationActions copied from
# test/conformance/chainsaw/validating-policies/validation-rules/failure-policy/fail/policy.yaml, whose
# chainsaw test (../fail/chainsaw-test.yaml) asserts a non-conforming Pod is actually denied; a policy with no
# matchConstraints resource rule is rejected at admission time (see
# test/conformance/chainsaw/validating-policies/validation-checks/missing-match-constraints)
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  validationActions:
    - Deny
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: [v1]
        operations:  [CREATE]
        resources:   [pods]
  validations:
    - expression: "has(object.metadata.labels) && 'team' in object.metadata.labels"
      message: "label 'team' is required"
```

`spec.validationActions` is what makes the policy enforce. The CRD marks it required, but the API server admits
a policy that omits it anyway — the result is a policy that reaches `Ready: true` and is webhook-registered
while allowing every request unconditionally. Don't omit it.

```yaml
# exceptions/team-a-exceptions.yaml — explicit metadata.namespace matters; see the Kustomization note below
apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: team-a-exempt
  namespace: team-a
spec:
  policyRefs:
    - name: require-team-label
      kind: ValidatingPolicy
```

These two files are deliberately minimal so the bundle is pushable and the example runs end to end; they aren't
a template for a real policy or exception. `policies.kyverno.io/v1beta1` is the CRD's storage version; a plain
`kubectl get validatingpolicy -o yaml` returns the `v1` representation instead, since `v1` is the
default-preferred served version. Nothing is converted or lost — it's a discovery artifact of which version
`kubectl` picks when none is given.

```bash
kyverno oci push ./baseline -i ghcr.io/acme/baseline:v1.0.0
```

## 2. Resolve the digest

Pin the tag to its digest before writing the `OCIRepository`:

```bash
kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0 --digest-only
# sha256:d34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33f
```

## 3. `OCIRepository`

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: baseline-policies
  namespace: flux-system
spec:
  interval: 10m
  url: oci://ghcr.io/acme/baseline
  ref:
    digest: sha256:d34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33f
  layerSelector:
    mediaType: application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip
    operation: extract
```

`spec.url` carries no tag or digest; the digest lives in `spec.ref.digest`. Pinning `layerSelector.mediaType` to
the bundle's content-layer media type is a SHOULD in the spec: it keeps extraction correct even after a future
major format version that adds a second layer. This `OCIRepository` has no `spec.secretRef`, matching a public
registry such as `ghcr.io` with an anonymous-readable repository or `ttl.sh`; see
[Authentication](#authentication) below for a private registry.

## 4. `Kustomization`

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: baseline-policies
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: OCIRepository
    name: baseline-policies
  path: ./
  prune: true
  dependsOn:
    - name: kyverno
      namespace: flux-system
```

**Don't add a `kustomization.yaml` to the bundle root.** `kyverno oci push` rejects a `kustomization.yaml`
anywhere in the input tree, because its `kind` isn't one of the resources this format supports (see
[bundle-spec.md "What a writer excludes"](./bundle-spec.md#what-a-writer-excludes)). Leave `spec.path` at the
bundle root and let Flux's `Kustomization` controller generate its own kustomization at reconcile time from its
recursive manifest scan; don't commit one into the bundle.

**Give every namespaced document an explicit `metadata.namespace`.** A `PolicyException` or a `Namespaced*Policy`
needs its own `metadata.namespace` set in the source YAML. The `Kustomization` above sets no `spec.targetNamespace`
on purpose: when set, kustomize overrides `metadata.namespace` on every namespaced object it applies, including a
`PolicyException` that already carries an explicit namespace in its source YAML — silently moving it to
`targetNamespace` instead. Cluster-scoped objects such as `ValidatingPolicy` are unaffected once their CRD is
installed and discoverable, which the `dependsOn` above guarantees — but don't rely on `targetNamespace` for
anything namespaced in the bundle.

## Authentication

Flux and the Kyverno CLI authenticate to the registry independently; neither configures the other.

- **Flux** reads `spec.secretRef`, a `kubernetes.io/dockerconfigjson` secret in the same namespace as the
  `OCIRepository`, or `spec.provider: aws`, `azure`, or `gcp` for workload-identity auth instead of a secret.
  `generic` is the default and uses `secretRef` when one is set; it also works anonymously against a public
  registry, which is why the `OCIRepository` above has no `secretRef`. For a private registry, create the
  secret and reference it:

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: baseline-policies-pull
    namespace: flux-system
  type: kubernetes.io/dockerconfigjson
  stringData:
    .dockerconfigjson: '{"auths":{"ghcr.io":{"auth":"<base64 of user:token>"}}}'
  ```

  Replace `<base64 of user:token>` with your own `base64(<username>:<token>)` before applying; the value above
  is a placeholder and won't authenticate against any registry.

  ```yaml
  spec:
    secretRef:
      name: baseline-policies-pull
  ```

  A `secretRef` that points at a Secret that doesn't exist makes the `OCIRepository` fail to reconcile, so don't
  add it unless the Secret is also created.
- **`kyverno oci resolve`, `push`, and `pull`** authenticate through a multi-keychain (Docker config, GitHub, AWS
  ECR, GCP, Azure); there's no `--username`/`--password` flag. Log in with `docker login`, or your cloud CLI, before
  running these commands against a private registry.

## Verify

```bash
kyverno oci inspect -i ghcr.io/acme/baseline@sha256:d34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33fd34db33f
flux get sources oci -n flux-system
flux get kustomizations -n flux-system
```

`kyverno oci inspect` shows the digest's resource index, sets, and Kyverno compatibility from its manifest and
config — it doesn't verify the content layer, so it won't catch an archive that disagrees with the index; run
`kyverno oci validate -i` for that. `flux get` confirms Flux fetched and reconciled the same digest.

This example was verified end to end against a real kind cluster and an anonymous public registry: the
`OCIRepository`/`Kustomization` YAML above reconciled with no field or value changes beyond the digest and
registry URL. Two things in it weren't exercised in that verification and remain as documented, not tested:
`spec.dependsOn` ordering against a real Kyverno-installing `Kustomization`, and `secretRef` against a private
registry.

## Update

A digest-pinned `OCIRepository` doesn't move on its own. To roll out a new bundle, push it, resolve its digest,
and change `spec.ref.digest` to that value — that's the only edit that moves a pinned deployment forward.
