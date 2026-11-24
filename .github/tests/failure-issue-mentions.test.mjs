// Tests for the mentions input of .github/actions/workflow/failure-issue/action.yaml.
// Run with: node .github/tests/failure-issue-mentions.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const source = inlineScript('.github/actions/workflow/failure-issue/action.yaml')
  .replace("'${{ inputs.conclusion }}'", 'CONCLUSION')
  .replace('${{ inputs.needs }}', '{}');

const MARKER = '<!-- workflow-failure:Tests:refs/heads/main -->';

async function run({ mentions, open = [], labelError = false }) {
  const calls = { create: [], update: [], comment: [] };
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: { listJobsForWorkflowRun: async () => ({ data: [] }) },
      issues: {
        listForRepo: async () => ({ data: open }),
        create: async a => {
          if (labelError && a.labels) {
            throw Object.assign(new Error('Label does not exist'), { status: 422 });
          }
          calls.create.push(a);
          return { data: { number: 999, html_url: 'u' } };
        },
        update: async a => { calls.update.push(a); },
        addLabels: async () => {},
        createComment: async a => { calls.comment.push(a); },
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 1, runNumber: 10,
    ref: 'refs/heads/main', serverUrl: 's', sha: 'a'.repeat(40), actor: 'me',
  };
  Object.assign(process.env, { EXTRA_LABELS: '', NOTE: '', COMMITS: 'list', COMMITS_URL: '', MENTIONS: mentions });
  await runScript(source, { github, core: fakeCore(), context, Buffer, CONCLUSION: 'failure' });
  return calls;
}
const mentioned = body => [...body.matchAll(/(?:^|[\s,])@([A-Za-z0-9-]+)/g)].map(m => m[1]);

test('a new issue mentions each login once, at the top', async () => {
  const calls = await run({ mentions: 'alice bob alice' });
  const body = calls.create[0].body;
  assert.ok(body.startsWith('**Pull request authors in this run:** @alice, @bob. '));
  assert.deepEqual(mentioned(body), ['alice', 'bob', 'me']);
  assert.ok(body.endsWith(MARKER));
});

test('an existing issue is updated without mentions', async () => {
  const calls = await run({ mentions: 'alice', open: [{ number: 5, body: MARKER, labels: [] }] });
  assert.ok(!calls.update[0].body.includes('@alice'));
  assert.ok(calls.comment.every(c => !c.body.includes('@alice')));
});

test('at most 10 logins are mentioned', async () => {
  const logins = Array.from({ length: 13 }, (_, i) => `user${i}`);
  const calls = await run({ mentions: logins.join(' ') });
  const body = calls.create[0].body;
  assert.equal(mentioned(body).filter(l => l.startsWith('user')).length, 10);
  assert.ok(body.includes('@user9 and 3 more listed under **Commits**.'));
});

test('values that are not logins are dropped', async () => {
  const calls = await run({ mentions: 'ok dependabot[bot] -bad bad- a--b @team/x <b>x</b>' });
  assert.deepEqual(mentioned(calls.create[0].body), ['ok', 'me']);
});

test('without mentions the body is unchanged', async () => {
  const calls = await run({ mentions: '' });
  assert.ok(calls.create[0].body.startsWith('## Workflow failure'));
});

test('the unlabelled fallback also mentions', async () => {
  const calls = await run({ mentions: 'alice', labelError: true });
  assert.ok(calls.create[0].body.startsWith('**Pull request authors in this run:** @alice.'));
});
