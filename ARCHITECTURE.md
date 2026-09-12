# Architecture

A navigable map of how kyverno/kyverno is put together: the binaries it ships, how `pkg/` is layered, which
mechanisms are sanctioned for new code, and which parts of the tree are generated and must never be hand-edited.

This file describes **what exists today**, verified against code in this session. For narrower, per-package detail
see the nested `AGENTS.md` files linked from each section, and `docs/context/` for cross-cutting write-ups. ADRs for
*why* a non-obvious structural choice was made don't exist yet — see the "what's left" list in
[docs/designs/agent-friendly-restructure.md](docs/designs/agent-friendly-restructure.md).

## Runtime boundaries (`cmd/`)

Kyverno ships as several independently deployed binaries. Most are composed from shared helpers in `cmd/internal/`
(`internal.WithKubeconfig()`, `internal.WithKyvernoClient()`, ...): `kyverno`, `kyverno-init`, `cleanup-controller`,
`reports-controller`, and `background-controller` all import it. `kubectl-kyverno` (the CLI) and `readiness-checker`
do not — both do their own minimal, standalone setup instead.

| Binary | Source | Purpose |
|---|---|---|
| `kyverno` | `cmd/kyverno/` | The admission controller. Runs the webhook server (sync validate/mutate), policy/report/webhook/certmanager controllers, and the CEL-based engines (`pkg/cel/policies/{vpol,mpol,gpol,ivpol}`) alongside the legacy `kyverno.io` engine. |
| `kyvernopre` | `cmd/kyverno-init/` | Init container. Cleans up stale webhook configurations before the admission controller starts. |
| `kubectl-kyverno` | `cmd/cli/kubectl-kyverno/` | Offline CLI for applying/testing policies without a cluster. Built on library code in `pkg/cli` (distinct from the `cmd/cli` command tree itself). |
| `cleanup-controller` | `cmd/cleanup-controller/` | Reconciles `CleanupPolicy`/`ClusterCleanupPolicy`, runs a cleanup-policy validation webhook, and handles CronJob-invoked cleanup plus the CEL `dpol` (DeletingPolicy) engine. |
| `reports-controller` | `cmd/reports-controller/` | Produces and aggregates `PolicyReport`/`ClusterPolicyReport` from admission and background scans. |
| `background-controller` | `cmd/background-controller/` | Processes `generate` and `mutate-existing` rules against already-existing resources via `UpdateRequest`s, plus the CEL `gpol`/`mpol` background paths. |
| `readiness-checker` | `cmd/readiness-checker/` | Small utility binary used by probes/Helm hooks: `check-endpoints`, `check-http`, `scale-deploy`, `delete-webhooks`. |

## Domain layering (`pkg/`)

Roughly bottom-up; lower layers have few or no dependencies on higher ones.

**Foundational (near-leaf):** `logging` (logr + zerologr backend, everything depends on it), `toggle` (feature-flag
registry — see [pkg/toggle/AGENTS.md](pkg/toggle/AGENTS.md)), `config` (runtime/ConfigMap config), `tracing` /
`metrics` (OpenTelemetry + Prometheus, used pervasively by `pkg/clients`), `tls`, `leaderelection`, `deprecations`
(maps legacy `kyverno.io` kinds to their `policies.kyverno.io` replacements), `breaker`, `informers`, `profiling`,
`pss`, `auth`, `userinfo`.

**Client access:** `pkg/client/` is 100% generated typed clientset/informers/listers, covering both kyverno's own CRDs
and the externally-defined `policies.kyverno.io` types from `github.com/kyverno/api` — never hand-edited, regenerate
via `make codegen-client-all`. `pkg/clients/` is a second, distinct generation layer (instrumented metrics+tracing
+logging wrapper clients, produced by `hack/client-wrapper`) that actually gets consumed by business logic.
`pkg/clients/dclient` is hand-written and is the dominant pattern for new code needing dynamic/discovery-based access
across arbitrary GVKs; it embeds no instrumentation decorators of its own, but every production binary constructs it
via `cmd/internal.Setup`, which hands it already metrics/tracing-wrapped dynamic and kube clients — see
[pkg/clients/AGENTS.md](pkg/clients/AGENTS.md) for the exact boundary. **Prefer `pkg/clients/dclient.Interface` for
generic resource access, or the typed `pkg/clients/{kube,kyverno}` wrappers when the type is known — never construct
a raw `pkg/client`/`k8s.io/client-go` client directly in new controller/webhook code** (accepting one of those
packages' *interface* types as a function parameter is fine and common — the generated wrappers embed those same
interfaces — the rule is about what constructs the concrete client, not what type a function signature names).

**Policy engine core:** `pkg/engine` is the legacy (`kyverno.io`) rule-evaluation engine — JMESPath variable
substitution, pattern/anchor matching, mutate patch generation, external API/context calls. `pkg/cel` is the newer,
parallel CEL-based engine stack for `policies.kyverno.io` types, split by policy kind: `vpol`
(ValidatingPolicy), `mpol` (MutatingPolicy), `gpol` (GeneratingPolicy), `dpol` (DeletingPolicy), `ivpol`
(ImageValidatingPolicy). It sits alongside `pkg/engine`, not layered under it — both are invoked from `cmd/kyverno`
and the other controller binaries for their respective policy families. See [pkg/engine/AGENTS.md](pkg/engine/AGENTS.md)
and [pkg/cel/AGENTS.md](pkg/cel/AGENTS.md). `pkg/admissionpolicy` bridges CEL policy authoring to native Kubernetes
`ValidatingAdmissionPolicy`/`MutatingAdmissionPolicy`. `pkg/autogen` (and `pkg/cel/autogen`) compute autogen rules
for Pod-controller kinds. `pkg/image` holds all image signature/attestation verification — cosign and notary
verifiers live under `pkg/image/verifiers/{cpol,ivpol}/{cosign,notary}` (not `pkg/cosign`/`pkg/notary`, which no
longer exist); `pkg/sigstoretuf` handles TUF trust-root management for the cosign verifiers. See
[pkg/image/AGENTS.md](pkg/image/AGENTS.md).

**Policy lifecycle / admission:** `pkg/policy`, `pkg/validation`, `pkg/exceptions` (`PolicyException` matching),
`pkg/policycache` (informer-driven in-memory cache read at webhook time), `pkg/globalcontext` (`GlobalContextEntry`
caching for JMESPath/CEL context). `pkg/webhooks` holds the actual `AdmissionReview` HTTP handlers — the
synchronous validate/mutate path returns immediately, while generate/mutate-existing rules are queued as
`UpdateRequest`s for `pkg/background` to process asynchronously. See [pkg/webhooks/AGENTS.md](pkg/webhooks/AGENTS.md)
and [pkg/background/AGENTS.md](pkg/background/AGENTS.md). `pkg/controllers/*` wires all of the above into
controller-runtime-style reconcilers; see [pkg/controllers/AGENTS.md](pkg/controllers/AGENTS.md) for the
subdirectory → responsibility → leader-election map.

**Reporting:** `pkg/openreports` adapts Kyverno's internal report model to the upstream `openreports.io` API,
consumed by `pkg/controllers/report/*` and `cmd/reports-controller`.

**Cross-cutting outcomes:** `pkg/event` is the sanctioned way engine/webhook/background code surfaces user-facing
pass/fail/block outcomes as Kubernetes Events (`event.NewPolicyFailEvent(...)` etc.) — plain Go `error` handling
remains standard everywhere else; `pkg/event` is specifically for recording a policy *decision*, not general error
propagation.

## API layer (`api/`)

```text
api/kyverno/{v1,v1beta1,v2,v2alpha1,v2beta1}
api/policyreport/v1alpha2   (wgpolicyk8s.io)
api/reports/v1              (reports.kyverno.io)
```

**`policies.kyverno.io` (the CEL-based types — ValidatingPolicy, MutatingPolicy, GeneratingPolicy, DeletingPolicy,
ImageValidatingPolicy, PolicyException, and their Namespaced\* variants) is NOT in this repo.** It lives in the
external `github.com/kyverno/api` module (pinned in `go.mod`), imported as e.g.
`policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"`. It was split out deliberately so
external Go projects can import Kyverno's API types without the whole controller dependency tree (no backfilled ADR
exists for this yet — see [docs/context/shared/repo-boundaries.md](docs/context/shared/repo-boundaries.md) for the
current writeup). Only the generated CRD manifests (`config/crds/policies.kyverno.io/*.yaml`) and consuming Go code
remain here.

Versioning rules (convention, enforced by reviewers, not tooling): new types are never added to `v1`/`v2`, only to
`v2alpha1` and promoted as they stabilize; attributes can be added but never deleted/modified within a version
(deprecate and remove after 3 minor releases instead); newer versions may reference older stable types, never the
reverse. Full detail, kept in one place: [docs/context/shared/api-versioning.md](docs/context/shared/api-versioning.md).
See also [api/AGENTS.md](api/AGENTS.md).

## Sanctioned defaults

- **Client access:** `pkg/clients/dclient` for dynamic/discovery-based access; typed `pkg/clients/{kube,kyverno}`
  wrappers when the type is known. Never construct a raw `pkg/client`/`client-go` client in new code (accepting
  those interface types as a parameter is fine — see the client-access section above).
- **Logging:** `logr` API, `zerologr` backend for app logs, `klogr` for client-go/K8s library logs. Levels: L0
  errors, L2 startup/policy-application results, L3 variable evaluation/intermediate decisions, L4+ deep debugging.
  Full detail: [docs/dev/logging/logging.md](docs/dev/logging/logging.md).
- **Feature flags:** `pkg/toggle` — env var + CLI flag + a method on the `Toggles` interface, read via
  `toggle.FromContext(ctx).<Feature>()` — is the preferred mechanism for shared, env-backed toggles (a separate
  container-argument-only convention also exists for flags that don't need this). Full detail:
  [docs/dev/feature-flags/README.md](docs/dev/feature-flags/README.md), and [pkg/toggle/AGENTS.md](pkg/toggle/AGENTS.md)
  for implementation gotchas.
- **User-facing outcomes:** `pkg/event`, not ad hoc logging (see above).
- **Outbound HTTP:** `pkg/tracing`'s OTel-instrumented `http.RoundTripper`, used by `pkg/engine/apicall` for
  external context calls. There is no separate generic HTTP client package beyond this.

## Generated / no-edit zones

Confirmed via `DO NOT EDIT` headers and Makefile targets:

| Path | Regenerate with |
|---|---|
| `pkg/client/{clientset,informers,listers}/**` | `make codegen-client-all` |
| `api/**/zz_generated.{deepcopy,register}.go` | `make codegen-api-all` |
| `pkg/config/mocks/mock_config.go` | no `make` target — see note below |
| `config/crds/**/*.yaml` | `make codegen-crds-all` |
| `docs/user/crd/**` (HTML reference) | `make codegen-api-docs` |
| `charts/*/README.md` | `make codegen-helm-all` (helm-docs) |

`pkg/config/mocks/mock_config.go` carries a `Code generated by MockGen. DO NOT EDIT.` header, but there's no
`go:generate` directive and no Makefile target that produces it (`mockgen` isn't in `make install-tools`'s tool
list) — it's the repo's only gomock file and was evidently generated by hand once and committed. Regenerating it
means installing `mockgen` yourself and rerunning it against `pkg/config/config.go`.

`pkg/clients/**/*.generated.go` and `pkg/clients/**/interface.generated.go` (produced by `hack/client-wrapper` via
`make codegen-client-wrappers`) belong on this list too, even though — unlike everything else above — they carry no
`DO NOT EDIT` header today. Treat them as generated regardless. A `.claude/settings.json` with `permissions.deny`
rules enforcing this by path is planned but not yet added — see
[docs/designs/agent-friendly-restructure.md](docs/designs/agent-friendly-restructure.md).

## Ownership

`CODEOWNERS` is the enforced source of truth for review requirements. [OWNERS.md](OWNERS.md) is the readable
subsystem-ownership map (who owns what area) that `docs/context/components/*.md` files cite. `MAINTAINERS.md`
redirects to `kyverno/community`'s org-wide maintainer list — a different, narrower purpose than `OWNERS.md`.

## Related repositories

Kyverno's code and process live in more than one GitHub repo. See
[docs/context/shared/repo-boundaries.md](docs/context/shared/repo-boundaries.md) for the full list and why each is
separate — most importantly `kyverno/api` (CEL policy CRD types) and `kyverno/KDP` (design proposals: any new design
work belongs there, not in a new in-repo proposal process).

## Test architecture

`test/cli/` (fixtures for `kubectl-kyverno test`), `test/conformance/chainsaw/` (the e2e conformance suite — 1000+
`chainsaw-test.yaml` cases, the largest single source of truth for runtime behavior), `test/fuzz/` (Go native fuzz +
OSS-Fuzz), `test/policy/` (reusable hand-authored policy fixtures), `test/integration/` (Go integration tests for
the CEL policy kinds). See [docs/dev/README.md](docs/dev/README.md) and the
[chainsaw quick-start](https://kyverno.github.io/chainsaw/latest/quick-start/) for how to add a conformance case —
there's no co-located skill for this yet (tracked as future work in
[docs/designs/agent-friendly-restructure.md](docs/designs/agent-friendly-restructure.md)).
