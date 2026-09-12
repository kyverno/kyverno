# Design notes

This directory is for lightweight, in-repo technical notes — not the project's design-proposal process.

**Substantial design proposals belong in [kyverno/KDP](https://github.com/kyverno/KDP)** (Kyverno Design
Proposals), which is the actual, existing venue for that (see `ROADMAP.md`). Don't duplicate or compete with it.

Use a file here instead of a KDP proposal only when **all** of the following are true:
- The design is too small or too implementation-specific to justify a full KDP proposal.
- It's still useful to have the reasoning reviewed atomically with code, or preserved for whoever touches this area
  next — not just left in a PR description that gets harder to find over time.
- It's not a lasting architectural decision on its own (if it is, it will belong in a `docs/decisions/` ADR instead
  — that directory doesn't exist yet; see the "what's left" list in
  [agent-friendly-restructure.md](./agent-friendly-restructure.md)).

Each file should say up front what it is (status: in progress / done / superseded) and link to the GitHub issue or
PR it belongs to. See [agent-friendly-restructure.md](./agent-friendly-restructure.md) for a working example — a
status note for an in-progress, multi-PR effort, not a finished proposal.

When a note here stops being useful as a live reference (the work is done and nothing points back to it anymore), it
will eventually move to a `docs/archive/` directory rather than being deleted — that directory and its README don't
exist yet either; see the "what's left" list linked above for the current plan.
