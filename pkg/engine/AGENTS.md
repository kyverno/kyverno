# AGENTS.md — pkg/engine

The legacy (`kyverno.io`) rule-evaluation engine was deleted in #17710 PR 4. This file is a placeholder;
the gotchas it used to document no longer apply to what remains here (`api`, `apicall`, `context`, `jmespath`,
`adapters`, `mutate/patch`, `anchor`/`pattern`/`validate`/`wildcards`/`operator`/`jsonutils`, the kept
`handlers.WithResponses` helper — see [ARCHITECTURE.md](../../ARCHITECTURE.md) for why each survives). A
rewrite describing the surviving packages lands in the final commit of that PR.
