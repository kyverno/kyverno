# Dependency Management Policy

This document describes how the Kyverno project selects, obtains, and tracks its third-party dependencies. It satisfies [OSPS-DO-06](https://baseline.openssf.org/versions/2026-02-19#osps-do-06---publish-dependency-management-policy) of the Open Source Project Security Baseline.

## Language Dependencies

Kyverno is written in Go. All direct and transitive Go module dependencies are declared in [`go.mod`](go.mod) and pinned to exact versions in [`go.sum`](go.sum). Contributors run `go mod tidy` to keep the dependency list minimal and consistent; CI enforces this via the `unused-package-check` target.

## Selection Criteria

When evaluating a new dependency, maintainers consider:

- **Necessity** — Can the functionality be achieved without an external dependency?
- **Maintenance status** — Is the project actively maintained with timely security fixes?
- **License compatibility** — Is the license compatible with Apache 2.0?
- **Security posture** — Does the project follow responsible disclosure practices?
- **Ecosystem trust** — Is it widely adopted in the Kubernetes/Go ecosystem?

New dependencies require maintainer approval during code review.

## Automated Dependency Updates

[Dependabot](https://github.com/dependabot/dependabot-core) is configured to monitor and propose updates daily for:

| Ecosystem | Scope | Schedule |
|-----------|-------|----------|
| Go modules (`gomod`) | `/`, `/hack/controller-gen/`, `/hack/api-group-resources/` | Daily |
| GitHub Actions | `/`, `/.github/actions/*/` | Daily |

Dependabot groups related packages (Kubernetes, Sigstore, OpenTelemetry) to reduce noise. All proposed updates are reviewed by maintainers and must pass CI before merging.

## Vulnerability Scanning

- **Trivy** scans container images on every push and on a periodic schedule (see `.github/workflows/trivy.yaml` and `trivy-periodic-scan.yaml`). Findings above the configured severity threshold are automatically opened as GitHub issues.
- **golangci-lint with gosec** runs on every pull request to detect insecure coding patterns in Go source (see `.github/workflows/lint.yaml`).
- **OpenSSF Scorecard** runs weekly to assess the project's overall supply-chain security posture (see `.github/workflows/scorecard.yaml`).

## Pinning and Reproducibility

- Go module dependencies are pinned by content hash in `go.sum`.
- GitHub Actions are pinned to full commit SHAs in all workflow files.
- Container base images are referenced by digest where possible.

## Dependency Lifecycle

Kyverno publishes a [compatibility matrix](https://kyverno.io/docs/installation/#compatibility-matrix) documenting which versions of Kubernetes are supported by each Kyverno release. Dependencies that fall out of upstream support are upgraded or replaced as part of the regular release cycle.

## Third-Party Attribution

Kyverno distributes third-party software as part of its container images and Helm chart. Attribution notices are provided in the following locations:

- **[NOTICE](NOTICE)** — repo-root notice file listing project copyright and notable upstream attributions
- **`/var/run/ko/NOTICE`** and **`/var/run/ko/LICENSE`** — embedded in each container image at build time via `ko`'s `kodata` mechanism
- **[FOSSA](https://app.fossa.com/projects/git%2Bgithub.com%2Fkyverno%2Fkyverno)** — machine-readable inventory of all third-party licenses, updated on every push to `main`

To produce a per-package license report from the current dependency graph:
```bash
go install github.com/google/go-licenses@v1.6.0 && go-licenses report ./cmd/kyverno/...
```
The `NOTICE` file is maintained manually; the command above outputs a CSV that can be used to verify or update the counts and notable components it contains.

## Vulnerability Remediation SLA

When a vulnerability is confirmed in Kyverno or one of its dependencies, the security response team targets the following fix timelines, measured from the date of confirmed triage:

| Severity | CVSS Score | Target Fix Window |
|----------|------------|-------------------|
| Critical | 9.0 – 10.0 | 7 days |
| High     | 7.0 – 8.9  | 14 days |
| Medium   | 4.0 – 6.9  | 28 days |
| Low      | < 4.0      | Next minor release |

These timelines are targets, not guarantees. Complex vulnerabilities, coordinated disclosure embargoes, or upstream dependency blockers may require adjusted timelines; any deviation is communicated in the associated GitHub issue or security advisory.

Vulnerabilities are reported via the process described in [SECURITY.md](SECURITY.md).

## Related Resources

- [go.mod](go.mod) — Go module dependency list
- [.github/dependabot.yml](.github/dependabot.yml) — Dependabot configuration
- [Compatibility Matrix](https://kyverno.io/docs/installation/#compatibility-matrix) — Supported Kubernetes versions
- [SECURITY-INSIGHTS.yml](SECURITY-INSIGHTS.yml) — Machine-readable security metadata
