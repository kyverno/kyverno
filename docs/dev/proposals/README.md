# Kyverno Design Proposals

This directory will become the home for Kyverno Design Proposals (KDPs) after
the KDP handoff. Until then, the separate
[kyverno/KDP](https://github.com/kyverno/KDP) repository remains the
authoritative venue for new proposals and substantive revisions. After the
handoff, new proposals and substantive revisions belong here so that designs,
implementation, and repository history can evolve together. Existing
proposals in the KDP repository remain historical records. The KDP repository
should receive a redirect or archival update as part of that handoff.

Use [template.md](./template.md) when starting a proposal. A proposal should
explain the motivation, user-facing design, implementation approach, migration
impact, drawbacks, alternatives, prior art, unresolved questions, and CRD
changes. Open it as a pull request early enough for design review to influence
the implementation.

## Lifecycle

Record the current state in the proposal's `Meta` section:

1. **Draft** — The author is developing the proposal and gathering early
   feedback. Major changes are expected.
2. **Proposed** — The proposal is ready for broad review. Important design and
   compatibility questions should be resolved before acceptance.
3. **Accepted** — The proposal's pull request has received approval from at
   least two maintainers and completed the Final Comment Period (FCP). The FCP
   lasts seven days unless all maintainers agree to close it early; acceptance
   requires a majority of binding maintainer votes. A maintainer records the
   outcome by updating `Meta.Status` and linking the proposal pull request or
   decision record. Acceptance guides implementation but does not guarantee a
   release or exact delivery date.
4. **Implementing** — Implementation is in progress. Link implementation pull
   requests from the proposal.
5. **Implemented** — The feature has shipped. Record the release or final
   implementation pull request when practical.
6. **Rejected** — Maintainers decided not to pursue the design. Preserve the
   proposal and rationale for future reference.
7. **Withdrawn** — The author or maintainers stopped pursuing the proposal
   before a final decision.
8. **Superseded** — Another proposal replaces this one. Link both proposals in
   their metadata.

A proposal is a design record intended to guide implementation and review. It
is not the canonical user or developer documentation, and it may not be
maintained after implementation. Once a feature ships, the code, API
documentation, tests, and user documentation define current behavior. Avoid
editing an implemented proposal merely to track routine implementation drift;
write a new proposal when a later change introduces a substantial new design
or reverses an important decision.

## Review and maintenance

- Keep one proposal per Markdown file, using a descriptive kebab-case name.
- Include concrete examples and identify compatibility and security effects.
- Link related issues, pull requests, and superseded proposals.
- Before requesting acceptance, resolve material open questions and include
  implementation evidence or a proof plan for compatibility, security, and
  operational behavior.
- Resolve material open questions before moving to **Accepted**; questions
  explicitly deferred to implementation should be identified as such.
- Update the proposal during implementation when an accepted architectural or
  API decision changes materially.
- Do not treat proposal approval as a substitute for API review, tests,
  documentation, release notes, or the normal pull request process.

### Acceptance procedure

1. Open a pull request containing the proposal and invite community review.
2. Incorporate feedback and obtain at least two maintainer approvals.
3. Mark the pull request as entering the seven-day FCP. Maintainers may end
   the FCP early only with unanimous maintainer agreement.
4. Record binding maintainer votes. A majority in favor accepts the proposal;
   an absent vote from a maintainer with binding voting rights counts as
   affirmative, as in the existing KDP process. Substantial new arguments
   return the proposal to **Proposed** for further development.
5. After the FCP outcome, update the proposal status and link the pull request
   or decision record that contains the approvals, votes, and evidence.
