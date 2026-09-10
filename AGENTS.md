# AGENTS.md — Kyverno

This file provides context for AI coding agents working on the Kyverno codebase.

## Project Overview

Kyverno is a Kubernetes-native policy engine for security, compliance, automation, and governance through policy-as-code. It validates, mutates, generates, and cleans up Kubernetes resources using admission controls and background scans, and verifies container image signatures for supply chain security.

- **Language:** Go
- **Module:** `github.com/kyverno/kyverno`
- **License:** Apache 2.0
- **Go version:** See `go.mod` for the current version

## Repository Structure

```
api/                  # Kubernetes API type definitions (CRDs)
  kyverno/            #   kyverno.io API group (v1, v1beta1, v2, v2alpha1, v2beta1)
  policyreport/       #   wgpolicyk8s.io API group
  reports/            #   reports.kyverno.io API group
cmd/                  # Entry points for all binaries
  kyverno/            #   Main admission controller
  kyverno-init/       #   Init container (kyvernopre) — pre-flight resource cleanup
  cli/                #   Kyverno CLI (kubectl-kyverno)
  cleanup-controller/ #   Cleanup controller
  reports-controller/ #   Reports controller
  background-controller/ # Background controller (generate/mutate existing)
  readiness-checker/  #   Readiness checker (check-endpoints, check-http, scale-deploy, delete-webhooks)
  internal/           #   Shared bootstrap helpers composed by every cmd/*/main.go
pkg/                  # Core library code
  engine/             #   Policy engine (rule evaluation, matching, context)
  webhooks/           #   Admission webhook handlers
  controllers/        #   Controller implementations
  cel/                #   CEL-based policy evaluation
  client/             #   Generated Kubernetes clientset, listers, informers for kyverno's own CRDs (never hand-edit)
  clients/            #   Instrumented client wrappers (metrics/tracing/logging); dclient is the preferred
                      #   entry point for new code needing dynamic/discovery-based access to arbitrary GVKs
  config/             #   Runtime configuration
  toggle/             #   Feature flags
  logging/            #   Structured logging utilities
  metrics/            #   Prometheus metrics
  validation/         #   Policy validation logic
  autogen/            #   Auto-generation of rules for Pod controllers
  image/              #   Image signature/attestation verification (cosign + notary verifiers live under
                      #   image/verifiers/{cpol,ivpol}/{cosign,notary}; pkg/cosign and pkg/notary no longer exist)
  sigstoretuf/        #   TUF root/trust management for sigstore, used by pkg/image's cosign verifiers
  utils/              #   Shared utilities
ext/                  # Small standalone utility packages
charts/               # Helm charts
  kyverno/            #   Main Kyverno chart
  kyverno-policies/   #   Default policies chart
config/               # CRD manifests and install manifests
  crds/               #   Generated CRD YAML files
test/                 # Tests
  cli/                #   CLI test cases (kyverno test)
  conformance/        #   Conformance / e2e tests (chainsaw)
  fuzz/               #   Fuzz tests
  policy/             #   Policy test fixtures
docs/                 # Internal developer documentation
  dev/                #   API design, controllers, logging, feature flags, reports
scripts/              # Build and CI scripts
hack/                 # Code generation helpers
```

## Build System

The project uses `make` extensively. Tools are auto-installed into `.tools/` on first use.

### Key Build Commands

| Command | Description |
|---|---|
| `make build-all` | Build all binaries |
| `make build-kyverno` | Build the main kyverno binary → `cmd/kyverno/kyverno` |
| `make build-kyverno-init` | Build kyvernopre binary → `cmd/kyverno-init/kyvernopre` |
| `make build-cli` | Build CLI binary → `cmd/cli/kubectl-kyverno/kubectl-kyverno` |
| `make build-cleanup-controller` | Build cleanup controller binary |
| `make build-reports-controller` | Build reports controller binary |
| `make build-background-controller` | Build background controller binary |
| `make install-tools` | Install all development tools into `.tools/` |
| `make clean-tools` | Remove installed tools |

### Formatting & Linting

| Command | Description |
|---|---|
| `make fmt` | Run `go fmt ./...` |
| `make vet` | Run `go vet ./...` |
| `make imports` | Fix imports with `goimports` |
| `make fmt-check` | Verify formatting (fails if diff is non-empty) |
| `make imports-check` | Verify imports (fails if diff is non-empty) |
| `make unused-package-check` | Run `go mod tidy` check |

Linting is configured via `.golangci.yml` (golangci-lint v2). Enabled linters include `gosec`, `misspell`, `paralleltest`, `unconvert`, `errname`, `importas`, and others. The `importas` linter enforces specific import alias conventions — see `.golangci.yml` for the full alias rules.

Formatters enabled: `gci`, `gofmt`, `gofumpt`, `goimports`.

**Pre-commit checklist (required for code changes):**

- Run `make imports fmt` before committing.
- Run `make imports-check fmt-check` and ensure both pass.

**Pre-PR checks (required before opening/updating a PR):**

- Run `make codegen-all-code` then `make verify-codegen`.
- Run `./.tools/golangci-lint run` (install first with `make install-tools` if needed).


### Testing

| Command | Description |
|---|---|
| `make test-unit` | Run all unit tests with race detector and coverage |
| `make test-cli` | Run all CLI tests |
| `make test-cli-local` | Run local CLI test suite |
| `make test-clean` | Clear Go test cache |
| `make helm-test` | Run Helm chart tests |

- **Unit tests:** `go test -race -covermode atomic -coverprofile coverage.out ./...`
- **CLI tests:** Use `kubectl-kyverno test` against test fixtures in `test/cli/`
- **Conformance/E2E tests:** Use [chainsaw](https://kyverno.github.io/chainsaw/latest/quick-start/) in `test/conformance/`
- **Fuzz tests:** Located in `test/fuzz/`

### Code Generation

Code generation is heavily used. **Always run codegen after modifying API types.**

| Command | Description |
|---|---|
| `make codegen-all` | Run all code generation (code + docs) |
| `make codegen-all-code` | Generate all code (API, clients, CRDs, CLI, helm, manifests) |
| `make codegen-api-all` | Generate API register and deepcopy functions |
| `make codegen-client-all` | Generate clientset, listers, informers, wrappers |
| `make codegen-crds-all` | Generate all CRD manifests |
| `make codegen-helm-all` | Generate Helm chart CRDs and docs |
| `make verify-codegen` | Verify generated code is up to date (CI check) |

Generated files follow these patterns:
- `zz_generated.deepcopy.go` — deep copy functions in API packages
- `zz_generated.register.go` — API type registrations
- `pkg/client/` — generated clientset, listers, informers
- `config/crds/` — generated CRD YAML manifests

### Docker Images (ko)

| Command | Description |
|---|---|
| `make ko-build-all` | Build all local images with ko |
| `make ko-build-kyverno` | Build kyverno image locally |
| `make ko-publish-all` | Build and publish all images |

### Local Development with KinD

| Command | Description |
|---|---|
| `make kind-create-cluster` | Create a local KinD cluster |
| `make kind-delete-cluster` | Delete the KinD cluster |
| `make kind-load-all` | Build and load all images into KinD |
| `make kind-deploy-kyverno` | Build, load, and deploy Kyverno via Helm |
| `make kind-deploy-all` | Deploy Kyverno + default policies |

Override `KIND_IMAGE` for k8s version, `KIND_NAME` for cluster name.

## Architecture

See **[ARCHITECTURE.md](./ARCHITECTURE.md)** for the full map: runtime boundaries for every binary, how `pkg/`
is layered (including the legacy `pkg/engine` vs. the newer CEL `pkg/cel` stack), sanctioned defaults (which
client package to use, how feature flags and user-facing events work), and the confirmed generated/no-edit zones.
Read it before making a structural change; don't restate it here as this section drifts otherwise.

### Nested AGENTS.md files

These packages have real, package-specific gotchas that don't belong at root scope — read the relevant one before
working in that area: [`api/`](./api/AGENTS.md), [`pkg/engine/`](./pkg/engine/AGENTS.md),
[`pkg/cel/`](./pkg/cel/AGENTS.md), [`pkg/webhooks/`](./pkg/webhooks/AGENTS.md),
[`pkg/background/`](./pkg/background/AGENTS.md), [`pkg/image/`](./pkg/image/AGENTS.md),
[`pkg/clients/`](./pkg/clients/AGENTS.md), [`pkg/toggle/`](./pkg/toggle/AGENTS.md),
[`pkg/controllers/`](./pkg/controllers/AGENTS.md). Every claim in them is grounded in code actually read in the
session that wrote them, not inferred from directory structure — if one goes stale, fix it in place rather than
letting drift accumulate the way the old `pkg/cosign`/`cmd/tools` references did.

## API Design Rules

API types for `kyverno.io`, `policyreport` (`wgpolicyk8s.io`), and `reports.kyverno.io` live in `api/` with
versioned packages. **`policies.kyverno.io` (the CEL-based types) does not** — it lives in the external
`github.com/kyverno/api` module; see `api/AGENTS.md`. For versioning/stability/deprecation rules, see
[docs/context/shared/api-versioning.md](./docs/context/shared/api-versioning.md) — the single canonical copy;
don't restate rules here.

## Coding Conventions

- **Import aliases:** Enforced by `importas` linter. Key patterns:
  - `github.com/kyverno/kyverno/api/<group>/<version>` → `<group><version>` (e.g., `kyvernov1`)
  - `k8s.io/api/<group>/<version>` → `<group><version>` (e.g., `corev1`)
  - See `.golangci.yml` for the full alias table
- **Logging:** Uses `logr` with `zerologr` backend. Default level is 2. See `docs/dev/logging/logging.md`:
  - L0: Errors (with stack traces)
  - L2: Startup info, policy application results
  - L3: Variable evaluation, intermediate decisions
  - L4+: Debugging, execution path details
- **Feature flags:** Managed via the `pkg/toggle` package. Feature toggles are backed by environment variables and CLI flags. See `docs/dev/feature-flags/README.md`.
- **CGO:** Disabled (`CGO_ENABLED=0`)
- **Generated code:** Never edit files matching `zz_generated.*` or content in `pkg/client/` manually. Run `make codegen-all` instead.

## Pull Request Guidelines

- Provide proof manifests for maintainers to verify changes
- New/changed functionality requires a corresponding documentation issue/PR on the [website repo](https://github.com/kyverno/website)
- Test changes with the Kyverno CLI; provide test manifests
- For e2e-testable changes, write conformance tests using [chainsaw](https://kyverno.github.io/chainsaw/latest/quick-start/) in `test/conformance/`
- Run `make verify-codegen` to ensure generated code is up to date before submitting

### PR Failure-Prevention Checklist (DCO + Codegen + CI)

Use this checklist before every push to avoid repeated CI failures:

1. **Always sign commits (DCO)**
   - Create commits with signoff: `git commit -s ...`
   - Verify signoff trailers exist:
     `git log --format='%h %s%n%b' origin/main..HEAD | grep -n "Signed-off-by"`
   - If you forgot signoff on local commits, add it before pushing:
     `git rebase --signoff <base-commit-or-branch>`

2. **Run full codegen verification**
   - Run `make codegen-all` (not just `codegen-all-code`) when docs/manifests may be affected.
   - Run `make verify-codegen` and ensure it exits cleanly with no git diff.

3. **Run required pre-push checks**
   - `make imports fmt`
   - `make imports-check fmt-check`
   - Relevant test targets for changed areas (unit/CLI/chainsaw as applicable).

4. **Keep PR branch clean and reviewable**
   - Avoid merging `main` into the PR branch unless required; prefer rebasing.
   - Keep local-only artifacts untracked (for example, `.claude-*` folders).
   - Re-check status before push: `git status --short`.

5. **Confirm GitHub checks after push**
   - Inspect checks quickly: `gh pr checks <pr-number> -R kyverno/kyverno`
   - If a check fails, fetch failing logs immediately and fix in the next commit.

## Useful References

- [ARCHITECTURE.md](./ARCHITECTURE.md) — Runtime boundaries, `pkg/` domain layering, sanctioned defaults, no-edit zones
- [Development Guide](./DEVELOPMENT.md) — Full build, test, debug, and deploy instructions
- [Contributing Guide](./CONTRIBUTING.md) — Contribution process and PR guidelines
- [API Design](./docs/dev/api/README.md) — API versioning and extension rules
- [Controllers Design](./docs/dev/controllers/README.md) — Controller list and internals
- [Logging](./docs/dev/logging/logging.md) — Logging levels and conventions
- [Feature Flags](./docs/dev/feature-flags/README.md) — How to add and use feature toggles
- [Reports Design](./docs/dev/reports/README.md) — Report architecture
- [Kyverno Docs](https://kyverno.io) — User-facing documentation
