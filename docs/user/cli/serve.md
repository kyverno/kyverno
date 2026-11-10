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
kyverno serve --allow-request-policies                   # policies arrive with each request (Playground)
```

| Flag | Default | Description |
|---|---|---|
| `--policies` | (required unless `--allow-request-policies`) | Policy files or directories. Can be repeated or comma-separated. |
| `--mode` | all | The modes to serve: `Kubernetes`, `JSON`, `HTTP`, `Envoy`. Can be repeated or comma-separated. |
| `--address` | `127.0.0.1:8080` | Listen address. A non-loopback address prints a warning. |
| `--allow-request-policies` | `false` | Accept policies inside `POST /v1alpha1/evaluate` requests. They are evaluated instead of `--policies`. |
| `--http-timeout` | `30s` | How long a policy HTTP call (CEL `http.Get`/`http.Post`) may wait for a response. Requests are serialized, so an unbounded call would stall every later request. |

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
| `POST` | `/v1alpha1/evaluate/{mode}` | Evaluate a raw JSON or YAML body against the server's policies. `mode` is not case-sensitive. |
| `POST` | `/v1alpha1/evaluate` | Evaluate an `EvaluationRequest`: a payload plus its context and, optionally, its policies. |
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

Evaluation requests must set `Content-Type`:

- `POST /v1alpha1/evaluate/{mode}`: `application/json`, `application/yaml`, `application/x-yaml`,
  `text/yaml` or `text/x-yaml`.
- `POST /v1alpha1/evaluate`: `application/json`.

Any other type, or none, is a 415. `curl --data-binary` sends `application/x-www-form-urlencoded`
unless you pass `-H`. These are not the "simple" types a web page can send without a CORS
preflight, so a page you visit cannot make your browser send policies to a local `kyverno serve`.
For the same reason, a server listening on a loopback address answers only requests whose `Host`
is a loopback name or address (`localhost`, `127.0.0.1`, `[::1]`), which blocks DNS rebinding.

For each Kubernetes-mode request, the resources in the body are loaded into an in-memory cluster.
Policies that look objects up see only those resources, the same as `kyverno apply -r`.

### EvaluationRequest (`POST /v1alpha1/evaluate`)

Every string field holds raw YAML or JSON, the same content the matching `kyverno apply` flag
would read from a file. Unknown fields are rejected.

| Field | `apply` equivalent | Description |
|---|---|---|
| `mode` (required) | | `Kubernetes`, `JSON`, `HTTP` or `Envoy` |
| `payload` (required) | `-r` / `--json` / `--http-payload` / `--envoy-payload` | What is evaluated, in the format the mode takes (see above) |
| `oldPayload` | | Kubernetes mode only: the previous state of the payload resources for `UPDATE`/`DELETE` (`oldObject`), matched by apiVersion, kind, namespace and name. Without it, a resource is its own old object, as in `apply`. |
| `policies` | positional policy paths | Policies evaluated instead of the server's set. Needs `--allow-request-policies`, otherwise 403. Loaded as strictly as at startup. |
| `exceptions` | `--exception` | CEL `PolicyException`s, applied in addition to any loaded with the policies |
| `context` | `--context-file` | A `cli.kyverno.io/v1alpha1` `Context`: the objects CEL `resource.get`/`resource.list` return, and image data |
| `clusterResources` | extra `-r` resources | Objects added to the in-memory cluster next to the payload. Namespace objects here are used for `namespaceSelector` and `namespaceObject`. |
| `parameterResources` | `--parameter-resource` | VAP/MAP parameter objects |
| `targetResources` | `--target-resource` | Existing objects that mutateExisting MutatingPolicies may mutate |
| `customResourceDefinitions` | `--crd-paths` | CRDs for custom kinds in the payload or targets. They apply to this request only. |
| `operation` | `request.operation` in `--values-file` | `CREATE` (default), `UPDATE` or `DELETE` |

```sh
jq -n --rawfile p policy.yaml --rawfile r pod.yaml \
  '{apiVersion: "serve.cli.kyverno.io/v1alpha1", kind: "EvaluationRequest", mode: "Kubernetes", policies: $p, payload: $r, operation: "UPDATE"}' |
  curl -s -X POST -H 'Content-Type: application/json' --data-binary @- http://127.0.0.1:8080/v1alpha1/evaluate
```

When the request carries `policies`, `policySetDigest` in the response is the digest of those
policies.

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

- Each result also carries the objects it produced:
  - `patchedResource`: the mutated object, on passing Mutation results.
  - `patchedTarget`: the mutated existing object, for mutateExisting.
  - `generatedResources`: the objects a Generation result created.
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
| 400 | Empty body, a body that does not parse, a JSON-mode body that is not a single object, an HTTP-mode body with unknown fields, Kubernetes resources without `apiVersion` or `kind`, or no policies at all. On `POST /v1alpha1/evaluate` also: an invalid `EvaluationRequest` (unknown field, data after the object, bad operation, request policies that fail to load), and an unknown `mode` or one not enabled with `--mode` |
| 403 | The request carries `policies` but the server was not started with `--allow-request-policies`, or a loopback server got a non-loopback `Host` |
| 404 | On `POST /v1alpha1/evaluate/{mode}`: an unknown mode or one not enabled with `--mode`. On any path: an unknown route |
| 405 | A known route called with the wrong method; the `Allow` header names the right one |
| 413 | Body over the size limit |
| 415 | Missing or unsupported `Content-Type` on an evaluate endpoint |
| 500 | The evaluation failed, including a recovered panic |

## Behavior notes

- **Same results as `apply`.** For the same policies and payload, `serve` reports the same per-rule
  results as `kyverno apply`. This includes apply's behavior of evaluating JSON-mode
  ValidatingPolicies against Kubernetes resources as raw JSON. To keep JSON-mode policies away from
  Kubernetes requests, serve them from a separate `--policies` directory.
- **Exceptions.** CEL `PolicyException`s found in `--policies` are always applied. This is the same
  as `apply --exceptions-within-policies`.
- **Namespaces.** Offline, `namespaceSelector` and `namespaceObject` first read the values file
  (`apply -f`). They then fall back to Namespace objects among the resources (`apply -r`) or
  `clusterResources`.
- **mutateExisting.** Targets are only mutated when the trigger matches the policy (its
  `matchConstraints`, `matchConditions`, and `mutateExisting` being enabled), as at admission.
- **Not supported in v1alpha1.** Values files, user info, global context entries,
  Git or OCI policy sources, reload, AdmissionReview, SubjectAccessReview, Envoy gRPC ext_authz,
  TLS, and authentication.
- **Request policies run arbitrary CEL** that the caller supplies. They are off by default. Only
  enable `--allow-request-policies` on a loopback address, or behind your own access control.

## Testing

```sh
make test-cli-serve        # curl end-to-end checks for all four modes (needs curl and jq)
go test -race ./cmd/cli/kubectl-kyverno/commands/serve/ ./cmd/cli/kubectl-kyverno/evaluator/
```

Fixtures are in `test/cli/serve/`.
