// Tests for the script of .github/workflows/e2e-gate.yaml.
// Run with: node .github/tests/e2e-gate.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const source = inlineScript('.github/workflows/e2e-gate.yaml');

const marker = branch => `body\n<!-- workflow-failure:Tests:refs/heads/${branch} -->`;
/** A tracking issue; branch null means labelled by hand, without the marker. */
const issue = (number, branch, extra = {}) => ({
  number, html_url: `u${number}`, body: branch ? marker(branch) : 'by hand',
  state: 'open', labels: [{ name: 'e2e-failure' }], ...extra,
});
const red = (n, base = 'main') => ({
  state: 'FAILURE', targetUrl: `u${n}`,
  description: `Not caused by this PR. Post-merge tests are failing on ${base} (#${n}); clears when they pass or the issue is closed`,
});
// The text and link written before the wording changed.
const oldRed = n => ({
  state: 'FAILURE', targetUrl: `https://github.com/o/r/issues/${n}`,
  description: `Post-merge tests are failing on main (#${n}). Clears when the Tests workflow passes or the issue is closed`,
});
const green = (base = 'main') => ({ state: 'SUCCESS', description: `No open e2e-failure issue for ${base}`, targetUrl: null });
/** A pull request as the GraphQL queries return it. */
const pull = (number, base, { labels = [], current = null, sha = `sha${number}`, state = 'OPEN', statusOid } = {}) => ({
  number, state, baseRefName: base, headRefOid: sha, labels: { nodes: labels.map(name => ({ name })) },
  commits: { nodes: [{ commit: { oid: statusOid || sha, status: current ? { context: current } : null } }] },
});

/**
 * @param listed    what the label-filtered issue list returns
 * @param byNumber  what reading an issue by number returns (missing: 404)
 * @param prs       the open pull requests
 * @param fresh     the pull requests as re-read later, when they changed since listing
 */
async function run({ eventName, payload = {}, listed = [], byNumber = {}, prs = [], fresh = null,
                     failWriteOn = null, failReadOf = null, failIssueReadOnce = null }) {
  const writes = [], issueReads = [];
  const core = fakeCore();
  const latest = () => fresh || prs;
  let issueReadFailures = failIssueReadOnce ? 1 : 0;
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      issues: {
        listForRepo: async a => {
          assert.equal(a.labels, 'e2e-failure');
          assert.equal(a.state, 'open');
          return { data: listed };
        },
        get: async a => {
          issueReads.push(a.issue_number);
          if (a.issue_number === failIssueReadOnce && issueReadFailures-- > 0) {
            throw Object.assign(new Error('Bad Gateway'), { status: 502 });
          }
          if (!byNumber[a.issue_number]) {
            throw Object.assign(new Error('Not Found'), { status: 404 });
          }
          return { data: byNumber[a.issue_number] };
        },
      },
      repos: {
        createCommitStatus: async a => {
          if (a.sha === failWriteOn) throw new Error('boom');
          writes.push(a);
        },
        listPullRequestsAssociatedWithCommit: async a => ({
          data: latest().filter(p => p.headRefOid === a.commit_sha || p.contains === a.commit_sha).map(p => ({
            number: p.number, state: p.state === 'OPEN' ? 'open' : 'closed',
            head: { sha: p.headRefOid }, base: { ref: p.baseRefName }, labels: p.labels.nodes,
          })),
        }),
      },
    },
    graphql: async (query, variables) => {
      if ('number' in variables) {
        if (variables.number === failReadOf) throw new Error('502');
        return { repository: { pullRequest: latest().find(p => p.number === variables.number) } };
      }
      assert.equal(variables.size, 30);
      // Two pages, so that the cursor is followed.
      const page = variables.cursor ? prs.slice(1) : prs.slice(0, 1);
      return { repository: { pullRequests: { pageInfo: { hasNextPage: !variables.cursor && prs.length > 1, endCursor: 'c1' }, nodes: page } } };
    },
  };
  const context = { repo: { owner: 'o', repo: 'r' }, eventName, payload };
  await runScript(source, { github, core, context, setTimeout: fn => fn() });
  return { writes, issueReads, log: core.log, failure: core.failure };
}
const prEvent = number => ({ eventName: 'pull_request_target', payload: { pull_request: { number } } });
const states = writes => writes.map(w => w.state);

test('a PR to a failing branch gets a red status that says it is not caused by the PR', async () => {
  const r = await run({ ...prEvent(5), listed: [issue(18609, 'release-1.20')], prs: [pull(5, 'release-1.20')] });
  assert.equal(r.writes.length, 1);
  const [status] = r.writes;
  assert.deepEqual([status.state, status.sha, status.context, status.target_url], ['failure', 'sha5', 'E2E Gate', 'u18609']);
  assert.ok(status.description.startsWith('Not caused by this PR.'));
  assert.ok(status.description.includes('#18609'));
  assert.ok(status.description.length <= 140);
});

test('an issue for another branch, including one whose name is a prefix, does not gate the PR', async () => {
  const r = await run({ ...prEvent(5), listed: [issue(9, 'release-1.1')], prs: [pull(5, 'release-1.19')] });
  assert.deepEqual(states(r.writes), ['success']);
});

test('the bypass label turns the PR green while the branch is failing', async () => {
  const r = await run({ ...prEvent(5), listed: [issue(9, 'main')], prs: [pull(5, 'main', { labels: ['e2e-gate-bypass'] })] });
  assert.deepEqual(states(r.writes), ['success']);
  assert.ok(r.writes[0].description.includes('e2e-gate-bypass'));
});

test('an issue labelled by hand gates every branch, and a labelled pull request gates nothing', async () => {
  const r = await run({ eventName: 'workflow_dispatch', listed: [issue(901, null), { ...issue(77, 'main'), pull_request: {} }], prs: [pull(1, 'main'), pull(2, 'release-1.19')] });
  assert.deepEqual(states(r.writes), ['failure', 'failure']);
  assert.ok(r.writes.every(w => w.description.includes('#901') && !w.description.includes('#77')));
});

test('a sweep writes only the statuses that changed', async () => {
  const r = await run({ eventName: 'workflow_run', prs: [pull(1, 'main', { current: green() }), pull(2, 'main', { current: red(900) }), pull(3, 'release-1.19')] });
  assert.deepEqual(r.writes.map(w => [w.sha, w.state]), [['sha2', 'success'], ['sha3', 'success']]);
  assert.deepEqual(r.issueReads, [900], 'the issue named in the red status is checked once before clearing it');
  assert.ok(r.log.includes('info: Evaluated 3 pull request(s), updated 2.'));
  assert.equal(r.failure, null);
});

test('an empty issue list does not clear a red status whose issue is still open', async () => {
  const r = await run({ eventName: 'workflow_run', byNumber: { 18166: issue(18166, 'main') }, prs: [pull(1, 'main', { current: red(18166) }), pull(2, 'main', { current: red(18166) }), pull(3, 'main')] });
  assert.deepEqual(r.writes.map(w => [w.sha, w.state]), [['sha3', 'failure']], 'nothing turns green, and the PR without a status turns red');
  assert.deepEqual(r.issueReads, [18166], 'one read for the whole sweep');
  assert.ok(r.log.some(l => l.startsWith('warning:') && l.includes('#18166') && l.includes('missing from the issue list')));
});

test('the issue is also recognised in the status text used before this change', async () => {
  const r = await run({ ...prEvent(1), byNumber: { 18166: issue(18166, 'main') }, prs: [pull(1, 'main', { current: oldRed(18166) })] });
  assert.deepEqual(states(r.writes), ['failure']);
  assert.ok(r.writes[0].description.startsWith('Not caused by this PR.'));
});

test('a red status clears when its issue is closed, unlabelled, deleted or about another branch', async () => {
  const cases = {
    closed: { 7: issue(7, 'main', { state: 'closed' }) },
    unlabelled: { 7: issue(7, 'main', { labels: [] }) },
    deleted: {},
    'another branch': { 7: issue(7, 'release-1.20') },
  };
  for (const [name, byNumber] of Object.entries(cases)) {
    const r = await run({ ...prEvent(1), byNumber, prs: [pull(1, 'main', { current: red(7) })] });
    assert.deepEqual(states(r.writes), ['success'], name);
  }
});

test('when the issue cannot be read, the PR stays red, and the next PR asks again instead of going green', async () => {
  const r = await run({ eventName: 'workflow_run', byNumber: { 7: issue(7, 'main') }, failIssueReadOnce: 7, prs: [pull(1, 'main', { current: red(7) }), pull(2, 'main', { current: red(7) })] });
  assert.deepEqual(r.writes, [], 'no status is written: both stay red');
  assert.deepEqual(r.issueReads, [7, 7]);
  assert.ok(r.failure.includes('#1'), 'the run fails for the PR that could not be checked');
});

test('one commit opened against a failing and a passing branch gets the red status, whichever PR the event is for', async () => {
  const prs = [pull(20, 'main', { sha: 'S' }), pull(21, 'release-x', { sha: 'S' })];
  const event = await run({ ...prEvent(21), listed: [issue(19, 'main')], prs });
  assert.deepEqual(states(event.writes), ['failure']);
  const sweep = await run({ eventName: 'schedule', listed: [issue(19, 'main')], prs });
  assert.deepEqual(sweep.writes.map(w => [w.sha, w.state]), [['S', 'failure']], 'the commit is written once');
});

test('a PR that merely contains the commit, or is closed, is not a sibling', async () => {
  const stacked = { ...pull(20, 'main', { sha: 'T' }), contains: 'S' };
  const closedSibling = pull(22, 'main', { sha: 'S', state: 'CLOSED' });
  const r = await run({ ...prEvent(21), listed: [issue(19, 'main')], prs: [stacked, closedSibling, pull(21, 'release-x', { sha: 'S' })] });
  assert.deepEqual(states(r.writes), ['success']);
});

test('an event for a closed PR writes nothing', async () => {
  const r = await run({ ...prEvent(1), listed: [issue(9, 'main')], prs: [pull(1, 'main', { state: 'CLOSED' })] });
  assert.deepEqual(r.writes, []);
});

test('a PR that cannot be written or re-read does not stop the sweep, and the run fails', async () => {
  const write = await run({ eventName: 'workflow_run', prs: [pull(1, 'main'), pull(2, 'main')], failWriteOn: 'sha1' });
  assert.deepEqual(write.writes.map(w => w.sha), ['sha2']);
  assert.ok(write.failure.includes('#1'));
  const read = await run({ eventName: 'workflow_run', prs: [pull(1, 'main'), pull(2, 'main')], failReadOf: 1 });
  assert.deepEqual(read.writes.map(w => w.sha), ['sha2']);
  assert.ok(read.failure.includes('#1'));
});

test('a sweep re-reads a PR before writing: merged or pushed-to PRs are skipped, a new bypass label is honoured', async () => {
  const base = { eventName: 'workflow_run', listed: [issue(9, 'main')], prs: [pull(1, 'main')] };
  assert.deepEqual((await run({ ...base, fresh: [pull(1, 'main', { state: 'MERGED' })] })).writes, []);
  assert.deepEqual((await run({ ...base, fresh: [pull(1, 'main', { sha: 'pushed' })] })).writes, []);
  assert.deepEqual(states((await run({ ...base, fresh: [pull(1, 'main', { labels: ['e2e-gate-bypass'] })] })).writes), ['success']);
});

test('a status read from a commit that is not the head is not trusted', async () => {
  const r = await run({ ...prEvent(1), prs: [pull(1, 'main', { current: green(), statusOid: 'older' })] });
  assert.deepEqual(r.writes.map(w => w.sha), ['sha1']);
});

test('two open issues for one branch are reported, and a long list of issues still fits the status', async () => {
  const two = await run({ ...prEvent(1), listed: [issue(5, 'main'), issue(6, 'main')], prs: [pull(1, 'main')] });
  assert.ok(two.log.some(l => l.startsWith('warning:') && l.includes('More than one')));
  const many = await run({ ...prEvent(1), listed: Array.from({ length: 30 }, (_, i) => issue(10000 + i, null)), prs: [pull(1, 'main')] });
  assert.ok(many.writes[0].description.length <= 140);
});
