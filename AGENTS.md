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
  readiness-checker/  #   Readiness checker
  internal/           #   Shared internal cmd helpers
  tools/              #   Internal tooling (webhook-cleanup, etc.)
pkg/                  # Core library code
  engine/             #   Policy engine (rule evaluation, matching, context)
  webhooks/           #   Admission webhook handlers
  controllers/        #   Controller implementations
  cel/                #   CEL-based policy evaluation
  client/             #   Generated Kubernetes clientset, listers, informers
  clients/            #   Client wrappers with tracing/metrics
  config/             #   Runtime configuration
  toggle/             #   Feature flags
  logging/            #   Structured logging utilities
  metrics/            #   Prometheus metrics
  validation/         #   Policy validation logic
  autogen/            #   Auto-generation of rules for Pod controllers
  cosign/             #   Cosign image signature verification
  notary/             #   Notary image signature verification
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

Kyverno runs as multiple controllers in a Kubernetes cluster:

- **Admission Controller** (`cmd/kyverno/`): The core component. Receives AdmissionReview requests, evaluates validate/mutate rules synchronously, and queues generate/audit rules for async processing. Also validates policies and configures webhooks.
- **Background Controller** (`cmd/background-controller/`): Handles generate and mutate-existing rules for existing resources via UpdateRequests.
- **Reports Controller** (`cmd/reports-controller/`): Creates policy reports from admission and background scans. Aggregates intermediary reports into `PolicyReport`/`ClusterPolicyReport`.
- **Cleanup Controller** (`cmd/cleanup-controller/`): Executes resource deletion based on `CleanupPolicy`/`ClusterCleanupPolicy` via CronJobs.
- **Init Container** (`cmd/kyverno-init/`): Runs pre-flight cleanup before the admission controller starts.
- **CLI** (`cmd/cli/kubectl-kyverno/`): Offline policy testing and validation tool.

Controller code is primarily in `pkg/controllers/`. Webhook handlers are in `pkg/webhooks/`. The policy engine is in `pkg/engine/`.

## API Design Rules

- API types live in `api/` with versioned packages
- API groups: `kyverno.io`, `policies.kyverno.io`, `policyreport.io`, `reports.kyverno.io`
- New resource types must NOT be added to `kyverno.io/v1`; use `v2alpha1` and promote as they stabilize
- New attributes can be added without a new version
- Attributes cannot be deleted or modified in a version; deprecate and remove after 3 minor releases
- Newer API versions may reference older stable types, but not vice versa

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

## Useful References

- [Development Guide](./DEVELOPMENT.md) — Full build, test, debug, and deploy instructions
- [Contributing Guide](./CONTRIBUTING.md) — Contribution process and PR guidelines
- [API Design](./docs/dev/api/README.md) — API versioning and extension rules
- [Controllers Design](./docs/dev/controllers/README.md) — Controller list and internals
- [Logging](./docs/dev/logging/logging.md) — Logging levels and conventions
- [Feature Flags](./docs/dev/feature-flags/README.md) — How to add and use feature toggles
- [Reports Design](./docs/dev/reports/README.md) — Report architecture
- [Kyverno Docs](https://kyverno.io) — User-facing documentation
