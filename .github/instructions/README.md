Path-specific custom instructions for GitHub Copilot — each `*.instructions.md` file here has an `applyTo: <glob>`
frontmatter field and is read by Copilot (Chat, code review, and the coding agent) whenever it's working on a
matching file, combined with `.github/copilot-instructions.md` rather than replacing it. This is a Copilot-specific
mechanism, not a cross-tool convention — other AI tools (including this repo's own `AGENTS.md` files, which Claude
Code and others read) don't look here.

It exists because `.github/copilot-instructions.md` documents that Copilot's code review only auto-reads the
*root* `AGENTS.md`, not the nested per-package ones this repo has. Each file here routes Copilot to one package's
`AGENTS.md` for a high-traffic path that would otherwise be invisible to it:

| File | Applies to | Points at |
|---|---|---|
| `api.instructions.md` | `api/**` | `api/AGENTS.md`, `docs/context/shared/api-versioning.md` |
| `engine.instructions.md` | `pkg/engine/**` | `pkg/engine/AGENTS.md` |
| `webhooks.instructions.md` | `pkg/webhooks/**` | `pkg/webhooks/AGENTS.md` |

To add one for another package: create `<name>.instructions.md` with `applyTo: '<path>/**'` frontmatter, and point
it at that package's `AGENTS.md` the same way.
