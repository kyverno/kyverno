// Tests for the "Summarize batch" step of .github/workflows/check-tests.yaml.
// Run with: node .github/tests/check-tests-batch.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const source = inlineScript('.github/workflows/check-tests.yaml');
const sha = c => c.repeat(40);
const HEAD = sha('f');
const wfRun = (id, run_number, head_sha, conclusion, matrix = 'full') => ({
  id, run_number, head_sha, conclusion, display_title: `Tests on main (${matrix})`,
  html_url: `run/${id}`,
});
const commit = (c, title, login = 'dev') => ({
  sha: sha(c), html_url: `commit/${c}`, author: { login }, commit: { message: `${title}\n\nbody`, author: { name: login } },
});

async function run({ matrix = 'full', green = [], recent = [], commits = [], prs = {} }) {
  const core = fakeCore();
  const outputs = {};
  let summary = '';
  core.setOutput = (k, v) => { outputs[k] = v; };
  core.summary = {
    addHeading(t) { summary += `# ${t}\n`; return this; },
    addRaw(t) { summary += t; return this; },
    write: async () => {},
  };
  const github = {
    paginate: async (fn, args, map) => {
      const response = await fn(args);
      return map ? map(response, () => {}) : response.data;
    },
    rest: {
      actions: {
        // github.paginate normalizes a { total_count, workflow_runs }
        // response so that response.data is the array of runs, and this stub
        // stands in for that normalized page.
        listWorkflowRuns: async a => a.status === 'success'
          ? { data: green }
          : { data: { workflow_runs: recent } },
      },
      repos: {
        compareCommitsWithBasehead: async () => ({ data: { commits } }),
        listPullRequestsAssociatedWithCommit: async a => ({ data: prs[a.commit_sha] || [] }),
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, runId: 100, runNumber: 50,
    ref: 'refs/heads/main', serverUrl: 's', sha: HEAD,
  };
  process.env.MATRIX = matrix;
  await runScript(source, { github, core, context });
  return { outputs, summary, log: core.log };
}

test('without a green run, the range is unknown and there is no compare URL', async () => {
  const { outputs, summary } = await run({});
  assert.match(outputs.markdown, /No earlier green full run/);
  assert.equal(outputs['compare-url'], '');
  assert.ok(summary.includes(outputs.markdown));
});

test('a reduced green run does not cover a full run', async () => {
  const { outputs } = await run({ green: [wfRun(90, 40, sha('a'), 'success', 'reduced')] });
  assert.match(outputs.markdown, /No earlier green full run/);
});

test('lists the commits since the last green run, newest first, with their pull requests', async () => {
  const commits = [commit('b', 'first'), commit('c', 'second')];
  const prs = { [sha('c')]: [{ number: 7, html_url: 'pr/7', merged_at: 'x', base: { ref: 'main' }, user: { login: 'alice' } }] };
  const { outputs } = await run({ green: [wfRun(90, 40, sha('a'), 'success')], commits, prs });
  assert.equal(outputs['compare-url'], `s/o/r/compare/${sha('a')}...${HEAD}`);
  const md = outputs.markdown;
  assert.ok(md.includes('[`aaaaaaaaa..fffffffff`]'));
  assert.ok(md.includes('(2 commits)'));
  assert.ok(md.indexOf('second') < md.indexOf('first'));
  assert.ok(md.includes('| [`ccccccccc`](commit/c) | [#7](pr/7) | alice | second |'));
  assert.ok(md.includes('Authors: alice, dev'));
  assert.ok(!md.includes('New in this run'));
});

test('lists the authors of merged pull requests to mention, without bots or duplicates', async () => {
  const pr = (number, login, type = 'User', extra = {}) =>
    ({ number, html_url: `pr/${number}`, merged_at: 'x', base: { ref: 'main' }, user: { login, type }, ...extra });
  const commits = [commit('b', 'one'), commit('c', 'two'), commit('d', 'three'), commit('e', 'four'), commit('9', 'five', 'direct')];
  const prs = {
    [sha('b')]: [pr(1, 'zoe')],
    [sha('c')]: [pr(2, 'alice')],
    [sha('d')]: [pr(3, 'dependabot[bot]', 'Bot')],
    [sha('e')]: [pr(4, 'alice'), pr(5, 'other', 'User', { merged_at: null }), pr(6, 'bob')],
  };
  const { outputs } = await run({ green: [wfRun(90, 40, sha('a'), 'success')], commits, prs });
  assert.equal(outputs['pr-authors'], 'alice bob zoe');
});

test('without a batch range there is nobody to mention', async () => {
  const { outputs } = await run({});
  assert.equal(outputs['pr-authors'], '');
});

test('marks the commits the previous failing run did not test', async () => {
  const commits = [commit('b', 'old'), commit('c', 'new one'), commit('d', 'new two')];
  const { outputs } = await run({
    green: [wfRun(90, 40, sha('a'), 'success')],
    recent: [wfRun(100, 50, HEAD, null), wfRun(95, 45, sha('b'), 'failure'), wfRun(93, 44, sha('9'), 'cancelled')],
    commits,
  });
  const md = outputs.markdown;
  assert.ok(md.includes('[previous run](run/95) tested `bbbbbbbbb` (failure). 2 of these commits are new in this run.'));
  assert.ok(md.includes('| Commit | Pull request | Author | Title | New in this run |'));
  assert.match(md, /\| new one \| yes \|/);
  assert.match(md, /\| old \|  \|/);
});

test('escapes titles so that they cannot break the table or mention people', async () => {
  const { outputs } = await run({ green: [wfRun(90, 40, sha('a'), 'success')], commits: [commit('b', 'a | b <x> @team & co', '@bot')] });
  assert.ok(outputs.markdown.includes('a \\| b &lt;x&gt; @&#8203;team &amp; co'));
  assert.ok(!/(^|[^;])@team/.test(outputs.markdown));
});

test('escapes backslashes so that an escaped pipe cannot end a cell', async () => {
  const { outputs } = await run({ green: [wfRun(90, 40, sha('a'), 'success')], commits: [commit('b', 'a \\| b', 'dev')] });
  assert.ok(outputs.markdown.includes('a \\\\\\| b'));
});

test('the same commit as the last green run gives a one-line summary', async () => {
  const { outputs } = await run({ green: [wfRun(90, 40, HEAD, 'success')] });
  assert.match(outputs.markdown, /tested the same commit/);
});
