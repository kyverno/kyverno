// Tests for the /conformance command in .github/workflows/comment-commands.yaml:
// the "Authorize and validate conformance command" and "Comment result" steps.
// Run with: node --test .github/tests/conformance-command.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const file = '.github/workflows/comment-commands.yaml';
const validate = inlineScript(file, 'Authorize and validate conformance command');
const report = inlineScript(file, 'Comment result');

const SUITES = [
  { id: 'assert', chainsaw: true, api: '' },
  { id: 'mutate', chainsaw: true, api: '' },
  { id: 'cel-http', chainsaw: true, api: '' },
  { id: 'generate-mutating-admission-policy', chainsaw: true, api: 'v1beta1' },
  { id: 'helm-tests', chainsaw: false, api: '' },
];
const NODES = [
  { version: 'v1.34.11', api: 'v1beta1' },
  { version: 'v1.35.8', api: 'v1beta1' },
  { version: 'v1.36.4', api: 'v1' },
  { version: 'v1.37.0', api: 'v1' },
];
const HEAD = 'a'.repeat(40);

async function plan(body, { permission = 'write', sender = 'maintainer', state = 'open' } = {}) {
  const core = fakeCore();
  const outputs = {};
  core.setOutput = (k, v) => { outputs[k] = v; };
  const asked = [];
  const github = {
    rest: {
      repos: {
        getCollaboratorPermissionLevel: async ({ username }) => {
          asked.push(username);
          return { data: { permission: username === 'maintainer' ? permission : 'read' } };
        },
      },
      pulls: { get: async () => ({ data: { state, head: { sha: HEAD } } }) },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' },
    issue: { number: 7 },
    payload: { comment: { body, user: { login: 'maintainer' } }, sender: { login: sender } },
  };
  process.env.SUITES = JSON.stringify(SUITES);
  process.env.NODES = JSON.stringify(NODES);
  await runScript(validate, { core, github, context });
  return { outputs, failure: core.failure, asked };
}

const groups = outputs => JSON.parse(outputs.groups).map(g => [g.k8s, JSON.parse(g.suites)]);

test('a bare command runs each suite on the latest version it supports', async () => {
  const { outputs } = await plan('/conformance');
  assert.equal(outputs.valid, 'true');
  assert.equal(outputs.sha, HEAD);
  assert.equal(outputs['chainsaw-tests'], '');
  // The v1beta1-only suite runs on v1.35, the newest version it supports.
  assert.deepEqual(groups(outputs), [
    ['v1.37.0', ['assert', 'mutate', 'cel-http', 'helm-tests']],
    ['v1.35.8', ['generate-mutating-admission-policy']],
  ]);
  assert.equal(outputs['k8s-summary'], 'Kubernetes v1.37.0 (4 suites) and v1.35.8 (1 suites)');
});

test('a suite name selects one suite', async () => {
  const { outputs } = await plan('/conformance mutate\r\nplease');
  assert.equal(outputs.valid, 'true');
  assert.deepEqual(groups(outputs), [['v1.37.0', ['mutate']]]);
  assert.equal(outputs.description, 'suite `mutate`');
  assert.equal(outputs['k8s-summary'], 'Kubernetes v1.37.0');
});

test('a suite defaults to the latest version it supports', async () => {
  const { outputs } = await plan('/conformance generate-mutating-admission-policy');
  assert.deepEqual(groups(outputs), [['v1.35.8', ['generate-mutating-admission-policy']]]);
});

test('a suite on a version it does not support is rejected', async () => {
  const { outputs } = await plan('/conformance generate-mutating-admission-policy v1.37');
  assert.equal(outputs.valid, 'false');
  assert.match(outputs.error, /doesn't run on Kubernetes `v1.37.0`/);
});

test('suite/regex scopes the regex to the suite', async () => {
  const { outputs } = await plan('/conformance mutate/foreach-.* 1.36');
  assert.equal(outputs.valid, 'true');
  assert.deepEqual(groups(outputs), [['v1.36.4', ['mutate']]]);
  assert.equal(outputs['chainsaw-tests'], 'foreach-.*');
});

test('a bare regex runs every chainsaw suite', async () => {
  const { outputs } = await plan('/conformance ^cel.*$ v1.34.11');
  assert.equal(outputs.valid, 'true');
  assert.equal(outputs['chainsaw-tests'], '^cel.*$');
  assert.deepEqual(groups(outputs), [['v1.34.11', ['assert', 'mutate', 'cel-http', 'generate-mutating-admission-policy']]]);
});

test('a regex accepts RE2-compatible syntax', async () => {
  for (const regex of ['a{2,1000}', 'x(?:y|z)+?', '[a-z]{3}/b']) {
    const { outputs } = await plan(`/conformance mutate/${regex}`);
    assert.equal(outputs.valid, 'true', regex);
    assert.equal(outputs['chainsaw-tests'], regex);
  }
});

test('a version alone runs every suite on that version', async () => {
  const { outputs } = await plan('/conformance v1.35');
  assert.deepEqual(groups(outputs), [['v1.35.8', ['assert', 'mutate', 'cel-http', 'generate-mutating-admission-policy', 'helm-tests']]]);
});

test('a regex on a non-chainsaw suite is rejected', async () => {
  const { outputs } = await plan('/conformance helm-tests/x');
  assert.equal(outputs.valid, 'false');
});

for (const [name, body] of [
  ['an unknown suite', '/conformance mutat'],
  ['an unknown version', '/conformance mutate v1.99'],
  ['a malformed version', '/conformance mutate 1.36.4.1'],
  ['too many arguments', '/conformance mutate v1.36 extra'],
  ['a quote in the regex', "/conformance mutate/a'b"],
  ['a backtick in the regex', '/conformance a`b.*'],
  ['a backslash in the regex', '/conformance mutate/a\\.b'],
  ['a semicolon in the regex', '/conformance a;b.*'],
  ['a double quote in the regex', '/conformance a"b.*'],
  ['an empty suite regex', '/conformance mutate/'],
  ['an invalid regex', '/conformance mutate/a(b'],
  ['a prefix-only match', '/conformancefoo'],
  ['a long line', `/conformance ${'a.'.repeat(200)}`],
  ['a repeat count over 1000', '/conformance mutate/a{1001}'],
  ['a repeat range over 1000', '/conformance mutate/a{1,1001}'],
  ['a nested repetition', '/conformance mutate/a**'],
  ['a flag group', '/conformance mutate/(?i)abc'],
]) {
  test(`rejects ${name}`, async () => {
    const { outputs, failure } = await plan(body);
    assert.equal(outputs.valid, 'false');
    assert.ok(failure);
    assert.match(outputs.error, /Usage:/);
    assert.equal(outputs.sha, undefined);
  });
}

test('requires write permission from the author', async () => {
  const { outputs } = await plan('/conformance mutate', { permission: 'triage' });
  assert.equal(outputs.valid, 'false');
  assert.match(outputs.error, /@maintainer needs repository write permission/);
});

test('requires write permission from the editor of the comment', async () => {
  const { outputs, asked } = await plan('/conformance mutate', { sender: 'outsider' });
  assert.deepEqual(asked, ['maintainer', 'outsider']);
  assert.equal(outputs.valid, 'false');
  assert.match(outputs.error, /@outsider/);
});

test('rejects a closed pull request', async () => {
  const { outputs } = await plan('/conformance mutate', { state: 'closed' });
  assert.equal(outputs.valid, 'false');
});

async function comment(environment) {
  const core = fakeCore();
  core.summary = { addRaw() { return this; }, write: async () => {} };
  const bodies = [];
  const github = { rest: { issues: { createComment: async ({ body }) => { bodies.push(body); } } } };
  const context = { repo: { owner: 'o', repo: 'r' }, issue: { number: 7 }, serverUrl: 'https://github.com', runId: 42 };
  for (const key of ['VALID', 'ERROR', 'SHA', 'K8S_SUMMARY', 'DESCRIPTION', 'REGEX', 'COUNTS',
    'IMAGES_RESULT', 'CLI_RESULT', 'TESTS_RESULT']) {
    process.env[key] = environment[key] ?? '';
  }
  await runScript(report, { core, github, context });
  return bodies[0];
}

const success = { IMAGES_RESULT: 'success', CLI_RESULT: 'success', TESTS_RESULT: 'success' };

test('reports a pass with the tested commit, the counts, and a link to the run', async () => {
  const body = await comment({
    VALID: 'true', SHA: HEAD, K8S_SUMMARY: 'Kubernetes v1.37.0', DESCRIPTION: 'suite `mutate`',
    COUNTS: '{"failed": 0, "missing": 0, "passed": 12, "skipped": 1}', ...success,
  });
  assert.match(body, /passed for suite `mutate` on Kubernetes v1.37.0/);
  assert.ok(body.includes(`at ${HEAD}.`));
  assert.match(body, /12 passed, 0 failed, 1 skipped/);
  assert.match(body, /https:\/\/github.com\/o\/r\/actions\/runs\/42/);
});

test('reports a regex that matched no test', async () => {
  const body = await comment({
    VALID: 'true', SHA: HEAD, REGEX: 'nomatch', DESCRIPTION: 'tests matching `nomatch` in suite `mutate`',
    COUNTS: '{"failed": 0, "missing": 0, "passed": 0, "skipped": 0}', ...success,
  });
  assert.match(body, /:warning: Conformance ran no tests: no test matched the regex/);
  assert.doesNotMatch(body, /passed for/);
});

test('reports a pass of a suite without chainsaw reports', async () => {
  const body = await comment({
    VALID: 'true', SHA: HEAD, DESCRIPTION: 'suite `helm-tests`',
    COUNTS: '{"failed": 0, "missing": 0, "passed": 0, "skipped": 0}', ...success,
  });
  assert.match(body, /passed/);
});

test('reports a build failure', async () => {
  const body = await comment({ VALID: 'true', SHA: HEAD, IMAGES_RESULT: 'failure', CLI_RESULT: 'success', TESTS_RESULT: 'skipped' });
  assert.match(body, /:x: Conformance failed/);
});

test('reports a rejected command', async () => {
  const body = await comment({ VALID: 'false', ERROR: 'Unknown suite `x`.' });
  assert.match(body, /was rejected: Unknown suite/);
});

test('reports a plan job that failed without an error', async () => {
  const body = await comment({});
  assert.match(body, /couldn't be validated/);
});
