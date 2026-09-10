# Related repositories

The `kyverno` GitHub org has ~48 repos. This is the subset that actually matters when working in
`kyverno/kyverno`, and why each one is separate rather than a folder in this repo.

| Repo | What it holds | Why it's separate |
|---|---|---|
| [`kyverno/api`](https://github.com/kyverno/api) | The `policies.kyverno.io` CEL-policy CRD Go types (ValidatingPolicy, MutatingPolicy, GeneratingPolicy, DeletingPolicy, ImageValidatingPolicy, PolicyException) | Deliberate: lets external Go projects import Kyverno's API types without pulling in the full controller codebase and its dependency tree. See [ADR-0003](../decisions/ADR-0003-policies-api-externalized.md). |
| [`kyverno/KDP`](https://github.com/kyverno/KDP) | Kyverno Design Proposals | The project's actual design-proposal process. **Any substantial design work belongs there, not in a new in-repo process** — `docs/designs/` in this repo is explicitly scoped to notes too small for KDP, see its README. |
| [`kyverno/website`](https://github.com/kyverno/website) | User-facing documentation (kyverno.io) | User docs have their own build/publish pipeline and audience; `README.md`/`CONTRIBUTING.md` already point PRs with doc impact there. |
| [`kyverno/community`](https://github.com/kyverno/community) | Governance, Code of Conduct, Security policy, Maintainers/Contributors lists, `AI_USAGE_POLICY.md` | Org-wide policy shared across all `kyverno/*` repos, not specific to this codebase. `GOVERNANCE.md`, `SECURITY.md`, `MAINTAINERS.md`, `CONTRIBUTORS.md`, `CODE_OF_CONDUCT.md` in this repo are intentionally thin redirects here. |
| [`kyverno/chainsaw`](https://github.com/kyverno/chainsaw) | The declarative e2e test tool | `test/conformance/chainsaw/` depends on it as a tool, but chainsaw itself is a general-purpose Kubernetes testing project with its own users beyond Kyverno. |
| `kyverno/policies`, `kyverno/policy-reporter*` (4 repos), `kyverno/kyverno-json`, `kyverno/sdk`, `kyverno/playground`, `kyverno/pod-security-admission` | Adjacent ecosystem projects (sample policies, the Policy Reporter product family, a JSON-native policy engine, an SDK, the web playground) | Genuinely separate products under the same org, some with their own release cadence and language stack. Out of scope for this repo's structure. |

## On merging `kyverno/api` back into this repo

Go workspaces and per-module monorepos are an increasingly common pattern precisely because splitting related code
across repos can leave an agent (or a human) blind to the type definitions it's calling. That's a real cost of the
current split. But undoing it is a **maintainer/release-engineering decision**, not something this doc-structure
effort should do unilaterally — it would change `kyverno/api`'s independent release cadence and require a
deprecation/redirect story for every external importer of `github.com/kyverno/api`.

**In the meantime**, if you're working across the `kyverno`/`api` boundary: check both repos out as sibling
directories and use Claude Code's `--add-dir ../api` (or a committed `additionalDirectories` entry) so your agent
session sees both repos' `AGENTS.md` and code at once, without needing the repos actually merged.
