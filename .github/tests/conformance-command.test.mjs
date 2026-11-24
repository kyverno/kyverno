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

test('a bare command runs every suite on the latest version', async () => {
  const { outputs } = await plan('/conformance');
  assert.equal(outputs.valid, 'true');
  assert.equal(outputs.sha, HEAD);
  assert.equal(outputs['k8s-version'], 'v1.37.0');
  assert.equal(outputs['chainsaw-tests'], '');
  // The v1beta1-only suite doesn't run on v1.37.
  assert.deepEqual(JSON.parse(outputs.suites), ['assert', 'mutate', 'cel-http', 'helm-tests']);
});

test('a suite name selects one suite', async () => {
  const { outputs } = await plan('/conformance mutate\r\nplease');
  assert.equal(outputs.valid, 'true');
  assert.deepEqual(JSON.parse(outputs.suites), ['mutate']);
  assert.equal(outputs.description, 'suite `mutate`');
});

test('a suite defaults to the latest version it supports', async () => {
  const { outputs } = await plan('/conformance generate-mutating-admission-policy');
  assert.equal(outputs['k8s-version'], 'v1.35.8');
});

test('a suite on a version it does not support is rejected', async () => {
  const { outputs } = await plan('/conformance generate-mutating-admission-policy v1.37');
  assert.equal(outputs.valid, 'false');
  assert.match(outputs.error, /doesn't run on Kubernetes `v1.37.0`/);
});

test('suite/regex scopes the regex to the suite', async () => {
  const { outputs } = await plan('/conformance mutate/foreach-.* 1.36');
  assert.equal(outputs.valid, 'true');
  assert.deepEqual(JSON.parse(outputs.suites), ['mutate']);
  assert.equal(outputs['chainsaw-tests'], 'foreach-.*');
  assert.equal(outputs['k8s-version'], 'v1.36.4');
});

test('a bare regex runs every chainsaw suite', async () => {
  const { outputs } = await plan('/conformance ^cel.*$ v1.34.11');
  assert.equal(outputs.valid, 'true');
  assert.equal(outputs['chainsaw-tests'], '^cel.*$');
  assert.deepEqual(JSON.parse(outputs.suites), ['assert', 'mutate', 'cel-http', 'generate-mutating-admission-policy']);
});

test('a version alone runs every suite on that version', async () => {
  const { outputs } = await plan('/conformance v1.35');
  assert.equal(outputs['k8s-version'], 'v1.35.8');
  assert.ok(JSON.parse(outputs.suites).includes('generate-mutating-admission-policy'));
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
  for (const key of ['VALID', 'ERROR', 'SHA', 'K8S_VERSION', 'DESCRIPTION', 'IMAGES_RESULT', 'CLI_RESULT', 'TESTS_RESULT']) {
    process.env[key] = environment[key] ?? '';
  }
  await runScript(report, { core, github, context });
  return bodies[0];
}

test('reports a pass with a link to the run', async () => {
  const body = await comment({
    VALID: 'true', SHA: HEAD, K8S_VERSION: 'v1.37.0', DESCRIPTION: 'suite `mutate`',
    IMAGES_RESULT: 'success', CLI_RESULT: 'success', TESTS_RESULT: 'success',
  });
  assert.match(body, /passed for suite `mutate` on Kubernetes v1.37.0/);
  assert.match(body, /https:\/\/github.com\/o\/r\/actions\/runs\/42/);
});

test('reports a build failure', async () => {
  const body = await comment({ VALID: 'true', IMAGES_RESULT: 'failure', CLI_RESULT: 'success', TESTS_RESULT: 'skipped' });
  assert.match(body, /failed/);
});

test('reports a rejected command', async () => {
  const body = await comment({ VALID: 'false', ERROR: 'Unknown suite `x`.' });
  assert.match(body, /was rejected: Unknown suite/);
});

test('reports a plan job that failed without an error', async () => {
  const body = await comment({});
  assert.match(body, /couldn't be validated/);
});
