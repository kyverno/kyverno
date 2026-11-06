# kyverno serve (experimental, v1alpha1)

> **Experimental developer preview.** `kyverno serve` and its `v1alpha1` HTTP API may change in any
> release without a deprecation cycle. It has no authentication or TLS, never writes to a cluster,
> evaluates one request at a time, and is meant for **local development and testing only**.

`kyverno serve` runs the Kyverno CLI policy evaluation as a local HTTP server. Use it to try
policies with `curl` or scripts, or to back tools such as the Kyverno Playground. It evaluates
through the same code path as `kyverno apply` (`cmd/cli/kubectl-kyverno/evaluator`), so a payload
gets the same results from both.

```sh
export KYVERNO_EXPERIMENTAL=true
kyverno serve --policies ./policies                       # all modes, 127.0.0.1:8080
kyverno serve --policies ./envoy --mode Envoy --address 127.0.0.1:9000
```

| Flag | Default | Description |
|---|---|---|
| `--policies` | (required) | Policy files or directories. Can be repeated or comma-separated. |
| `--mode` | all | The modes to serve: `Kubernetes`, `JSON`, `HTTP`, `Envoy`. Can be repeated or comma-separated. |
| `--address` | `127.0.0.1:8080` | Listen address. A non-loopback address prints a warning. |

**Startup is strict.** Policies are loaded once. The server refuses to start if:

- a path is unreadable;
- a document is not a supported policy;
- a legacy `kyverno.io` policy kind is present;
- or no policies are found.

This is unlike `apply`, which skips invalid files. A server that silently skipped a policy would
allow every later request that policy should have checked.

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1alpha1/evaluate/{mode}` | Evaluate a raw JSON or YAML body. `mode` is not case-sensitive. |
| `GET` | `/v1alpha1/policies` | List the loaded policies, the modes each applies to, and the policy-set digest. |
| `GET` | `/healthz`, `/readyz` | Health and readiness. |

### Request bodies

| Mode | Body | `apply` equivalent |
|---|---|---|
| `Kubernetes` | One or more resources: multi-document YAML, a JSON object, or a `v1/List` | `--resource` |
| `JSON` | One JSON or YAML object | `--json` |
| `HTTP` | A Kyverno Authz HTTP `CheckRequest` | `--http-payload` |
| `Envoy` | An Envoy ext_authz `CheckRequest` (protojson) | `--envoy-payload` |

Bodies are limited to 10 MiB.

For each Kubernetes-mode request, the resources in the body are loaded into an in-memory cluster.
Policies that look objects up see only those resources, the same as `kyverno apply -r`.

### Response

A completed evaluation returns **HTTP 200, even when a policy denies**:

```json
{
  "apiVersion": "serve.cli.kyverno.io/v1alpha1",
  "kind": "EvaluationResponse",
  "mode": "Kubernetes",
  "kyvernoVersion": "v1.20.0",
  "policySetDigest": "sha256:…",
  "decision": "Deny",
  "summary": { "pass": 1, "fail": 1, "warn": 0, "error": 0, "skip": 0 },
  "results": [
    {
      "policy": { "apiVersion": "policies.kyverno.io/v1beta1", "kind": "ValidatingPolicy", "name": "check-deployment-labels" },
      "rule": "check-deployment-labels",
      "type": "Validation",
      "status": "fail",
      "message": "Deployment labels must be env=prod",
      "resource": { "apiVersion": "apps/v1", "kind": "Deployment", "namespace": "default", "name": "bad-deployment" }
    }
  ],
  "artifacts": { "patched": [ … ], "generated": [ … ] }
}
```

`decision` is computed from the rule results, in this order of precedence:

1. `Error`: any rule errored.
2. `Deny`: any rule failed. This includes HTTP and Envoy denials.
3. `Allow`: at least one rule passed or warned.
4. `NoMatch`: nothing passed or failed, for example when no policy matched or every rule skipped.

**`NoMatch` and `Error` are never reported as `Allow`.**

- `artifacts.patched` holds the mutated resources or the mutated JSON payload.
- `artifacts.generated` holds the resources produced by GeneratingPolicies.
- `policySetDigest` is a SHA-256 hash of the loaded policies, bindings and exceptions. It does not
  depend on file layout or load order, so you can match a response to the exact policy set that
  produced it.

### Errors

Every non-2xx response has this body:

```json
{ "apiVersion": "serve.cli.kyverno.io/v1alpha1", "kind": "Status", "status": "Failure", "code": 400, "message": "…" }
```

| Code | When |
|---|---|
| 400 | Empty body, a body that does not parse, a JSON-mode body that is not a single object, or Kubernetes resources without `apiVersion` or `kind` |
| 404 | Unknown mode, mode not enabled with `--mode`, unknown route |
| 413 | Body over the size limit |
| 500 | The evaluation failed, including a recovered panic |

## Behavior notes

- **Same results as `apply`.** For the same policies and payload, `serve` reports the same per-rule
  results as `kyverno apply`. This includes apply's behavior of evaluating JSON-mode
  ValidatingPolicies against Kubernetes resources as raw JSON. To keep JSON-mode policies away from
  Kubernetes requests, serve them from a separate `--policies` directory.
- **Exceptions.** CEL `PolicyException`s found in `--policies` are always applied. This is the same
  as `apply --exceptions-within-policies`.
- **Not supported in v1alpha1.** Values files, user info, old objects or operations, cluster
  lookups, global context, Git or OCI policy sources, reload, AdmissionReview, SubjectAccessReview,
  Envoy gRPC ext_authz, TLS, and authentication.

## Testing

```sh
make test-cli-serve        # curl end-to-end checks for all four modes (needs curl and jq)
go test -race ./cmd/cli/kubectl-kyverno/commands/serve/ ./cmd/cli/kubectl-kyverno/evaluator/
```

Fixtures are in `test/cli/serve/`.
