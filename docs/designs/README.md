# Design notes

This directory is for lightweight, in-repo technical notes — not the project's design-proposal process.

**Substantial design proposals belong in [kyverno/KDP](https://github.com/kyverno/KDP)** (Kyverno Design
Proposals), which is the actual, existing venue for that (see `ROADMAP.md`). Don't duplicate or compete with it.

Use a file here instead of a KDP proposal only when **all** of the following are true:
- The design is too small or too implementation-specific to justify a full KDP proposal.
- It's still useful to have the reasoning reviewed atomically with code, or preserved for whoever touches this area
  next — not just left in a PR description that gets harder to find over time.
- It's not a lasting architectural decision on its own (if it is, it belongs in
  [docs/decisions/](../decisions/) as an ADR instead).

Each file should say up front what it is (status: in progress / done / superseded) and link to the GitHub issue or
PR it belongs to. See [agent-friendly-restructure.md](./agent-friendly-restructure.md) for a working example — a
status note for an in-progress, multi-PR effort, not a finished proposal.

When a note here stops being useful as a live reference (the work is done and nothing points back to it anymore),
move it to [docs/archive/](../archive/) rather than deleting it — see that directory's README for the process.
