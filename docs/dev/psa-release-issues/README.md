# PSA release issues

The `PSA release issues` workflow (`.github/workflows/psa-release-issues.yaml`) opens a GitHub issue for each
Kubernetes Pod Security Admission (PSA) minor release whose upstream changes the Pod Security Standards (PSS)
`ValidatingPolicy` templates in `charts/kyverno-policies` may need. The logic is in
`scripts/psa-release-issues.py`, the baseline and the check-to-policy map are in `scripts/psa-release-issues.yaml`,
and the tests are in `scripts/test-psa-release-issues.py`.

## How it works

PSA is developed in `kubernetes/kubernetes` under `staging/src/k8s.io/pod-security-admission`. The Kubernetes
publishing bot mirrors that directory to the read-only `kubernetes/pod-security-admission` repository, and tags
each release there as `kubernetes-1.<minor>.<patch>` and `v0.<minor>.<patch>` on the same commit. The script
fetches the `kubernetes-1.*` tags into a blobless clone, and diffs two tags to get the change to the staging
directory.

GitHub can't start a workflow from a release in another owner's repository, so the workflow runs quarterly, on
January 1, April 1, July 1, and October 1. Kubernetes ships a minor release about every 4 months, so an issue opens
1 to 13 weeks after the release. Kyverno doesn't need it sooner: a PSS change ships in a later Kyverno release.
GitHub can delay scheduled runs, and disables them in a public repository after 60 days without activity; run the
workflow manually to check sooner.

Only minor releases are checked, and each one is compared with the previous minor: `v1.38.0` with `v1.37.0`.
Patches and pre-releases, such as `v1.37.2` or `v1.38.0-rc.0`, are skipped. A PSA fix that a patch backports
also lands in the next minor, so the minor comparison includes it. The trees are compared, not the history,
because release branches diverge from `master`.

Every change in `policy/`, `api/`, or `test/` needs an issue, except fixtures that a new
`test/testdata/<level>/v1.<N>/` directory copies unchanged from `v1.<N - 1>`. Only a release with no other change
is reported as **no action needed**.

The script maps each changed `policy/check_*.go` file or fixture to its check ID, and through `checks` to a
policy. From the policy, it finds the `*.cel.yaml` template, the check-specific helpers in `_helpers.tpl`, and the
`test/cli/test-validating-policy/pss-*` tests. From the Go diff, it reads new `MajorMinorVersion(1, N)` gates and
changed string values, and checks whether the template already has them. These signals are string matches, not a
semantic analysis, so read the comparison before you decide. The issue doesn't include the diff: it links the
comparison, and each changed Go file links to its section of it.

When the template already has every added value and gate, and doesn't keep a value that upstream removed, the
issue asks a person to confirm the change. When it has only some of them, the issue lists what's already there and
asks only for the rest.

### Issue kinds

| Kind | Title ends with | `needs-manual-review` | When |
|---|---|---|---|
| Update required | `update the PSS CEL policies` | No | A mapped template lacks an upstream value or version gate, or only the upstream fixtures for its check changed |
| Confirmation required | `confirm the PSS CEL policies need no change` | No | Only test code changed, or the templates already seem to handle a check change. **Validation** then asks to check the upstream fixtures for that check. |
| Manual review required | `review the PSS CEL policies (manual review)` | Yes | A change can't be mapped or analyzed with confidence |

These changes need manual review, and appear under **Not mapped**:

- Shared `policy/` code, such as `helpers.go`, or base fixtures, which can affect every check.
- A change in `api/`.
- An upstream check ID that isn't in `checks`, or an entry in `checks` that upstream no longer declares. The
  script compares them whenever a release changes a check file.
- A check file that was renamed or split, or a mapped policy without a template.
- A check code change in which the script finds no version gate or value change. A code diff alone doesn't show
  that behavior changed, so the issue doesn't ask for an update.

### Issue structure

Every issue has the same sections, in this order:

| Section | Contents |
|---|---|
| Summary | The release, the issue type, and the recommended action |
| Release details | The release and the previous minor, the publish time in UTC, and the source and comparison links |
| What changed | What the upstream diff shows, split into code, fixtures, and test code, and the copied fixtures left out |
| Kyverno impact | Each mapped policy with its template, helpers, gates, and CLI tests, and the changes under **Not mapped**. "May already handle the change" is labeled as an inference: the script matches value names, but doesn't check under which version gate the template has them. |
| Required actions | A checklist for the issue type. A confirm issue asks only to confirm and record the outcome. |
| Validation | Checks for the actions, such as matching the upstream fixtures, `make test-cli-local-vpols`, `semverCompare` gates, and `helm template` with the chart's oldest and the new Kubernetes version. An issue lists only the checks that apply. No command runs the upstream fixtures against the PSS policies except `make verify-policies-sysctls`, which checks the `restrict-sysctls` allow-list at each version that `scripts/verify-policies-sysctls.sh` lists. A sysctls update therefore also asks for a line for the new version in that script. |
| Upstream evidence | The upstream commits and the changed files, with links to the comparison |

An issue generated from a PSA fork, rather than `kubernetes/pod-security-admission`, says so in its summary.

Upstream text can't ping anyone or link elsewhere: commit subjects are in code spans, the issue links commits
rather than pull requests, and no upstream code goes into the issue body.

### Release tracking and duplicates

The script stores no state. Each run checks every minor release after `since`, whatever its age, so a failed
or skipped run misses nothing.

Each issue ends with a hidden marker, such as `<!-- psa-cel-update: kubernetes-1.38.0 -->`, and the script skips
each release whose marker is in an open or closed issue with the `psa-cel-update` label. For concurrent runs:

1. The workflow runs one `Check releases` job at a time per repository, for every ref.
1. The script reads the issues again right before it creates one.
1. After creating one, it reads them once more, and closes its own issue as a duplicate if another run opened
   one first.

Don't remove the `psa-cel-update` label from these issues, and don't edit the marker.

## Enable the scheduled runs

Scheduled runs are off until a repository admin adds the repository variable `PSA_CEL_UPDATE_ISSUES` with the
value `true`, under **Settings** > **Secrets and variables** > **Actions** > **Variables**. Set it to `false` to
pause them.

The workflow uses the default `GITHUB_TOKEN` with `contents: read` and `issues: write`, and no secret. It creates
the `psa-cel-update`, `needs-manual-review`, and `area/pss` labels if they don't exist. When a scheduled run
fails, the `sync-issue` job opens a workflow-failure issue.

The repository must have Issues turned on. Every run, including a dry run, checks this first and fails with a
clear message when they're off. Forks have Issues off by default.

## Run the workflow manually

1. Go to **Actions** > **PSA release issues** > **Run workflow**.
1. Set the inputs:

   | Input | Default | Description |
   |---|---|---|
   | `dry-run` | `true` | Writes the issues to the job summary and an artifact instead of opening them |
   | `tag` | none | Checks only this minor release tag, such as `kubernetes-1.38.0` |
   | `since` | none | Overrides the baseline tag, a `kubernetes-1.<minor>.0` tag |
   | `repository` | none | For testing only: reads releases from this PSA fork, such as `https://github.com/OWNER/pod-security-admission`, instead of `kubernetes/pod-security-admission` |

1. Click **Run workflow**, and read the job summary.

Only a run of the default branch opens issues.

## Handle an issue

1. Read **What changed** and **Kyverno impact**, and open the comparison for each changed Go file.
1. Do the items under **Required actions**. For a policy change, open a pull request that references the issue.
1. Check each item under **Validation**.
1. Close the issue. A closed issue is never reopened or recreated.

To move the baseline, change `since` to a minor release whose changes the chart already handles. When upstream adds a
check, add it to `checks`. The tests fail when a mapped policy has no template.

## Test a change

The `Test` job runs `scripts/test-psa-release-issues.py` on each pull request that changes the workflow, the
script, the configuration, the `kyverno-policies` chart, or the PSS CLI tests. The tests use a local git repository shaped like the mirror, a fake GitHub API,
and a fake `gh` process, and they read the workflow file to check its permissions, pinned actions, timeouts,
inputs, defaults, and concurrency group. They don't run the workflow.

Only a real run verifies the workflow. Before you rely on it, in a fork:

1. Run it with `dry-run` set, and read the job summary.
1. Run it with `dry-run` cleared and a `tag` that needs an issue, and check the issue and its labels.
1. Run it again, and check that it doesn't open a second issue.
1. Set `PSA_CEL_UPDATE_ISSUES` to `true`, and check that a scheduled run starts.

### Test against a PSA fork

To see the whole path from an upstream change to an issue without touching upstream, point a manual run at a
fork of `kubernetes/pod-security-admission`. Forks don't copy tags, so you add two test tags:

1. In a clone of your PSA fork, check out a real release commit, such as the `kubernetes-1.38.0` tag from
   `kubernetes/pod-security-admission`, and tag it `kubernetes-1.98.0`. Versions 1.98 and 1.99 never become real
   releases, so the test issue's marker never blocks a real one.
1. Commit a small, clearly labeled change, such as a fake value added to the allowed list in
   `policy/check_sysctls.go`, and tag it `kubernetes-1.99.0`. Use annotated tags (`git tag -a`).
1. Push only the two tags to your PSA fork.
1. In your Kyverno fork, run the workflow with `repository` set to your PSA fork, `tag` set to
   `kubernetes-1.99.0`, and `dry-run` set. Check the job summary and the issue preview in the artifact.
1. Run it again with `dry-run` cleared to open the test issue, and once more to check that it doesn't open a second
   one.
1. Clean up: close the test issue, and delete the two tags from your PSA fork.

Use `tag`, because your fork doesn't have the baseline tag. The issue's links to `kubernetes/kubernetes` point to
a release that doesn't exist; the links to the mirror point to your fork.

To run the script locally, you need Python 3 with PyYAML and Git, and the GitHub CLI signed in with access to
`OWNER/NAME`, the repository that holds the issues:

```bash
python3 scripts/test-psa-release-issues.py
python3 scripts/psa-release-issues.py --repo OWNER/NAME --out /tmp/psa-release --dry-run
```

A dry run writes `summary.md` and one file per issue to `--out`. The script exits with `1` when it can't fetch
the mirror or handle a release, and the next run retries. It opens at most five issues per run, and a minor
release without the previous minor to compare with fails the run, so a person checks it.
