// Tests for the full-matrix label of the tracking issue in
// .github/workflows/check-tests.yaml: the "Keep full-matrix failures open"
// step of sync-issue and the full-matrix-label job.
// Run with: node .github/tests/check-tests-full-matrix.test.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { fakeCore, runScript } from './inline-script.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

/** The inline script of the first github-script step with the given name. */
function stepScript(file, name) {
  const lines = fs.readFileSync(path.join(root, file), 'utf8').split('\n');
  const step = lines.findIndex(l => l.trim() === `- name: ${name}`);
  assert.ok(step >= 0, `no step named ${name}`);
  const start = lines.findIndex((l, i) => i > step && l.trimEnd().endsWith('script: |'));
  const indent = lines[start + 1].match(/^ */)[0].length;
  const out = [];
  for (let i = start + 1; i < lines.length; i++) {
    if (lines[i].trim() !== '' && lines[i].match(/^ */)[0].length < indent) {
      break;
    }
    out.push(lines[i].slice(indent));
  }
  return out.join('\n');
}

const FILE = '.github/workflows/check-tests.yaml';
const keepOpen = stepScript(FILE, 'Keep full-matrix failures open');
const label = stepScript(FILE, 'Label full-matrix failures');

const MARKER = '<!-- workflow-failure:Tests:refs/heads/main -->';
const RUN_URL = 's/o/r/actions/runs/1';
const issue = (number, labels, body = `[\`1\`](${RUN_URL})\n${MARKER}`) =>
  ({ number, body, labels: labels.map(name => ({ name })) });
const job = (name, conclusion = 'success') =>
  ({ name, status: 'completed', conclusion, html_url: `job/${name}` });
const VERSIONS = ['v1.34.11', 'v1.35.8', 'v1.36.4', 'v1.37.0'];
const matrixJobs = (failing = {}) => [
  job('Build test images'),
  job('Conformance tests / kubernetes-matrix / matrix'),
  ...VERSIONS.flatMap(v => [
    job(`Conformance tests / assert (${v})`, failing[v] || 'success'),
    job(`Conformance tests / sharded (${v}, 0)`),
  ]),
  // Not part of the conformance-tests job, so its versions don't count.
  job('Conformance tests (policy library) / run (v1.30.0)', 'failure'),
];

async function run(script, { open = [], jobs = [], labelExists = false }) {
  const calls = { addLabels: [], removeLabel: [], createLabel: [], comment: [] };
  const core = fakeCore();
  const outputs = {};
  core.setOutput = (k, v) => { outputs[k] = v; };
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: {
        listJobsForWorkflowRun: async a => {
          assert.equal(a.run_id, 1);
          return { data: jobs };
        },
      },
      issues: {
        listForRepo: async a => {
          assert.equal(a.labels, 'e2e-failure');
          return { data: open };
        },
        createLabel: async a => {
          calls.createLabel.push(a);
          if (labelExists) throw Object.assign(new Error('exists'), { status: 422 });
        },
        addLabels: async a => { calls.addLabels.push(a); },
        removeLabel: async a => { calls.removeLabel.push(a); },
        createComment: async a => { calls.comment.push(a); },
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 1,
    ref: 'refs/heads/main', serverUrl: 's',
  };
  process.env.CONFORMANCE_JOB = 'Conformance tests';
  await runScript(script, { github, core, context });
  return { calls, outputs, log: core.log };
}

test('keep open: a reduced green run skips the issue that has the full-matrix label', async () => {
  const { outputs, log } = await run(keepOpen, { open: [issue(5, ['workflow-failure', 'e2e-failure', 'full-matrix'])] });
  assert.equal(outputs.skip, 'true');
  assert.ok(log.some(l => l.startsWith('notice:') && l.includes('#5')));
});

test('keep open: an issue without the label, or for another branch, is not skipped', async () => {
  for (const open of [
    [issue(5, ['e2e-failure'])],
    [issue(6, ['e2e-failure', 'full-matrix'], '<!-- workflow-failure:Tests:refs/heads/release-1.20 -->')],
  ]) {
    const { outputs } = await run(keepOpen, { open });
    assert.equal(outputs.skip, undefined);
  }
});

test('label: a failure between the oldest and the latest version labels the issue once', async () => {
  const { calls } = await run(label, {
    open: [issue(5, ['e2e-failure'])],
    jobs: matrixJobs({ 'v1.35.8': 'failure', 'v1.36.4': 'failure', 'v1.37.0': 'failure' }),
  });
  assert.equal(calls.createLabel[0].name, 'full-matrix');
  assert.deepEqual(calls.addLabels, [{ owner: 'o', repo: 'r', issue_number: 5, labels: ['full-matrix'] }]);
  assert.equal(calls.comment.length, 1);
  const body = calls.comment[0].body;
  assert.ok(body.includes('failed on Kubernetes v1.35, v1.36,'));
  assert.ok(body.includes('`Conformance tests / assert (v1.35.8)`'));
  assert.ok(!body.includes('v1.37.0'), 'a reduced run tests the latest version');
});

test('label: an existing label is not created again', async () => {
  const { calls, log } = await run(label, {
    open: [issue(5, ['e2e-failure'])], jobs: matrixJobs({ 'v1.35.8': 'failure' }), labelExists: true,
  });
  assert.equal(calls.addLabels.length, 1);
  assert.ok(!log.some(l => l.startsWith('warning:')));
});

test('label: an issue that already has the label gets no new comment', async () => {
  const { calls } = await run(label, {
    open: [issue(5, ['e2e-failure', 'full-matrix'])], jobs: matrixJobs({ 'v1.35.8': 'failure' }),
  });
  assert.deepEqual(calls, { addLabels: [], removeLabel: [], createLabel: [], comment: [] });
});

test('label: failures only on what a reduced run tests remove the label', async () => {
  const { calls } = await run(label, {
    open: [issue(5, ['e2e-failure', 'full-matrix'])],
    jobs: matrixJobs({ 'v1.34.11': 'failure', 'v1.37.0': 'failure' }),
  });
  assert.deepEqual(calls.removeLabel, [{ owner: 'o', repo: 'r', issue_number: 5, name: 'full-matrix' }]);
  assert.equal(calls.comment.length, 1);
  assert.equal(calls.addLabels.length, 0);
});

test('label: failures only on what a reduced run tests leave an unlabelled issue alone', async () => {
  const { calls } = await run(label, { open: [issue(5, ['e2e-failure'])], jobs: matrixJobs({ 'v1.34.11': 'failure' }) });
  assert.deepEqual(calls, { addLabels: [], removeLabel: [], createLabel: [], comment: [] });
});

test('label: an issue that does not show this run is left alone', async () => {
  const { calls, log } = await run(label, {
    open: [issue(5, ['e2e-failure'], `[\`2\`](s/o/r/actions/runs/2)\n${MARKER}`)],
    jobs: matrixJobs({ 'v1.35.8': 'failure' }),
  });
  assert.deepEqual(calls, { addLabels: [], removeLabel: [], createLabel: [], comment: [] });
  assert.ok(log.some(l => l.includes('nothing to label')));
});

test('label: without versioned conformance jobs the labels stay as they are', async () => {
  const { calls, log } = await run(label, {
    open: [issue(5, ['e2e-failure', 'full-matrix'])], jobs: [job('Build test images', 'failure')],
  });
  assert.deepEqual(calls, { addLabels: [], removeLabel: [], createLabel: [], comment: [] });
  assert.ok(log.some(l => l.startsWith('warning:')));
});

test('keep open: if the check fails, Sync issue does not run', () => {
  const lines = fs.readFileSync(path.join(root, FILE), 'utf8').split('\n');
  const step = lines.findIndex(l => l.trim() === '- name: Sync issue');
  const cond = lines.slice(step + 1, step + 6).join('\n');
  assert.match(cond, /steps\.full-matrix-open\.outcome != 'failure'/);
  assert.match(cond, /steps\.full-matrix-open\.outputs\.skip != 'true'/);
  const guard = lines.findIndex(l => l.trim() === 'id: full-matrix-open');
  assert.equal(lines[guard + 1].trim(), 'continue-on-error: true');
});

test('label: a timed-out job between the oldest and the latest version counts as a failure', async () => {
  const { calls } = await run(label, {
    open: [issue(5, ['e2e-failure'])], jobs: matrixJobs({ 'v1.35.8': 'timed_out' }),
  });
  assert.equal(calls.addLabels.length, 1);
});

test('label: a cancelled job between the oldest and the latest version neither adds nor removes the label', async () => {
  for (const labels of [['e2e-failure'], ['e2e-failure', 'full-matrix']]) {
    const { calls } = await run(label, {
      open: [issue(5, labels)], jobs: matrixJobs({ 'v1.34.11': 'failure', 'v1.36.4': 'cancelled' }),
    });
    assert.deepEqual(calls, { addLabels: [], removeLabel: [], createLabel: [], comment: [] }, labels.join());
  }
});
