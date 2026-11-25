// Tests for the commits input of .github/actions/workflow/failure-issue/action.yaml.
// Run with: node .github/tests/failure-issue-commits.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const source = inlineScript('.github/actions/workflow/failure-issue/action.yaml')
  .replace("'${{ inputs.conclusion }}'", 'CONCLUSION')
  .replace('${{ inputs.needs }}', '{}');

const MARKER = '<!-- workflow-failure:Tests:refs/heads/main -->';
const LIMIT = 65536;
const COMMITS = 'Commits since the [last green run](g): [`aaa..bbb`](c) (2 commits).\n\n| Commit | Title |\n| --- | --- |\n| b | two |\n| a | one |';

async function run({ conclusion = 'failure', commits = COMMITS, commitsUrl = 'https://compare', open = [], commentError = false }) {
  const calls = { create: [], update: [], comment: [] };
  const core = fakeCore();
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: { listJobsForWorkflowRun: async () => ({ data: [] }) },
      issues: {
        listForRepo: async () => ({ data: open }),
        create: async a => { calls.create.push(a); return { data: { number: 999, html_url: 'u' } }; },
        update: async a => { calls.update.push(a); },
        addLabels: async () => {},
        createComment: async a => {
          if (commentError) throw new Error('nope');
          calls.comment.push(a);
        },
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 1, runNumber: 10,
    ref: 'refs/heads/main', serverUrl: 's', sha: 'a'.repeat(40), actor: 'me',
  };
  Object.assign(process.env, { EXTRA_LABELS: '', NOTE: '', COMMITS: commits, COMMITS_URL: commitsUrl });
  await runScript(source, { github, core, context, Buffer, CONCLUSION: conclusion });
  return { calls, log: core.log };
}
const existing = { number: 5, body: MARKER, labels: [{ name: 'workflow-failure' }] };
const bigCommits = n => ['| Commit | Title |', '| --- | --- |',
  ...Array.from({ length: n }, (_, i) => `| c${i} | ${'x'.repeat(200)} |`)].join('\n');

test('a new issue lists the commits above the footer', async () => {
  const { calls } = await run({});
  const body = calls.create[0].body;
  assert.ok(body.includes(`### Commits\n\n${COMMITS}\n`));
  assert.ok(body.indexOf('### Commits') < body.indexOf(MARKER));
  assert.equal(calls.comment.length, 0);
});

test('an existing issue gets the commits in its body and in a comment for this run', async () => {
  const { calls } = await run({ open: [existing] });
  assert.ok(calls.update[0].body.includes(COMMITS));
  assert.equal(calls.comment.length, 1);
  assert.equal(calls.comment[0].issue_number, 5);
  assert.ok(calls.comment[0].body.includes('[Tests run](s/o/r/actions/runs/1)'));
  assert.ok(calls.comment[0].body.endsWith(COMMITS));
});

test('without commits, the body has no commits section and no comment is added', async () => {
  const { calls } = await run({ commits: '', open: [existing] });
  assert.ok(!calls.update[0].body.includes('### Commits'));
  assert.equal(calls.comment.length, 0);
  assert.match(calls.update[0].body, /\| \*\*Conclusion\*\* \| `failure` \|\n---\n/);
});

test('a long list is cut at a row and links to the full comparison', async () => {
  const { calls } = await run({ commits: bigCommits(400), open: [existing] });
  for (const body of [calls.update[0].body, calls.comment[0].body]) {
    assert.ok(body.length <= LIMIT, `${body.length}`);
    assert.ok(body.includes('| c0 |'), 'keeps the first rows');
    assert.ok(!body.includes('| c399 |'));
    assert.ok(body.includes('_The list is truncated. [Compare all commits](https://compare)._'));
    assert.match(body, /\| c\d+ \| x+ \|\n\n_The list is truncated/);
  }
});

test('without a URL the truncation note has no link', async () => {
  const { calls } = await run({ commits: bigCommits(400), commitsUrl: '' });
  assert.ok(calls.create[0].body.includes('_The list is truncated._'));
});

test('a failed comment warns and leaves the updated issue in place', async () => {
  const { calls, log } = await run({ open: [existing], commentError: true });
  assert.equal(calls.update.length, 1);
  assert.ok(log.some(l => l.startsWith('warning:') && l.includes('Could not comment')));
});

test('a passing run closes the issue without listing commits', async () => {
  const { calls } = await run({ conclusion: 'success', open: [existing] });
  assert.equal(calls.comment.length, 1);
  assert.ok(!calls.comment[0].body.includes('### Commits'));
  assert.equal(calls.update[0].state, 'closed');
});
