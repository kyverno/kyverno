# PSS upstream sync

The Pod Security Standards (PSS) `ValidatingPolicy` templates in `charts/kyverno-policies` follow the upstream
Kubernetes Pod Security Admission (PSA) checks. This page explains how Kyverno detects upstream PSA changes,
how the optional bot PR works, and what to do when a change needs work by hand.

## How it works

| Part | File | What it does |
|---|---|---|
| Pin | `scripts/pss-upstream/upstream.yaml` | The upstream PSA repository, branch, and commit that the chart is verified against |
| Fixture check | `scripts/verify-policies-pss-fixtures.py` | Renders the chart and checks it against the upstream PSA pass and fail fixtures at the pinned commit. Runs in the `CLI` workflow. |
| Detector | `scripts/sync-pss-upstream.py` | Compares the pin with the head of the pinned branch, classifies the change, runs the fixture check against the new commit, and writes a report |
| Publisher | `scripts/publish-pss-upstream.py` | Opens or updates one bot PR that moves the pin. Off by default. |
| Workflow | `.github/workflows/sync-pss-upstream.yaml` | Runs the detector weekly, on demand, and on pull requests that change these files. Runs the publisher when publishing is on. |

The detector classifies each upstream change:

- **`none`:** no non-test file in `policy/` and no fixture in `test/testdata/` changed.
- **`fixtures-only`:** only fixtures changed, for example a new `v1.<N>` directory.
- **`manual`:** a Go check in `policy/` changed, so the CEL policies may need changes by hand.

The detector exits with `0` for no change, `2` for a change whose new commit passes the fixture check, `3` for
a change whose new commit fails the fixture check, and `1` for an error. The workflow run is red for `3` and
`1`. On a scheduled run, a red result opens a workflow-failure issue.

## Handle a bot PR

When publishing is on, the bot keeps a single PR open from the `bot/pss-upstream-sync` branch.

- **A ready PR** means the change is `fixtures-only`, and the new commit passes the fixture check. Review the
  report in the PR description, and check that the `CLI` workflow's fixture check passes. Then merge it as
  usual.
- **A draft PR with the `needs-manual-cel` label** means the change is `manual`, or the new commit fails the
  fixture check. To finish it:
  1. Read the Go diff in the PR description, and decide what the chart needs.
  2. Push your CEL changes to `bot/pss-upstream-sync`. After you push, the bot stops updating the branch,
     posts one comment, and leaves your work alone.
  3. Add a CLI test under `test/cli/test-validating-policy/pss-*`, and a `semverCompare` gate if the change
     starts at a new Kubernetes version. For a new upstream check, update `scripts/pss-upstream/mapping.yaml`.
  4. When the fixture check passes, mark the PR ready for review yourself.

The bot never merges, approves, or closes a PR, and never marks a draft ready.

## Enable publishing

Publishing is off until a maintainer sets it up. It needs these four things:

1. **A GitHub App** installed on the repository, with these repository permissions:
   - **Contents:** Read and write, to push `bot/pss-upstream-sync`.
   - **Pull requests:** Read and write, to open and update the PR.
   - **Issues:** Read and write, to create the `needs-manual-cel` label and post the pause comment.
   - **Metadata:** Read. Every App has this permission.

   A token from an App is required, not the workflow's `GITHUB_TOKEN`. GitHub doesn't run workflows on a PR
   that `GITHUB_TOKEN` opens, so the fixture check wouldn't run on the bot PR.
2. **Two repository secrets:** `PSS_UPSTREAM_APP_CLIENT_ID` (the App's client ID) and
   `PSS_UPSTREAM_APP_PRIVATE_KEY` (a private key for the App, as PEM text or base64-encoded PEM).
3. **The repository variable** `PSS_UPSTREAM_PUBLISH` set to `true`.
4. **The workflow on the default branch.** Publishing runs only on `schedule` and `workflow_dispatch` runs of
   the default branch.

The bot commits as `<app-slug>[bot]`, with a `Signed-off-by` trailer for the DCO check. The PR title follows
`.github/semantic.yml`: `chore(pss): sync with upstream Pod Security Admission <commit>`.

## Pause publishing

Use either of these:

- Set the repository variable `PSS_UPSTREAM_PUBLISH` to `false`. The workflow keeps detecting changes and runs
  the publisher as a dry run.
- Disable the workflow: `gh workflow disable sync-pss-upstream.yaml`.

## Move the pin by hand

1. Run the detector, which writes the new pin as `upstream.yaml` in the output directory:

   ```bash
   python3 scripts/sync-pss-upstream.py --out /tmp/pss-sync --preview --cli cmd/cli/kubectl-kyverno/kubectl-kyverno
   ```

2. Copy the `repository`, `branch`, and `commit` values from `/tmp/pss-sync/upstream.yaml` into
   `scripts/pss-upstream/upstream.yaml`.
3. Run `make verify-policies-pss-fixtures`, and open a PR.

## Run the scripts locally

These commands need Python 3 with PyYAML, and Git. The fixture check also needs Helm and a built CLI
(`make build-cli`).

```bash
python3 scripts/test-sync-pss-upstream.py
python3 scripts/test-publish-pss-upstream.py
python3 scripts/sync-pss-upstream.py --out /tmp/pss-sync
python3 scripts/publish-pss-upstream.py --dry-run --result /tmp/pss-sync --repo OWNER/NAME
```

In the last command, replace `OWNER/NAME` with the repository to plan against. A dry run reads the branch
and PR state, prints the planned actions, and writes nothing. It needs a `GH_TOKEN` that can read the
repository, and a bot identity in `BOT_NAME` and `BOT_EMAIL`, or `APP_SLUG`.
