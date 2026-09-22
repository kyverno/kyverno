# Design notes

This directory is for lightweight, in-repo technical notes. The project's
design-proposal process is now documented in
[docs/dev/proposals/](../dev/proposals/README.md).

**Substantial design proposals belong in
[docs/dev/proposals/](../dev/proposals/README.md)** (Kyverno Design Proposals).
The separate [kyverno/KDP](https://github.com/kyverno/KDP) repository contains
historical proposals and should not receive new proposals.

Use a file here instead of a proposal in `docs/dev/proposals/` only when
**all** of the following are true:
- The design is too small or too implementation-specific to justify a full KDP proposal.
- It's still useful to have the reasoning reviewed atomically with code, or preserved for whoever touches this area
  next — not just left in a PR description that gets harder to find over time.
- It's not a lasting architectural decision on its own (if it is, it will belong in a `docs/decisions/` ADR instead
  — that directory doesn't exist yet).

Each file should say up front what it is (status: in progress / done / superseded) and link to the GitHub issue or
PR it belongs to.

When a note here stops being useful as a live reference (the work is done and nothing points back to it anymore), it
will eventually move to a `docs/archive/` directory rather than being deleted — that directory and its README don't
exist yet either.
