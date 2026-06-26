# CEL Attestation Library (`pkg/cel/libs/attestation`)

This document covers the standalone attestation CEL library, its design rationale,
how it differs from the existing `imageverify` library, and how to use it in
`ValidatingPolicy` (VPOL) rules.

## Background

Prior to this library, attestation verification in Kyverno was only possible via
`ImageValidatingPolicy` (IVPol). IVPol resolves OCI references from pod image
fields, which makes it unsuitable for verifying attestations on arbitrary OCI
artifacts — for example, in a CI/CD pipeline gate where the artifact reference is
stored in a custom resource.

The `attestation` library decouples the verification functions from
`ImageValidatingPolicyLike` so they can be registered in any CEL environment,
including `ValidatingPolicy`.

## CEL Functions

All four functions from the `imageverify` library are available under the same names:

| Function | Signature | Returns |
|---|---|---|
| `verifyImageSignatures` | `(image: string, attestors: list)` | `int` — number of successful attestor matches |
| `verifyAttestationSignatures` | `(image: string, attestationType: string, attestors: list)` | `int` — number of successful attestor matches |
| `getImageData` | `(image: string)` | `dyn` — OCI image metadata |
| `extractPayload` | `(image: string, attestationType: string)` | `dyn` — decoded attestation payload |

### `attestationType` semantics

The `attestationType` argument is passed directly to the verifier without a
policy-spec lookup:

- **Cosign attestors** — the value is the in-toto predicate type, e.g.
  `"https://slsa.dev/provenance/v1"` or `"https://cyclonedx.org/bom"`.
- **Notary attestors** — the value is the OCI referrer artifact type, e.g.
  `"application/vnd.cncf.notary.signature.v2.payload"`.

This differs from the `imageverify` lib, where `attestationType` is a **name**
looked up in the policy's `spec.attestations` list. In the standalone lib, the
full type URI is passed directly in the CEL expression.

### No image-reference filter

The `imageverify` lib applies the policy's `spec.matchImageReferences` guard before
calling the verifier. The `attestation` lib does not — all OCI references are
accepted. This is intentional: the attestation lib is designed for arbitrary OCI
artifacts, not just container images.

## Architecture

```
pkg/cel/libs/
├── imageverify/          ← unchanged; used by IVPol compiler
│   ├── lib.go            Lib(version, imgCtx, ivpol, lister) — ivpol-bound
│   ├── impl.go           looks up attestation from policy spec
│   └── utils.go
└── attestation/          ← new; used by VPOL compiler
    ├── lib.go            Lib(version, imgCtx, lister) — policy-neutral
    ├── impl.go           constructs Attestation inline from type string
    └── impl_test.go
```

The two libraries register the same CEL function names. They are never in the
same `cel.Env` because IVPol and VPOL use separate compiler pipelines.

### `NewCompiler` signature change

`pkg/cel/policies/vpol/compiler.NewCompiler` now accepts two optional parameters:

```go
func NewCompiler(imgCtx imagedataloader.ImageContext, lister k8scorev1.SecretInterface) Compiler
```

- **`imgCtx`** — OCI image context for fetching artifacts. When `nil`, an
  unauthenticated context is created internally (works for public registries).
- **`lister`** — Kubernetes `SecretInterface` for pull-secret authentication.
  When provided, private registries protected by imagePullSecrets are accessible.

Pass `nil, nil` in contexts where image registry access is not needed (policy
validation, CLI dry-run, background report scanning).

## CI/CD Gate Pattern

The intended use case is a Kubernetes-native pipeline gate:

1. The CI pipeline creates a custom resource (e.g. `PipelineGate`) referencing
   the OCI artifact it just built.
2. A `ValidatingPolicy` verifies the attestation on that artifact before allowing
   the resource to be admitted.
3. `PolicyException` provides per-gate bypass capability.

### Example: require SLSA provenance

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-slsa-provenance
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: ["ci.example.com"]
      apiVersions: ["v1"]
      resources: ["pipelinegates"]
      operations: ["CREATE", "UPDATE"]
  validations:
  - expression: >
      verifyAttestationSignatures(
        object.spec.ociRef,
        "https://slsa.dev/provenance/v1",
        [{"cosign": {"keyless": {
          "issuer": "https://token.actions.githubusercontent.com",
          "subject": "https://github.com/example/repo/.github/workflows/release.yaml@refs/heads/main"
        }}}]
      ) > 0
    message: "SLSA provenance attestation from GitHub Actions is required"
```

### Example: exception for a specific gate

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: allow-manual-gate
spec:
  policyName: require-slsa-provenance
  matchConditions:
  - name: manual-override
    expression: "object.metadata.labels['ci.example.com/override'] == 'manual'"
```

### Example: extract and inspect the attestation payload

Use `verifyAttestationSignatures` first to verify the signature, then
`extractPayload` to read the payload in subsequent `variables`:

```yaml
spec:
  variables:
  - name: payload
    expression: >
      verifyAttestationSignatures(
        object.spec.ociRef,
        "https://slsa.dev/provenance/v1",
        [{"cosign": {"keyless": {"issuer": "https://token.actions.githubusercontent.com"}}}]
      ) > 0
        ? extractPayload(object.spec.ociRef, "https://slsa.dev/provenance/v1")
        : null
  validations:
  - expression: "variables.payload != null"
    message: "verified SLSA provenance payload is required"
  - expression: >
      variables.payload != null &&
      variables.payload.predicate.buildType == "https://github.com/slsa-framework/slsa-github-generator/..."
    message: "build must use the SLSA GitHub generator"
```

## Authenticated Registries

For private OCI registries, configure image pull secrets as Kubernetes `Secret`
resources in the Kyverno namespace. The `lister` passed to `NewCompiler` at
startup gives the attestation lib access to those secrets.

The production wiring in `cmd/kyverno/main.go` passes the secrets client:

```go
compiler := vpolcompiler.NewCompiler(
    nil,
    setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
)
```

## Testing

Unit tests live alongside the library at
`pkg/cel/libs/attestation/impl_test.go`. Integration tests demonstrating the
full VPOL compiler+engine pipeline are in
`pkg/cel/policies/vpol/engine/engine_test.go` under the `TestVPOL_Attestation*`
test functions.

The network-dependent evaluation tests (`TestVerifyImageSignatures_Notary`,
`TestVerifyAttestationSignatures_Notary`) run against
`ghcr.io/kyverno/test-verify-image:signed` and require network access.

To run only the non-network tests:

```sh
go test ./pkg/cel/libs/attestation/... -run 'TestLib_Compile|TestLib_Nil|TestLib_Same|TestLib_NoImage|TestLib_Inline'
go test ./pkg/cel/policies/vpol/engine/... -run 'TestVPOL_Attestation'
```
