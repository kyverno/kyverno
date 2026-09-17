# Architectural Plan: Retargeting Legacy-Policy PRs to `release-1.19`

Review of `PR_REBASE_INTENT.md` — grounded against the live state of `kyverno/kyverno` on 2026-09-13.

## 0. Executive summary (what the data says)

| Fact | Value | Implication |
|---|---|---|
| Open non-bot PRs on `main` | **339** (340 fetched − 1 Dependabot bot PR) | Manual triage is infeasible; needs tooling |
| Path-rule + content-tier classification (final, post-review-fixes) | 95 LEGACY_ONLY · 69 MIXED · 74 CEL_ONLY · 96 SHARED_ONLY · 5 REVIEW-MIGRATION | ~1/3 of PRs must move; MIXED and content-tier checks route ambiguous PRs to manual review |
| SHARED_ONLY PRs whose *diff text* still references legacy types | Folded into the counts above via the Tier-2 content scan (`--diff-scan`) | Path rules alone are insufficient; a 2nd content tier is required |
| `main` vs `release-1.19` divergence | 135 commits / 157 files | Base-branch flip alone is **not** enough — a rebase is mandatory |
| Rebase probe (40 LEGACY_ONLY PRs `git rebase --onto release-1.19`) | **38/40 clean** | Automated rebase is realistic; the 2 failures (#16988 and #16886) are real merge conflicts and are routed to `retarget/needs-author` |
| Fork PRs with `maintainerCanModify` | 100 % | Maintainers can push rebased branches directly — but this must be opt-in |
| PRs already `CONFLICTING` with `main` | 84 | Pre-existing debt; retargeting resolves many of these for free |
| Already-landed 1.20 work on `main` | #17535 (block legacy writes), #17544, #16868 | A false-positive class exists: "legacy-*gating*" PRs belong on `main` |

**Recommendation:** two-tier classifier (path → diff-content) + human-approved dry-run report, then a `/retarget release-1.19` slash-command bot that rebases via `git rebase --onto` and pushes to the fork (or opens a sibling PR when it cannot), plus a `pr-branch-guard` workflow for new PRs.

---

## 1. Repository structure analysis — legacy vs CEL vs shared

The CEL API types live in the external module `github.com/kyverno/api` (`policies.kyverno.io/v1alpha1|v1beta1`), so **`api/` in this repo is almost entirely legacy**. The import graph shows `pkg/engine/api`, `pkg/engine/jmespath`, `pkg/engine/mutate/patch` and `pkg/admissionpolicy` are consumed by CEL/report code and must be treated as shared.

### 1.1 LEGACY (kyverno.io ClusterPolicy / Policy / CleanupPolicy / UpdateRequest)

```
api/kyverno/v1/**                       api/kyverno/v1beta1/**   api/kyverno/v2beta1/**
api/kyverno/v2/cleanup_policy*          api/kyverno/v2/updaterequest_types.go
pkg/engine/**            (except shared sub-pkgs in §1.3)
pkg/autogen/**           pkg/policycache/**      pkg/controllers/policycache/**
pkg/background/**        (except gpol/, mpol/)
pkg/policy/**            (except gpol*.go, mpol*.go)
pkg/controllers/cleanup/**
pkg/controllers/admissionpolicygenerator/{cpol.go,generate-vap.go,vap.go}
pkg/admissionpolicy/kyvernopolicy_checker*
pkg/validation/policy/** pkg/validation/cleanuppolicy/**
pkg/webhooks/resource/{generation,imageverification,mutation,validation}/**
pkg/webhooks/resource/{updaterequest*,validation*}.go
pkg/webhooks/updaterequest/**   pkg/webhooks/policy/**
pkg/image/verifiers/cpol/**  pkg/cosign/**  pkg/notary/**  pkg/pss/**
cmd/cleanup-controller/**  cmd/background-controller/**
cmd/cli/kubectl-kyverno/commands/{migrate,fix}/**  cmd/cli/kubectl-kyverno/fix/**
config/crds/kyverno/kyverno.io_{clusterpolicies,policies,cleanuppolicies,clustercleanuppolicies,updaterequests}.yaml
charts/kyverno-policies/**
test/conformance/chainsaw/{validate,mutate,generate,cleanup,autogen,verify-images,verify-manifests,
  background-only,rangeoperators,generate-validating-admission-policy,generate-mutating-admission-policy*,
  deferred,force-failure-policy-ignore,policy-validation}/**
test/cli/{test,apply,test-generate,test-mutate,test-fail,test-cleanup-policy,test-context-apicall,
  test-context-configmap,test-exceptions,scenarios_to_cli,test-ruleless-policy,sample-policy-exclusion}/**
test/policy/**   test/fuzz/**
```

### 1.2 CEL (policies.kyverno.io *Policy types)

```
pkg/cel/**
pkg/background/{gpol,mpol}/**        pkg/policy/{gpol,mpol}*.go
pkg/webhooks/resource/{gpol,ivpol,mpol,vpol}/**   pkg/webhooks/celexception/**
pkg/controllers/deleting/**
pkg/controllers/policystatus/{gpol,ivpol,mpol,ngpol,nivpol,vpol}.go
pkg/controllers/admissionpolicygenerator/{vpol.go,mpol*.go}
pkg/controllers/webhook/mpol*.go
pkg/image/verifiers/ivpol/**  pkg/image/verification/**
config/crds/policies.kyverno.io/**   charts/kyverno/charts/crds/templates/policies.kyverno.io/**
test/conformance/chainsaw/{cel,validating-policies,mutating-policies,generating-policies,
  deleting-policies,image-validating-policies,namespaced-*}/**
test/cli/test-*-policy/**  test/cli/test-context-*-{vpol,mpol,gpol,dpol,ivpol}/**  test/cli/test-gpol-custom-crd/**
cmd/cli/kubectl-kyverno/processor/*{vpol,mpol,gpol,dpol,ivpol}*
```

### 1.3 SHARED (everything else, plus explicit overrides)

Explicit overrides that *look* legacy but are imported by CEL/reports (verified via import graph):

```
pkg/engine/api/**  pkg/engine/jmespath/**  pkg/engine/mutate/patch/**
pkg/engine/context/loaders/**  pkg/engine/factories/**  pkg/engine/adapters/**
pkg/admissionpolicy/**  (VAP/MAP builder used by both cpol and vpol)
```

Implicitly shared (no rule → SHARED): `cmd/cli/kubectl-kyverno/**` (apply/test), `pkg/controllers/{webhook,report,ttl,certmanager,globalcontext}`, `pkg/utils/**`, `pkg/config`, `pkg/webhooks/handlers`, `charts/kyverno/**`, `.github/**`, `Makefile`, `go.mod`, docs.

### 1.4 Two-tier classification (why paths alone fail)

Shared files (CLI `apply`/`test`, webhook controller, report controllers) contain hunks that reference `kyvernov1.ClusterPolicy`, `engineapi.*`, JMESPath, etc. Scanning the **diff text** of the 133 SHARED_ONLY PRs re-bucketed 46 of them:

| Signal in added/removed lines | Count | Effect |
|---|---|---|
| Legacy identifiers only (`kyvernov1.`, `ClusterPolicy`, `CleanupPolicy`, `UpdateRequest`, `engineapi.`, `jmespath`, `autogen.`, `policycache.`, `kyverno.io/v1`, `cpol`) | 19 | → **LEGACY_ONLY (content)** |
| CEL identifiers only (`policiesv1alpha1.`, `(Namespaced)?{Validating,Mutating,Generating,Deleting,ImageValidating}Policy`, `vpol|mpol|gpol|dpol|ivpol`, `policies.kyverno.io`) | 8 | → CEL_ONLY |
| Both | 19 | → MIXED (manual) |
| Neither | 87 | → SHARED_ONLY, keep on `main` |

Rule: **path tier decides first; content tier only refines PRs the path tier calls SHARED_ONLY.** A path-LEGACY_ONLY PR is never "rescued" by content.

### 1.5 Known false-positive class: 1.20 migration-grace work

PRs that *touch legacy CRDs/paths in order to gate, warn, block or migrate them* (#17554, #17553, #17534, #17519, #17494) belong on `main`. Detect via: title/body matches `/legacy|migrat|deprecat|1\.20/i` **and** PR touches `pkg/deprecations/**`, `charts/kyverno/**`, or `cmd/cli/**/legacypolicies`. Flag as `REVIEW-MIGRATION`, never auto-retarget. (The 40-PR sample of `LEGACY_ONLY` PRs yielded 38 clean rebases and 2 merge conflicts, #16988 and #16886, with migration PRs correctly held on `main`).

---

## 2. Dry-run plan for existing PRs

### 2.1 Tooling: `hack/pr-triage/pr-branch-triage.py` (prototype delivered, read-only)

Only `gh pr list` and `gh api GET /repos/…/pulls/N/files` and `gh pr diff` are used. **No mutation calls exist in the script.**

```
python3 pr-branch-triage.py --repo kyverno/kyverno --base main --skip-bots \
    --out pr-triage-report.md --json pr-triage-report.json
python3 pr-branch-triage.py --reclassify pr-triage-report.json    # offline re-run after rule edits
python3 pr-branch-triage.py --pr 17565 --repo kyverno/kyverno     # single-PR classification
```

Pipeline per PR:
1. Metadata: `number,title,author,isDraft,isCrossRepository,maintainerCanModify,mergeable,labels,headRepositoryOwner,headRefName`.
2. File list (paginated).
3. Tier 1 path classification (first-match-wins over SHARED-override → CEL → LEGACY → default SHARED).
4. Tier 2 diff-content scan **only if** Tier 1 = SHARED_ONLY or LEGACY_ONLY.
5. Migration-work heuristic (§1.5) → `REVIEW-MIGRATION` flag.
6. Optional `--probe-rebase`: in a throwaway `git worktree`, `git fetch upstream +pull/N/head:refs/triage/N`; `git rebase --onto release-1.19 $(git merge-base main HEAD)`; record OK/CONFLICT + conflicting files; `git rebase --abort`; remove worktree. Purely local — nothing pushed.

> Use `git rebase --onto`, **not** `cherry-pick A..B`: 40 % of PR branches contain `Merge branch 'main'` commits which cherry-pick refuses; rebase linearizes them. (Probe: cherry-pick 13/25 clean → rebase 38/40 clean.)

### 2.2 Classification criteria

| Category | Rule | Proposed action |
|---|---|---|
| **LEGACY_ONLY** | ≥1 LEGACY file (path), 0 CEL files (path or content) | Retarget → `release-1.19` (rebase) |
| **CEL_ONLY** | ≥1 CEL file (path), 0 LEGACY (path or content) — includes SHARED_ONLY promoted by a clean CEL-only content signal | Keep on `main` |
| **MIXED** | ≥1 LEGACY **and** ≥1 CEL (path or content) — includes SHARED_ONLY demoted by a legacy-touching content signal | Manual: split or keep on `main` |
| **SHARED_ONLY** | 0 LEGACY, 0 CEL, neutral content | Keep on `main`; re-check after removal PR lands |
| **REVIEW-MIGRATION** (flag) | §1.5 heuristic | Keep on `main`; human confirms |
| **STALE** (flag) | `updatedAt` > 90 d **or** `CONFLICTING` + no author activity 60 d | Comment + `needs-rebase`; close after 30 d |

**Content-tier safety invariant:** the Tier-2 diff-content scan (§2.1 step 4) only ever runs on Tier-1 `SHARED_ONLY` rows, and can only move a row in two directions: promote it to `CEL_ONLY` (safe — same "keep on main" action as before) or demote it to `MIXED` (forces manual review). Content signals alone can **never** produce an auto-eligible `LEGACY_ONLY` — only Tier-1 path matches (or an explicit human `override: RETARGET`) can make a PR eligible for the automated retarget gate in §2.4. This is what makes the mitigation in §5 ("never auto-move on content alone without path support") true by construction, not just by convention.

### 2.3 Report format

Header summary, then one table per category (already produced in `pr-triage-report.md`):

```
| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Proposed label | Proposed Action |
| #17565 | @ANAMASGARD | perf: reduce engine context checkpoint copy cost | | Y | Y | MERGEABLE | 9/0/0 | `type_legacy` | Retarget -> release-1.19 |
```

- `Files (L/C/S)` = counts of legacy / CEL / shared files — lets a reviewer eyeball borderline cases.
- `Proposed label` = the `type_*` category label described in §2.5, computed by `classify_pr()` but **not yet applied to any PR**.
- MIXED section includes a `<details>` block listing the legacy vs CEL file sets per PR (splitting guide input).
- Machine-readable JSON sidecar drives the execution phase, so **execution uses exactly the approved list**, not a re-classification.

### 2.4 Dry-run approval gate

1. Commit `pr-triage-report.md` to a tracking issue ("Legacy PR retargeting — batch 1").
2. Maintainers annotate overrides directly in the JSON (`"override": "KEEP_MAIN" | "RETARGET" | "SPLIT"`), e.g. confirming the 5 REVIEW-MIGRATION PRs → `KEEP_MAIN`. If a maintainer sets `"override": "RETARGET"` on a `MIXED` or `REVIEW-MIGRATION` PR, re-running `--probe-rebase` will evaluate that PR and allow it to proceed through the gate.
3. Only PRs with `(category=LEGACY_ONLY || override=RETARGET) && override!=KEEP_MAIN && probe=OK` proceed to automated execution; everything else is manual.

### 2.5 Category labels — `type_legacy` / `type_cel` / `type_mixed` / `type_shared`

As part of the dry run, every classified PR is now tagged in the report/JSON with a **proposed** category label (one-to-one with `classify_pr()`'s output). These four labels are visible triage state — separate from the workflow-state labels in §4.1 (`retarget/*`, `target/*`) which only apply during/after execution.

| Category | Label | Color (suggested) | Description | Applied to |
|---|---|---|---|---|
| LEGACY_ONLY | `type_legacy` | `#b60205` (red) | PR only touches legacy `kyverno.io` ClusterPolicy/Policy/CleanupPolicy code — candidate to retarget to `release-1.19` | 95 PRs |
| CEL_ONLY | `type_cel` | `#00BCD4` (cyan) | PR only touches `policies.kyverno.io` CEL policy code — stays on `main` | 74 PRs |
| MIXED | `type_mixed` | `#FBCA04` (yellow) | PR touches both legacy and CEL code — needs manual split/keep decision | 69 PRs |
| SHARED_ONLY | `type_shared` | `#c5def5` (light blue) | PR touches neither (shared infra/docs/deps/CI) — stays on `main` | 96 PRs |

*(Note: The 5 `REVIEW-MIGRATION` PRs are left unlabeled in the automated labeling step to require explicit human triage.)*

**Definition of done for this step:** labels exist in `.github/labels.yml` (added alongside the §4.1 `legacy-policy`/`cel-policy` glob-based labels — the `type_*` labels are the *triage-result* labels applied by the script below, `legacy-policy`/`cel-policy` are the *ongoing* glob-based labeler labels applied automatically to every future PR) and are backfilled onto the 334 classified PRs in the four labeled categories (out of 339 analyzed non-bot PRs).

**Tooling — `hack/pr-triage/apply-labels.sh` (delivered, dry-run by default):**

```
# Dry run — prints the exact `gh pr edit` commands, changes nothing:
./apply-labels.sh --json pr-triage-report.json

# Dry run for one category only:
./apply-labels.sh --json pr-triage-report.json --category LEGACY_ONLY

# Only after maintainer sign-off on the dry-run output:
./apply-labels.sh --json pr-triage-report.json --execute
```

Behavior:
- Reads `proposed_label` per PR from the triage JSON (never re-derives it — keeps labeling consistent with whatever report was approved).
- Skips PRs that already carry the correct `type_*` label (idempotent — safe to re-run).
- If a PR carries a *different* stale `type_*` label (e.g. reclassified after a push), removes it and adds the new one so exactly one `type_*` label is present at a time.
- **Default mode performs zero writes** — `--execute` is required to call `gh pr edit`. This satisfies the "dry run first, no changes yet" requirement: the labeling step itself is reviewed the same way as the retargeting step (§2.4).
- Rate-limited (`sleep 1` between calls) when `--execute` is used on a large batch.

**Sequencing:** applying `type_*` labels is a prerequisite, low-risk precursor to §3 (retargeting) — it makes the 340-PR backlog visually triageable on the PR list/board *before* any branch/base changes happen, and gives maintainers a lightweight way to override classification (relabeling a PR is equivalent to setting `override` in the JSON, and `apply-labels.sh`/the guard in §4 should be updated to prefer an existing GitHub label over the computed one, so a maintainer's manual relabel sticks across re-runs).

---

## 3. Execution strategy (post-approval)

### 3.1 Why a base flip alone is wrong

Changing `base` from `main` to `release-1.19` on a branch forked from `main` makes GitHub show the 135 `main`-only commits in the PR diff and turns CI red. **The head branch must be rebased onto `release-1.19` first**, then the base is switched (same order the cherry-pick bot uses).

### 3.2 Per-PR workflow (automatable; `hack/pr-triage/retarget.sh <PR>`)

```
1. Pre-flight
   gh pr view N --json baseRefName,headRefName,headRepositoryOwner,maintainerCanModify,isDraft,state
   abort if state!=OPEN or base!=main or override==KEEP_MAIN
2. Announce (idempotent — skip if marker comment exists)
   gh pr comment N --body-file templates/retarget-notice.md      # see §3.4
   gh pr edit N --add-label "retarget/release-1.19"
3. Rebase locally (throwaway worktree)
   git fetch --no-write-fetch-head upstream +pull/N/head:refs/triage/pr-N
   base=$(git merge-base upstream/main refs/triage/pr-N)
   git worktree add -d .wt-N refs/triage/pr-N
   git -C .wt-N rebase --onto upstream/release-1.19 $base   || { record CONFLICT; label "retarget/needs-author"; comment; abort }
4. Verify
   (cd .wt-N && make fmt-check imports-check && go build ./... && go test ./<changed pkgs>)   # cheap gate; CI does the rest
5. Publish rebased head — strategy A (preferred) or B
   # IMPORTANT: the rebased commits only exist in the .wt-N worktree, not in
   # the main checkout. Every push below MUST run with `git -C .wt-N ...` (or
   # `cd .wt-N` first) so `HEAD` resolves to the rebased tip, not whatever the
   # main working tree happens to have checked out.
   newHeadSha=$(git -C .wt-N rev-parse HEAD)
   A. maintainerCanModify==true AND explicit `/retarget release-1.19` author consent:
      git -C .wt-N push --force-with-lease="${headRefName}:${oldHeadSha}" https://github.com/<owner>/kyverno.git HEAD:<headRefName>
      then: gh pr edit N --base release-1.19
   B. else (no consent / 72 h silence / maintainerCanModify==false):
      git -C .wt-N push upstream HEAD:retarget/N-<headRefName>
      gh pr create --base release-1.19 --head retarget/N-<headRefName> \
         --title "$(gh pr view N --json title -q .title)" \
         --body "Retargeted from #N on behalf of @author. Co-authored-by preserved. Closes #N when merged."
      gh pr comment N --body "Superseded by #M (retargeted to release-1.19)"; gh pr edit N --add-label "superseded"
      (close #N only after #M merges — keeps the review thread alive)
6. Record & Update
   gh pr edit N --remove-label "retarget/release-1.19" --add-label "target/release-1.19"
   record {pr, oldHeadSha, newHeadSha, oldBase, newBase, strategy} → hack/pr-triage/executed.json   (rollback ledger; recorded immediately after mutation)
7. Cleanup (best-effort)
   git worktree remove -f .wt-N || true
   git update-ref -d refs/triage/pr-N || true
```

Rate-limit: batch ≤ 25 PRs/hour; the repo already has `pr-rate-limiter.yaml` — run the retarget bot with the `PR_UPDATER_APP` GitHub App (same identity as `cherry-pick-on-merge.yaml`) so pushes carry a bot identity and DCO stays intact (`rebase` preserves author + `Signed-off-by`).

**Strongly prefer A but make force-push opt-in per author**: the notice comment (§3.4) gives authors 72 h to (a) rebase themselves, (b) reply `/retarget release-1.19` to consent to the bot push, or (c) `/keep-main` to contest. Silence after 72 h → strategy B (sibling PR), never an un-consented force-push to someone's fork.

### 3.3 MIXED PRs — splitting guide (69 PRs)

| Situation | Guidance |
|---|---|
| Legacy hunks are incidental (rename, ctx propagation, import churn) — e.g. #17551, #15064 | Drop the legacy hunks, keep on `main`. The legacy files vanish from `main` anyway. |
| Feature spans both engines — e.g. #17194 (KMS verifier cache: cpol + ivpol), #16213 | **Split**: `git rebase -i` → two branches. Legacy branch → `release-1.19`; CEL branch → `main`. Provide `hack/pr-triage/split-helper.sh <PR> <legacy-globs>` that does `git checkout -b <pr>-legacy && git checkout upstream/release-1.19 -- <non-legacy files>` style filtering per file set. |
| Shared-package edits with both flavours (e.g. `pkg/admissionpolicy`, CLI) — #17510, #17335, #17478 | Keep on `main`; open a follow-up cherry-pick to `release-1.19` after merge using the existing `/cherry-pick release-1.19` command. |
| Large refactor / test infra — #15508 (314/132/140 files), #15866 | Keep on `main`; author decides whether legacy tests are worth back-porting. |

Default for MIXED when in doubt: **keep on `main`, cherry-pick the legacy part afterwards** (lowest coordination cost, uses existing automation).

### 3.4 Author communication

- **Labels** (add to `.github/labels.yml`): `target/release-1.19`, `target/main`, `retarget/proposed`, `retarget/needs-author`, `retarget/superseded`, `legacy-policy`, `cel-policy`.
- **Comment template** (`retarget-notice.md`):
  > 👋 Kyverno **v1.19 is the last release supporting `kyverno.io` ClusterPolicy/Policy/CleanupPolicy**. Legacy policy code is being removed from `main` (#\<removal-PR\>), so this PR — which only touches legacy code (`pkg/engine/…`, …) — needs to target **`release-1.19`**.
  > **Options (please pick one within 72 h):**
  > 1. Rebase yourself: `git rebase --onto upstream/release-1.19 $(git merge-base upstream/main HEAD)` then push; we'll switch the base.
  > 2. Reply `/retarget release-1.19` and a maintainer bot will rebase + force-push your branch and switch the base for you.
  > 3. Reply `/keep-main` if you believe this is mis-classified (e.g. 1.20 migration work).
  > If we don't hear back, we'll open a sibling PR against `release-1.19` preserving your authorship and link it here. Details: `docs/dev/legacy-policy-retirement.md`.
- **Docs**: add `docs/dev/legacy-policy-retirement.md` (branch policy table, path rules, how to split), link from `CONTRIBUTING.md` and `.github/PULL_REQUEST_TEMPLATE.md` (add a checkbox: "Targets the right branch — see legacy policy retirement guide").
- **Tracking issue** with the dry-run table, updated as PRs move (checkbox per PR).

### 3.5 Ordering

1. Create labels, docs, PR template update, and the `pr-branch-guard` workflow (§4) **before** any retarget so new PRs don't regress the queue.
2. Run dry run → approval.
3. Retarget LEGACY_ONLY in batches (newest/most-active first — highest chance of author response).
4. Land the legacy-removal PR on `main` **after** batch 1 completes (removal PR itself gets a `REVIEW-MIGRATION` exemption).
5. Re-run the triage against the *post-removal* `main`: any SHARED_ONLY PR that is now `CONFLICTING` gets the same notice.
6. MIXED / STALE handled manually in parallel.

---

## 4. Automation for new PRs — `pr-branch-guard`

Reuse the existing `labels.yml → actions/labeler` pipeline (it already renders globs) plus a small guard job.

### 4.1 Labels (in `.github/labels.yml`, so `pr-labelling.yaml` applies them automatically)

These `legacy-policy`/`cel-policy` labels are the **ongoing, glob-based** labels applied automatically to every PR (present and future) by the labeler action. They are distinct from — and complementary to — the four **triage-result** `type_legacy`/`type_cel`/`type_mixed`/`type_shared` labels from §2.5, which are computed once by `pr-branch-triage.py`, backfilled onto the current 340-PR backlog via `apply-labels.sh`, and represent the classifier's category verdict (including the two-tier content-scan refinement and the MIXED bucket, neither of which a pure glob-based labeler can express). New PRs get both: the glob labeler tags file-level touch points immediately on open, while `pr-branch-guard` (§4.2) additionally applies/updates the `type_*` label so the backlog stays consistently labeled going forward.

```yaml
legacy-policy:
  color: b60205
  description: Touches kyverno.io ClusterPolicy/Policy/CleanupPolicy code (release-1.19 only)
  rules:
    - changed-files:
        - any-glob-to-any-file: [ <LEGACY globs from §1.1> ]
cel-policy:
  color: 00BCD4
  description: Touches policies.kyverno.io CEL policy code
  rules:
    - changed-files:
        - any-glob-to-any-file: [ <CEL globs from §1.2> ]
```

### 4.2 Workflow `.github/workflows/pr-branch-guard.yaml`

```yaml
on:
  pull_request_target: { types: [opened, synchronize, reopened, edited] }   # 'edited' catches base changes
permissions: { contents: read, pull-requests: write, checks: write }
jobs:
  guard:
    steps:
      - checkout (base ref only — never execute PR code)
      - run: python3 hack/pr-triage/pr-branch-triage.py --pr ${{ github.event.pull_request.number }} --diff-scan --json out.json
      - github-script:
          const r = JSON.parse(fs.readFileSync('out.json'))[0];
          const base = context.payload.pull_request.base.ref;
          const exempt = labels.includes('migration/1.20') || body.includes('/keep-main');
          if (r.category === 'LEGACY_ONLY' && base === 'main' && !exempt)  → fail check + sticky comment (template §3.4)
          if (r.category === 'CEL_ONLY'    && base.startsWith('release-1.19')) → fail check ("CEL changes go to main; release-1.19 receives cherry-picks only")
          if (r.category === 'MIXED')      → neutral check + comment linking splitting guide
          else → pass
```

- Enforcement: make the `pr-branch-guard` check **required** on `main` via branch protection/ruleset; keep it *advisory* (neutral) on `release-1.19` for the first 2 weeks.
- Escape hatch: label `migration/1.20` (maintainer-only, via `labels.yml` permission convention) or `/keep-main` comment → skip. All skips are logged in the check summary.
- Single source of truth: the same `RULES` list is consumed by the triage script, the guard, and (rendered) by `labels.yml` — add a `make verify-pr-rules` codegen check that regenerates the two label blocks from the Python rules so they can't drift.
- Post-removal hardening (v1.20+): once legacy code is gone from `main`, the guard on `main` can additionally fail on *any* file under the LEGACY globs except `config/crds/kyverno/*` (retained CRDs) and `pkg/deprecations/**`.

### 4.3 `release-1.19` branch hygiene

- Rulesets: require the same CI as `main` (unit, CLI tests, conformance for legacy suites), DCO, and linear history.
- Reverse-guard: PRs to `release-1.19` that add **new features** (conventional-commit `feat:`) get a warning comment — patch branches take fixes; features only when maintainers explicitly decide 1.19.x is a feature-receiving LTS line (this should be an explicit decision recorded in the docs, since 95 open PRs in LEGACY_ONLY include `feat:` items such as #17494, #17519).

---

## 5. Risk mitigation & rollback

| Risk / edge case | Mitigation |
|---|---|
| Rebase conflict (probe: ~5 %, 2/40; will rise for stale PRs) | Bot never resolves conflicts. Label `retarget/needs-author`, comment with conflicting files and the exact `git rebase --onto` command. |
| Force-push overwrites author's un-pushed work | Only push with `--force-with-lease=<headRefName>:<oldHeadSha>` recorded at pre-flight; only after `/retarget` explicit consent **and** `maintainerCanModify`; silence/no-consent routes to Strategy B (sibling PR); ledger keeps `oldHeadSha` for restore. |
| Author lacks/revokes `maintainerCanModify` (today 0 %, but can change) | Strategy B (sibling PR from an upstream `retarget/*` branch). Keep original open until sibling merges. |
| Draft PRs (8 LEGACY_ONLY) | Comment + label only; never rebase drafts automatically — the author is still iterating. |
| Dependabot/renovate PRs | Skipped entirely (`--skip-bots`); dependency bumps follow `main`. |
| Misclassification: 1.20 migration PRs (#17554, #17553, #17534, #17519, #17494) | `REVIEW-MIGRATION` flag; require human override; `/keep-main` escape hatch; guard exempts `migration/1.20` label. |
| Misclassification: shared file with legacy-only hunks (19 found) | Tier-2 content scan; borderline → MIXED (manual), never auto-move on content alone without path support unless `override: RETARGET`. |
| `release-1.19` CI differs from `main` (workflows evolved) | Before batch 1, cherry-pick CI-only fixes (`.github/workflows/**`) to `release-1.19` so retargeted PRs get green signals; verify with one canary PR (#17525: 1 commit, 4 legacy files, probe OK). |
| Review history/approvals lost on sibling PR (strategy B) | Sibling body links original; reviewers re-approve; `superseded` label; original closed only after sibling merges. |
| Author never responds → PR rots | STALE policy: 30 d after notice → `needs-rebase` + reminder; 60 d → close with "reopen anytime" comment. Bot-opened sibling PRs are owned by the triaging maintainer. |
| Removal PR lands before all LEGACY_ONLY PRs move | Retargeting is independent of removal (it depends only on `release-1.19`). Post-removal, re-run triage; newly `CONFLICTING` PRs get the same treatment. No ordering hazard other than more noise. |
| GitHub API rate limits / partial batch | Idempotent steps keyed on marker comment + labels; ledger allows resume; ≤25 PRs/hour. |
| Wrong retarget (PR should have stayed on `main`) | **Rollback per PR** from ledger: `gh pr edit N --base main`; `git push --force-with-lease=<ref>:<newHeadSha> <oldHeadSha>:<headRefName>` (strategy A) or close sibling + remove `superseded` (strategy B). The original head SHA is always retained in the ledger and in GitHub's PR timeline ("force-pushed from …"). |
| Rules drift between triage script, guard, and `labels.yml` | Single Python `RULES` source + `make verify-pr-rules` codegen check in CI. |

### Rollback plan (batch level)

1. Freeze: disable `pr-branch-guard` enforcement (ruleset toggle), stop the retarget bot.
2. For each ledger entry (`hack/pr-triage/executed.json`) in reverse order: restore base, restore head SHA (force-with-lease against the bot's SHA so any author pushes in between are not clobbered), remove labels, post a short "reverted retarget" comment.
3. The ledger, dry-run JSON and probe outputs are committed to the tracking issue so the operation is fully reproducible/auditable.

---

## 6. Deliverables checklist

- [x] `pr-branch-triage.py` — read-only classifier + Markdown/JSON report, emitting a `proposed_label` (`type_legacy`/`type_cel`/`type_mixed`/`type_shared`) per PR (`hack/pr-triage/pr-branch-triage.py`). Single-PR mode (`--pr`), Tier-2 diff-content scan (`--diff-scan`), and the migration heuristic (`REVIEW-MIGRATION`) are implemented directly inside this script.
- [x] `--probe-rebase` (git-worktree based, per-PR-scoped local refs, `--no-write-fetch-head`, always cleans up and never pushes) implemented inside `pr-branch-triage.py`; sample runs: 8/8 clean, then 38/40 clean on a 40-PR sample (with conflict files captured).
- [x] Live dry-run report for 339 analyzed PRs (`hack/pr-triage/pr-triage-report.md/json`): 95 LEGACY_ONLY, 69 MIXED, 74 CEL_ONLY, 96 SHARED_ONLY, 5 REVIEW-MIGRATION (1 bot PR excluded).
- [x] `hack/pr-triage/apply-labels.sh` — applies the 4 `type_*` labels; **dry-run by default (prints `gh label create`/`gh pr edit` commands only), requires `--execute` to mutate anything**; bootstraps missing labels, queries live labels from GitHub (with failure handling), and preserves manual maintainer labels unless `--force` is given. Verified with live dry-run tests.
- [ ] Maintainer review/approval of the `type_*` dry-run output, then `apply-labels.sh --execute` to backfill labels on the 334 open PRs in the four labeled categories (not yet run — awaiting approval)
- [ ] `hack/pr-triage/retarget.sh` + `split-helper.sh` (execution; only after approval)
- [ ] `.github/labels.yml` additions (both the glob-based `legacy-policy`/`cel-policy` labels and the four `type_*` triage labels); `.github/workflows/pr-branch-guard.yaml`
- [ ] `docs/dev/legacy-policy-retirement.md`; `CONTRIBUTING.md` + PR template updates
- [ ] Tracking issue with the approved report and per-PR checkboxes
