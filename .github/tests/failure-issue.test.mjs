// Tests for .github/actions/workflow/failure-issue/action.yaml.
// Run with: node .github/tests/failure-issue.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const source = inlineScript('.github/actions/workflow/failure-issue/action.yaml')
  .replace("'${{ inputs.conclusion }}'", 'CONCLUSION')
  .replace('${{ inputs.needs }}', '{}');

const MARKER = '<!-- workflow-failure:Tests:refs/heads/main -->';
const BOTH = ['workflow-failure', 'e2e-failure'];
const issue = (number, labels, body = MARKER, extra = {}) =>
  ({ number, body, labels: labels.map(name => ({ name })), ...extra });
const wfRun = (run_number, conclusion, extra = {}) =>
  ({ run_number, head_branch: 'main', head_sha: 'f'.repeat(40), status: 'completed', conclusion, ...extra });

async function run({
  conclusion, labels = 'e2e-failure', note = '', latestRunOnly = 'false',
  runPages = [[]], runsError = false, runNumber = 10,
  open = [], hiddenFromLabelList = false,
}) {
  const calls = { create: [], update: [], addLabels: [], comment: [], lists: [], runPages: [] };
  const core = fakeCore();
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: {
        listJobsForWorkflowRun: async () => ({ data: [] }),
        listWorkflowRuns: async a => {
          assert.equal(a.workflow_id, 'check-tests.yaml');
          calls.runPages.push(a.page);
          if (runsError) throw new Error('nope');
          return { data: { workflow_runs: runPages[a.page - 1] || [] } };
        },
      },
      issues: {
        listForRepo: async a => {
          calls.lists.push(a.labels || '(unfiltered)');
          if (a.labels) {
            return { data: hiddenFromLabelList ? [] : open.filter(i => i.labels.some(l => l.name === a.labels)) };
          }
          const page = a.page || 1;
          return { data: open.slice((page - 1) * 100, page * 100) };
        },
        create: async a => { calls.create.push(a); return { data: { number: 999, html_url: 'u' } }; },
        update: async a => { calls.update.push(a); },
        addLabels: async a => { calls.addLabels.push(a); },
        createComment: async a => { calls.comment.push(a); },
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 1, runNumber,
    ref: 'refs/heads/main', serverUrl: 's', sha: 'a'.repeat(40), actor: 'me',
  };
  Object.assign(process.env, {
    EXTRA_LABELS: labels, NOTE: note, LATEST_RUN_ONLY: latestRunOnly,
    GITHUB_WORKFLOW_REF: 'o/r/.github/workflows/check-tests.yaml@refs/heads/main',
  });
  await runScript(source, { github, core, context, Buffer, CONCLUSION: conclusion });
  return { calls, log: core.log, writes: calls.create.length + calls.update.length + calls.addLabels.length + calls.comment.length };
}
const closed = calls => calls.update.filter(u => u.state === 'closed').map(u => u.issue_number);

test('latest-run-only: a re-run of an old run that passes after a newer run failed leaves the issue open', async () => {
  const r = await run({ conclusion: 'success', latestRunOnly: 'true', runPages: [[wfRun(11, 'failure'), wfRun(10, 'success')]], open: [issue(5, BOTH)] });
  assert.equal(r.writes, 0);
  assert.equal(r.calls.lists.length, 0, 'returns before looking for the issue');
  assert.ok(r.log.some(l => l.startsWith('notice:') && l.includes('Run #11') && l.includes('newer')));
});

test('latest-run-only: a re-run of an old run that fails after a newer run passed opens no issue', async () => {
  const r = await run({ conclusion: 'failure', latestRunOnly: 'true', runPages: [[wfRun(12, 'success')]] });
  assert.equal(r.writes, 0);
});

test('latest-run-only: the newest finished run acts', async () => {
  const r = await run({ conclusion: 'success', latestRunOnly: 'true', runNumber: 12, runPages: [[wfRun(12, 'success'), wfRun(11, 'failure')]], open: [issue(5, BOTH)] });
  assert.deepEqual(closed(r.calls), [5]);
});

test('latest-run-only: newer runs that decided nothing do not count', async () => {
  for (const newer of [
    wfRun(11, null, { status: 'in_progress' }),
    wfRun(11, null, { status: 'queued' }),
    wfRun(11, 'cancelled'),
    wfRun(11, 'failure', { head_branch: 'release-1.20' }),
  ]) {
    const r = await run({ conclusion: 'success', latestRunOnly: 'true', runPages: [[newer]], open: [issue(5, BOTH)] });
    assert.deepEqual(closed(r.calls), [5], `${newer.status}/${newer.conclusion} on ${newer.head_branch}`);
  }
});

test('latest-run-only: a newer finished run beyond the first page of runs is found', async () => {
  const otherBranch = Array.from({ length: 100 }, (_, i) => wfRun(300 - i, 'success', { head_branch: 'release-1.20' }));
  const r = await run({ conclusion: 'success', latestRunOnly: 'true', runPages: [otherBranch, [wfRun(150, 'failure'), wfRun(10, 'failure')]], open: [issue(5, BOTH)] });
  assert.equal(r.writes, 0);
  assert.deepEqual(r.calls.runPages, [1, 2]);
});

test('latest-run-only: paging stops once this run is reached', async () => {
  const page = [...Array.from({ length: 99 }, (_, i) => wfRun(109 - i, 'cancelled')), wfRun(10, 'failure')];
  const r = await run({ conclusion: 'success', latestRunOnly: 'true', runPages: [page, [wfRun(9, 'success')]], open: [issue(5, BOTH)] });
  assert.deepEqual(r.calls.runPages, [1]);
  assert.deepEqual(closed(r.calls), [5]);
});

test('latest-run-only: if the runs cannot be listed, the run acts and warns', async () => {
  const r = await run({ conclusion: 'success', latestRunOnly: 'true', runsError: true, open: [issue(5, BOTH)] });
  assert.deepEqual(closed(r.calls), [5]);
  assert.ok(r.log.some(l => l.startsWith('warning:') && l.includes('Could not list the runs')));
});

test('without the input, runs are not listed and an old run still acts', async () => {
  const r = await run({ conclusion: 'success', labels: '', runPages: [[wfRun(11, 'failure')]], open: [issue(5, ['workflow-failure'])] });
  assert.deepEqual(r.calls.runPages, []);
  assert.deepEqual(closed(r.calls), [5]);
  assert.deepEqual(r.calls.lists, ['workflow-failure'], 'one list request, as before');
});

test('success closes every tracking issue for the branch, and only for that branch', async () => {
  const r = await run({ conclusion: 'success', open: [issue(7, BOTH), issue(5, BOTH), issue(6, BOTH, '<!-- workflow-failure:Tests:refs/heads/release-1.20 -->')] });
  assert.deepEqual(closed(r.calls), [5, 7]);
  assert.equal(r.calls.comment.length, 2);
});

test('failure updates the oldest tracking issue and closes the others as duplicates', async () => {
  const r = await run({ conclusion: 'failure', open: [issue(7, BOTH), issue(5, BOTH)] });
  assert.equal(r.calls.update[0].issue_number, 5);
  assert.ok(r.calls.update[0].body.includes(MARKER));
  const duplicate = r.calls.update.find(u => u.issue_number === 7);
  assert.equal(duplicate.state, 'closed');
  assert.equal(duplicate.state_reason, 'not_planned');
  assert.ok(r.calls.comment[0].body.includes('#5'));
  assert.equal(r.calls.create.length, 0);
});

test('an issue that lost the workflow-failure label is still found, and relabelled', async () => {
  const failing = await run({ conclusion: 'failure', open: [issue(5, ['e2e-failure'])] });
  assert.equal(failing.calls.create.length, 0);
  assert.equal(failing.calls.update[0].issue_number, 5);
  assert.deepEqual(failing.calls.addLabels[0].labels, ['workflow-failure']);
  const passing = await run({ conclusion: 'success', open: [issue(5, ['e2e-failure'])] });
  assert.deepEqual(closed(passing.calls), [5]);
});

test('an issue with no labels left is found through the unfiltered list, on failure and on success', async () => {
  const failing = await run({ conclusion: 'failure', open: [issue(5, [])] });
  assert.equal(failing.calls.create.length, 0);
  assert.deepEqual(failing.calls.addLabels[0].labels, BOTH);
  const passing = await run({ conclusion: 'success', open: [issue(7, BOTH), issue(5, [])] });
  assert.deepEqual(closed(passing.calls), [5, 7]);
});

test('an empty label-filtered list does not produce a second issue', async () => {
  const r = await run({ conclusion: 'failure', open: [issue(5, BOTH)], hiddenFromLabelList: true });
  assert.equal(r.calls.create.length, 0);
  assert.equal(r.calls.update[0].issue_number, 5);
});

test('the unfiltered list is read to the end, so an older tracking issue on a later page is found', async () => {
  const filler = Array.from({ length: 100 }, (_, i) => issue(1000 + i, [], 'unrelated'));
  const r = await run({ conclusion: 'success', open: [issue(900, []), ...filler.slice(1), issue(5, [])], hiddenFromLabelList: true });
  assert.deepEqual(closed(r.calls), [5, 900]);
});

test('nothing open: a failure creates the issue with every label, ignoring a pull request that quotes the marker', async () => {
  const r = await run({ conclusion: 'failure', open: [issue(3, BOTH, MARKER, { pull_request: {} })] });
  assert.equal(r.calls.create.length, 1);
  assert.deepEqual(r.calls.create[0].labels, BOTH);
});

test('nothing open: a success writes nothing', async () => {
  assert.equal((await run({ conclusion: 'success' })).writes, 0);
});

test('callers without extra labels: a passing run makes one list request, a failing one checks the unfiltered list before creating', async () => {
  const passing = await run({ conclusion: 'success', labels: '' });
  assert.deepEqual(passing.calls.lists, ['workflow-failure']);
  const failing = await run({ conclusion: 'failure', labels: '' });
  assert.deepEqual(failing.calls.lists, ['workflow-failure', '(unfiltered)']);
  assert.deepEqual(failing.calls.create[0].labels, ['workflow-failure']);
});

test('the note sits between the summary table and the job results', async () => {
  const r = await run({ conclusion: 'failure', note: '### Clearing the E2E Gate\ntext' });
  const body = r.calls.create[0].body;
  const at = s => body.indexOf(s);
  assert.ok(at('| **Conclusion** |') < at('### Clearing the E2E Gate'));
  assert.ok(at('### Clearing the E2E Gate') < at('\n---\n'));
  assert.ok(at('\n---\n') < at(MARKER));
});

test('without a note the body ends with the table, the footer and the marker', async () => {
  const r = await run({ conclusion: 'failure', labels: '', note: '' });
  assert.match(r.calls.create[0].body, /\| \*\*Conclusion\*\* \| `failure` \|\n---\n\*This issue is maintained/);
});
