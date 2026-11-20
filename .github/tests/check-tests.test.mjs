// Tests for the scripts of .github/workflows/check-tests.yaml: the batch
// summary, and the steps of sync-issue that decide what happens to the
// tracking issue.
// Run with: node .github/tests/check-tests.test.mjs
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { fakeCore, inlineScript, runScript } from './inline-script.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const WORKFLOW = '.github/workflows/check-tests.yaml';
const workflow = read(WORKFLOW);

/** The lines of a `key: |` block that starts at line `start`. */
function block(lines, start) {
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

/** The inline script of the step with the given name. The file has several. */
function stepScript(name) {
  const lines = workflow.split('\n');
  const step = lines.findIndex(l => l.trim() === `- name: ${name}`);
  assert.ok(step >= 0, `no step named ${name}`);
  const next = lines.findIndex((l, i) => i > step && /^\s*- name: /.test(l));
  const start = lines.findIndex((l, i) => i > step && l.trimEnd().endsWith('script: |'));
  assert.ok(start >= 0 && (next < 0 || start < next), `step ${name} has no inline script`);
  return block(lines, start);
}

/** The text of one job of the workflow. */
function jobText(id) {
  const lines = workflow.split('\n');
  const start = lines.findIndex(l => l === `  ${id}:`);
  assert.ok(start >= 0, `no job ${id}`);
  const end = lines.findIndex((l, i) => i > start && /^ {2}[\w-]+:$/.test(l));
  return lines.slice(start, end < 0 ? lines.length : end).join('\n');
}

/** A `core` stub that also records outputs and the job summary. */
function core() {
  const stub = fakeCore();
  stub.outputs = {};
  stub.setOutput = (name, value) => { stub.outputs[name] = String(value); };
  stub.summaryText = '';
  const add = text => { stub.summaryText += `${text}\n`; return stub.summary; };
  stub.summary = {
    addHeading: add, addRaw: add,
    addLink: (text, href) => add(`${text} ${href}`),
    addTable: rows => add(JSON.stringify(rows)),
    write: async () => {},
  };
  return stub;
}

const withEnv = async (env, fn) => {
  const saved = { ...process.env };
  Object.assign(process.env, env);
  try {
    return await fn();
  } finally {
    for (const key of Object.keys(env)) {
      if (key in saved) process.env[key] = saved[key]; else delete process.env[key];
    }
  }
};

// ── Test results ────────────────────────────────────────────────────────────

const TEST_JOBS = ['images', 'cli', 'conformance-tests', 'conformance-policy-library-tests', 'k6-tests', 'perf'];
/** The needs context: every test job passed unless `results` says otherwise. */
const needs = (results = {}, batch = 'success') => ({
  ...Object.fromEntries(TEST_JOBS.map(id => [id, { result: results[id] || 'success', outputs: {} }])),
  batch: { result: batch, outputs: { 'issue-note': '### Suspect commits\n\n@alice' } },
});

async function testResults(needsContext) {
  const c = core();
  await withEnv({ NEEDS: JSON.stringify(needsContext) }, () =>
    runScript(stepScript('Test results'), { core: c, process }));
  return c.outputs;
}

test('test results: a failed, cancelled or skipped batch job never counts', async () => {
  for (const batch of ['failure', 'cancelled', 'skipped', 'success']) {
    const out = await testResults(needs({}, batch));
    assert.deepEqual([out.failed, out.cancelled], ['false', 'false'], `batch ${batch}`);
  }
  // Nor does it hide what the tests did.
  const failed = await testResults(needs({ 'conformance-tests': 'failure' }, 'cancelled'));
  assert.deepEqual([failed.failed, failed.cancelled], ['true', 'false']);
  const cancelled = await testResults(needs({ perf: 'cancelled' }, 'failure'));
  assert.deepEqual([cancelled.failed, cancelled.cancelled], ['false', 'true']);
});

test('test results: every other needed job counts, also one added later', async () => {
  const out = await testResults({ ...needs(), 'new-tests': { result: 'failure' } });
  assert.equal(out.failed, 'true');
  assert.equal((await testResults(needs({ images: 'failure', cli: 'cancelled' }))).cancelled, 'true');
});

test('test results: the job results table lists the test jobs only, without outputs', async () => {
  const out = await testResults(needs({ 'k6-tests': 'failure' }, 'failure'));
  assert.deepEqual(JSON.parse(out.needs), Object.fromEntries(
    TEST_JOBS.map(id => [id, { result: id === 'k6-tests' ? 'failure' : 'success' }])));
  assert.ok(!out.needs.includes('alice'));
});

// ── The workflow around the scripts ─────────────────────────────────────────

test('sync-issue needs every test job and batch, and no step reads needs.*.result', () => {
  const job = jobText('sync-issue');
  const needed = /\n {4}needs:\n((?: {6}.*\n)+)/.exec(job)[1]
    .split('\n').map(l => l.trim()).filter(l => l.startsWith('- ')).map(l => l.slice(2));
  assert.deepEqual(needed, [...TEST_JOBS, 'batch']);
  const [jobLevel, steps] = job.split('\n    steps:\n');
  const code = steps.split('\n').filter(l => !l.trim().startsWith('#') && !l.trim().startsWith('//'));
  assert.deepEqual(code.filter(l => l.includes('needs.*')), [], 'steps read the Test results outputs');
  // The job condition does read it, so a cancelled batch job needs its own clause.
  assert.ok(jobLevel.includes("needs.batch.result == 'cancelled'"));
});

test('sync-issue passes the planned values to the failure-issue action', () => {
  const job = jobText('sync-issue');
  const sync = job.slice(job.indexOf('- name: Sync issue'));
  assert.ok(sync.includes("if: steps.defer.outputs.deferred != 'true' && steps.plan.outputs.sync == 'true'"));
  assert.ok(sync.includes("conclusion: ${{ steps.tests.outputs.failed == 'true' && 'failure' || job.status }}"));
  assert.ok(sync.includes('needs: ${{ steps.tests.outputs.needs }}'));
  assert.ok(sync.includes("labels: e2e-failure${{ steps.plan.outputs.full-matrix == 'true' && ',full-matrix' || '' }}"));
  assert.ok(sync.includes('${{ steps.plan.outputs.note }}'));
});

test('the first attempt of a failing run still defers, by the step name tests-rerun.yaml looks for', () => {
  const job = jobText('sync-issue');
  const defer = job.slice(job.indexOf('- name: Defer to the automatic re-run'), job.indexOf('- name: Plan the issue update'));
  assert.ok(defer.includes('id: defer'));
  assert.ok(defer.includes("github.run_attempt == '1' &&"));
  assert.ok(defer.includes("steps.tests.outputs.failed == 'true' &&"));
  assert.ok(defer.includes("steps.tests.outputs.cancelled != 'true'"));
  assert.ok(read('.github/workflows/tests-rerun.yaml').includes('select(.name == "Defer to the automatic re-run" and .conclusion == "success")'));
  // A deferred attempt plans nothing and leaves the issue alone.
  const plan = job.slice(job.indexOf('- name: Plan the issue update'), job.indexOf('- name: Sync issue'));
  assert.ok(plan.includes("if: steps.defer.outputs.deferred != 'true'"));
});

test('the job names in the plan script are the ones the workflow uses', () => {
  const plan = stepScript('Plan the issue update');
  const matrixJob = /const MATRIX_JOB = '(.+)';/.exec(plan)[1];
  const batchJob = /const BATCH_JOB = '(.+)';/.exec(plan)[1];
  assert.ok(jobText('batch').includes(`\n    name: ${batchJob}\n`));
  // Only one job passes the matrix input on, and it is the one the script names.
  const passing = [...workflow.matchAll(/^ {2}([\w-]+):$/gm)].map(m => m[1])
    .filter(id => /\n {6}matrix: \$\{\{ inputs\.matrix/.test(jobText(id)));
  assert.deepEqual(passing, ['conformance-tests']);
  assert.ok(jobText('conformance-tests').includes(`\n    name: ${matrixJob}\n`));
});

test('the full-matrix label is declared', () => {
  assert.match(read('.github/labels.yml'), /\nfull-matrix:\n {2}color: [0-9A-Fa-f]{6}\n {2}description: .{1,100}\n/);
});

// ── Plan the issue update ───────────────────────────────────────────────────

const VERSIONS = ['v1.34.11', 'v1.35.8', 'v1.36.4', 'v1.37.0'];
const job = (name, conclusion = 'success') => ({ name, status: 'completed', conclusion });
/** The jobs of a full run, named as the jobs API names them. `failed` names the failed ones. */
const fullRun = (failed = []) => [
  'Batch summary', 'Build test images', 'Build test CLI', 'Performance benchmarks',
  'K6 tests / k6 (simple)', 'K6 tests / summary',
  'Conformance tests / kubernetes-matrix / matrix',
  'Conformance tests (policy library) / kubernetes-matrix / matrix',
  ...VERSIONS.map(v => `Conformance tests (policy library) / policy-library (${v}, 3)`),
  ...VERSIONS.flatMap(v => [
    `Conformance tests / assert (${v})`,
    `Conformance tests / reports (${v}, 2)`,
    `Conformance tests / rbac (standard, ${v})`,
    `Conformance tests / helm-upgrade (${v}, 3.6)`,
  ]),
  'Conformance tests / mutating-admission-policy-reports-v1beta1 (v1.34.11)',
  'Conformance tests / mutating-admission-policy-reports-v1beta1 (v1.35.8)',
  'Conformance tests / mutating-admission-policy-reports-v1 (v1.36.4)',
  'Conformance tests / mutating-admission-policy-reports-v1 (v1.37.0)',
  'Conformance tests / check-tests (v1.37.0)',
  'Conformance tests / v1.37.0 - cert-manager rsa',
].map(name => job(name, failed.includes(name) ? 'failure' : 'success'))
  .concat([{ name: 'sync-issue', status: 'in_progress', conclusion: null }]);

const marker = branch => `body\n<!-- workflow-failure:Tests:refs/heads/${branch} -->`;
const issue = (number, labels, branch = 'main', extra = {}) =>
  ({ number, body: marker(branch), labels: labels.map(name => ({ name })), ...extra });
const PLAIN = ['workflow-failure', 'e2e-failure'];
const HELD = [...PLAIN, 'full-matrix'];

/**
 * @param jobs         the jobs of the run
 * @param open         the open issues
 * @param emptyLists   labels whose list comes back empty although issues carry them
 */
async function plan({
  matrix = 'full', failed = false, cancelled = false, branch = 'main',
  jobs = fullRun(), jobsError = false, open = [], emptyLists = [], listError = false,
  batchNote = '### Suspect commits\n\n@alice: please check.', batchResult = 'success',
}) {
  const c = core();
  const lists = [];
  let jobReads = 0;
  const forbidden = () => { throw new Error('the plan step must not write'); };
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: {
        listJobsForWorkflowRun: async a => {
          jobReads++;
          assert.deepEqual([a.run_id, a.filter], [77, 'latest']);
          if (jobsError) throw new Error('Bad Gateway');
          return { data: jobs };
        },
      },
      issues: {
        listForRepo: async a => {
          assert.equal(a.state, 'open');
          lists.push(a.labels);
          if (listError) throw new Error('Bad Gateway');
          return { data: emptyLists.includes(a.labels) ? [] : open.filter(i => i.labels.some(l => l.name === a.labels)) };
        },
        update: forbidden, createComment: forbidden, addLabels: forbidden, removeLabel: forbidden, create: forbidden,
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 77,
    ref: `refs/heads/${branch}`, serverUrl: 'https://github.com',
  };
  await withEnv({
    MATRIX: matrix, FAILED: String(failed), CANCELLED: String(cancelled),
    BATCH_NOTE: batchNote, BATCH_RESULT: batchResult,
  }, () => runScript(stepScript('Plan the issue update'), { github, core: c, context, process }));
  return {
    sync: c.outputs.sync === 'true', fullMatrix: c.outputs['full-matrix'] === 'true',
    note: c.outputs.note, log: c.log, lists, jobReads,
  };
}
const fails = (...names) => ({ failed: true, jobs: fullRun(names) });

test('full run fails only on versions in between: the issue gets full-matrix', async () => {
  const r = await plan(fails('Conformance tests / assert (v1.35.8)', 'Conformance tests / reports (v1.36.4, 2)'));
  assert.deepEqual([r.sync, r.fullMatrix], [true, true]);
  assert.ok(r.note.includes('This run failed only on v1.35.8, v1.36.4, which reduced runs skip'));
  assert.ok(r.log.some(l => l.startsWith('notice:') && l.includes('full-matrix')));
  assert.deepEqual(r.lists, [], 'a failing run does not read the issue');
});

test('full run fails on both kinds of versions: no label', async () => {
  for (const covered of ['Conformance tests / assert (v1.34.11)', 'Conformance tests / rbac (standard, v1.37.0)']) {
    const r = await plan(fails('Conformance tests / assert (v1.35.8)', covered));
    assert.deepEqual([r.sync, r.fullMatrix], [true, false], covered);
    assert.ok(!r.note.includes('This run failed only on'));
  }
});

test('a failed job without a Kubernetes version runs in both matrices: no label', async () => {
  for (const name of ['Build test images', 'Build test CLI', 'Performance benchmarks', 'K6 tests / k6 (simple)',
    'Conformance tests / kubernetes-matrix / matrix']) {
    const alone = await plan(fails(name));
    assert.equal(alone.fullMatrix, false, name);
    const mixed = await plan(fails(name, 'Conformance tests / assert (v1.35.8)'));
    assert.equal(mixed.fullMatrix, false, `${name} with a version in between`);
  }
});

test('the policy library runs every version in both matrices: no label', async () => {
  const r = await plan(fails('Conformance tests (policy library) / policy-library (v1.35.8, 3)'));
  assert.equal(r.fullMatrix, false);
});

test('jobs that read the beta, stable or smoke list follow the same rule', async () => {
  const label = async name => (await plan(fails(name))).fullMatrix;
  // beta: v1.34.11 is the oldest version, v1.35.8 is in between.
  assert.equal(await label('Conformance tests / mutating-admission-policy-reports-v1beta1 (v1.34.11)'), false);
  assert.equal(await label('Conformance tests / mutating-admission-policy-reports-v1beta1 (v1.35.8)'), true);
  // stable: v1.36.4 is in between, v1.37.0 is the latest version.
  assert.equal(await label('Conformance tests / mutating-admission-policy-reports-v1 (v1.36.4)'), true);
  assert.equal(await label('Conformance tests / mutating-admission-policy-reports-v1 (v1.37.0)'), false);
  // smoke: the latest version, in both matrices.
  assert.equal(await label('Conformance tests / check-tests (v1.37.0)'), false);
  assert.equal(await label('Conformance tests / v1.37.0 - cert-manager rsa'), false);
});

test('a failed batch job is not a test failure and does not change the label', async () => {
  assert.equal((await plan(fails('Batch summary', 'Conformance tests / assert (v1.36.4)'))).fullMatrix, true);
  assert.equal((await plan(fails('Batch summary', 'Conformance tests / assert (v1.37.0)'))).fullMatrix, false);
});

test('versions are compared as numbers', async () => {
  const jobs = ['v1.9.3', 'v1.10.1', 'v1.11.0'].map(v => job(`Conformance tests / assert (${v})`));
  jobs[1].conclusion = 'failure';
  assert.equal((await plan({ failed: true, jobs })).fullMatrix, true);
  jobs[1].conclusion = 'success';
  jobs[0].conclusion = 'failure';
  assert.equal((await plan({ failed: true, jobs })).fullMatrix, false);
});

test('the jobs of a failing full run cannot be read: the issue gets the label, to be safe', async () => {
  const unreadable = await plan({ failed: true, jobsError: true });
  assert.deepEqual([unreadable.sync, unreadable.fullMatrix], [true, true]);
  assert.ok(unreadable.log.some(l => l.startsWith('warning:') && l.includes('Bad Gateway')));
  assert.match(unreadable.note, /jobs of this run could not be read/);
  assert.match(unreadable.note, /with matrix `full`/);
  // A reduced run doesn't read the jobs, so an unreadable list changes nothing there.
  const reduced = await plan({ matrix: 'reduced', failed: true, jobsError: true });
  assert.deepEqual([reduced.sync, reduced.fullMatrix, reduced.jobReads], [true, false, 0]);
});

test('no failed job in the list: no label, and the failure is still recorded', async () => {
  const none = await plan({ failed: true, jobs: fullRun() });
  assert.deepEqual([none.sync, none.fullMatrix], [true, false]);
  assert.doesNotMatch(none.note, /could not be read\. This issue has the label/);
});

test('the job list is read in small pages, with retries', () => {
  assert.match(stepScript('Plan the issue update'), /listJobsForWorkflowRun, \{\s+owner, repo, run_id: context\.runId, filter: 'latest', per_page: 30\s+\}/);
  const step = read(WORKFLOW).split('- name: Plan the issue update')[1].split('- name: Sync issue')[0];
  assert.match(step, /\n {10}retries: 3\n/);
});

test('a reduced run that fails never adds the label, and updates an issue that has it', async () => {
  // The run has jobs on the oldest and the latest version only.
  const jobs = fullRun(['Conformance tests / assert (v1.34.11)']).filter(j => !/v1\.3[56]\./.test(j.name));
  const r = await plan({ matrix: 'reduced', failed: true, jobs, open: [issue(5, HELD)] });
  assert.deepEqual([r.sync, r.fullMatrix], [true, false]);
  assert.equal(r.jobReads, 0);
  // The action never removes a label, so the issue keeps full-matrix. The
  // note explains the label in every failure body, whoever set it.
  assert.ok(r.note.includes('### The `full-matrix` label'));
  assert.ok(r.note.includes('a passing reduced run leaves the issue open'));
});

test('the note tells how to clear a full-matrix issue', async () => {
  for (const r of [await plan(fails('Conformance tests / assert (v1.35.8)')), await plan({ matrix: 'reduced', failed: true })]) {
    assert.ok(r.note.includes('[Run workflow](https://github.com/o/r/actions/workflows/check-tests.yaml) on `main` with matrix `full`'));
  }
});

test('reduced run passes, the issue has full-matrix: left open, without a write', async () => {
  const r = await plan({ matrix: 'reduced', open: [issue(5, HELD)] });
  assert.equal(r.sync, false);
  assert.equal(r.note, '', 'nobody is mentioned by a run that passed');
  const notice = r.log.find(l => l.startsWith('notice:'));
  assert.ok(notice.includes('#5') && notice.includes('full-matrix') && notice.includes('matrix full'));
});

test('reduced run passes, the issue has no full-matrix: closed as before', async () => {
  const r = await plan({ matrix: 'reduced', open: [issue(5, PLAIN)] });
  assert.equal(r.sync, true);
  assert.equal(r.note, '');
});

test('full run passes: closes the issue, with or without full-matrix, and reads nothing', async () => {
  for (const labels of [HELD, PLAIN]) {
    const r = await plan({ matrix: 'full', open: [issue(5, labels)] });
    assert.equal(r.sync, true);
    assert.deepEqual([r.lists, r.jobReads], [[], 0]);
    assert.equal(r.note, '');
  }
});

test('reduced run passes: only an issue of the same branch holds it', async () => {
  // release-1.2 is a prefix of release-1.20, and a pull request can carry the label too.
  const others = [issue(6, HELD, 'release-1.20'), issue(7, HELD, 'main', { pull_request: {} })];
  assert.equal((await plan({ matrix: 'reduced', branch: 'release-1.2', open: [...others, issue(5, PLAIN, 'release-1.2')] })).sync, true);
  assert.equal((await plan({ matrix: 'reduced', branch: 'release-1.20', open: [...others, issue(5, PLAIN, 'release-1.2')] })).sync, false);
  assert.equal((await plan({ matrix: 'reduced', branch: 'main', open: [...others, issue(5, PLAIN)] })).sync, true);
});

test('reduced run passes, two issues for the branch and one has full-matrix: both stay', async () => {
  const r = await plan({ matrix: 'reduced', open: [issue(5, PLAIN), issue(6, HELD)] });
  assert.equal(r.sync, false);
});

test('an empty issue list never closes anything', async () => {
  // Both lists come back empty although the issue is open.
  const both = await plan({ matrix: 'reduced', open: [issue(5, HELD)], emptyLists: ['workflow-failure', 'full-matrix'] });
  assert.equal(both.sync, false);
  assert.deepEqual(both.lists, ['workflow-failure', 'full-matrix']);
  assert.ok(both.log.some(l => l.startsWith('info:') && l.includes('nothing to close')));
  // One list is enough to see the label.
  for (const label of ['workflow-failure', 'full-matrix']) {
    const one = await plan({ matrix: 'reduced', open: [issue(5, HELD)], emptyLists: [label] });
    assert.equal(one.sync, false, `empty ${label} list`);
    assert.ok(one.log.some(l => l.startsWith('notice:') && l.includes('#5')));
  }
  // The same holds for an issue without the label: it is closed by the next run that sees it.
  assert.equal((await plan({ matrix: 'reduced', open: [issue(5, PLAIN)], emptyLists: ['workflow-failure'] })).sync, false);
  assert.equal((await plan({ matrix: 'reduced', open: [] })).sync, false);
});

test('an issue whose workflow-failure label was removed still holds a reduced run', async () => {
  const r = await plan({ matrix: 'reduced', open: [issue(5, ['e2e-failure', 'full-matrix'])] });
  assert.equal(r.sync, false);
});

test('reduced run passes, the issues cannot be read: left as it is', async () => {
  const r = await plan({ matrix: 'reduced', open: [issue(5, PLAIN)], listError: true });
  assert.equal(r.sync, false);
  assert.ok(r.log.some(l => l.startsWith('warning:') && l.includes('Bad Gateway')));
});

test('a run cancelled without a test failure leaves the issue as it is', async () => {
  for (const matrix of ['full', 'reduced']) {
    const r = await plan({ matrix, cancelled: true, open: [issue(5, PLAIN)] });
    assert.equal(r.sync, false);
    assert.deepEqual([r.lists, r.jobReads], [[], 0]);
  }
  // A failure is recorded even if other jobs were then cancelled.
  assert.equal((await plan({ failed: true, cancelled: true })).sync, true);
});

test('the second attempt is judged by the jobs that still fail after the re-run', async () => {
  // First attempt: a flaky job on the oldest version and a real failure in
  // between. The re-run fixed the flaky one; the jobs API lists the latest
  // attempt of each job.
  const r = await plan(fails('Conformance tests / assert (v1.35.8)'));
  assert.equal(r.fullMatrix, true);
  // A second attempt that passes is a passing run of its matrix.
  assert.equal((await plan({ matrix: 'reduced', open: [issue(5, HELD)] })).sync, false);
  assert.equal((await plan({ matrix: 'full', open: [issue(5, HELD)] })).sync, true);
});

test('release branch: no matrix input, so every run is full and closes the issue', async () => {
  // The workflow passes inputs.matrix || 'full'.
  assert.ok(jobText('sync-issue').includes("MATRIX: ${{ inputs.matrix || 'full' }}"));
  const green = await plan({ matrix: 'full', branch: 'release-1.20', open: [issue(5, HELD, 'release-1.20')] });
  assert.deepEqual([green.sync, green.lists], [true, []]);
  const red = await plan({ ...fails('Conformance tests / assert (v1.36.4)'), branch: 'release-1.20' });
  assert.deepEqual([red.sync, red.fullMatrix], [true, true]);
  assert.ok(red.note.includes('on `release-1.20` with matrix `full`'));
});

test('a failing run puts the batch note first, a passing run has no note', async () => {
  const red = await plan({ matrix: 'reduced', failed: true, batchNote: '### Suspect commits\n\n@alice @bob: check.' });
  assert.ok(red.note.startsWith('### Suspect commits\n\n@alice @bob: check.\n\n### The `full-matrix` label'));
  for (const matrix of ['full', 'reduced']) {
    const green = await plan({ matrix, open: [issue(5, PLAIN)], batchNote: '@alice' });
    assert.equal(green.note, '');
  }
});

test('a batch job that failed, was cancelled or skipped changes nothing but the note', async () => {
  for (const batchResult of ['failure', 'cancelled', 'skipped']) {
    const red = await plan({ ...fails('Conformance tests / assert (v1.35.8)'), batchNote: '', batchResult });
    assert.deepEqual([red.sync, red.fullMatrix], [true, true], batchResult);
    assert.ok(red.note.includes(`The Batch summary job of this run (\`${batchResult}\`) left no list of suspect commits.`));
    assert.ok(!red.note.includes('@'));
    const green = await plan({ matrix: 'reduced', open: [issue(5, PLAIN)], batchNote: '', batchResult });
    assert.deepEqual([green.sync, green.note], [true, '']);
  }
});

// ── The plan, carried out by the failure-issue action ───────────────────────

/**
 * Runs the Sync issue step for a failing run: the action's script, with the
 * inputs the workflow builds from the plan.
 */
async function syncIssue(planned, open) {
  const calls = { create: [], update: [], addLabels: [], comment: [] };
  const github = {
    paginate: async (fn, args) => (await fn(args)).data,
    rest: {
      actions: { listJobsForWorkflowRun: async () => ({ data: [] }) },
      issues: {
        listForRepo: async a => ({
          data: (a.page || 1) > 1 ? [] : open.filter(i => !a.labels || i.labels.some(l => l.name === a.labels)),
        }),
        create: async a => { calls.create.push(a); return { data: { number: 999, html_url: 'u' } }; },
        update: async a => { calls.update.push(a); },
        addLabels: async a => { calls.addLabels.push(a); },
        createComment: async a => { calls.comment.push(a); },
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, workflow: 'Tests', runId: 77, runNumber: 10,
    ref: 'refs/heads/main', serverUrl: 'https://github.com', sha: 'a'.repeat(40), actor: 'me',
  };
  const source = inlineScript('.github/actions/workflow/failure-issue/action.yaml')
    .replace("'${{ inputs.conclusion }}'", "'failure'")
    .replace('${{ inputs.needs }}', '{}');
  await withEnv({
    EXTRA_LABELS: `e2e-failure${planned.fullMatrix ? ',full-matrix' : ''}`,
    NOTE: `${planned.note}\n\n### Clearing the E2E Gate\n`,
  }, () => runScript(source, { github, core: core(), context, Buffer, process }));
  return calls;
}

test('the action adds full-matrix to an open issue and writes the note into its body', async () => {
  const planned = await plan(fails('Conformance tests / assert (v1.35.8)'));
  const calls = await syncIssue(planned, [issue(5, PLAIN)]);
  assert.deepEqual(calls.addLabels.map(a => [a.issue_number, a.labels]), [[5, ['full-matrix']]]);
  const [{ issue_number, body }] = calls.update;
  assert.equal(issue_number, 5);
  const [suspects, label, clearing] = ['### Suspect commits', '### The `full-matrix` label', '### Clearing the E2E Gate'].map(h => body.indexOf(h));
  assert.ok(suspects >= 0 && suspects < label && label < clearing);
  assert.ok(body.includes('@alice'));
  assert.ok(body.includes('<!-- workflow-failure:Tests:refs/heads/main -->'));
});

test('the action opens a new issue with the full-matrix label', async () => {
  const planned = await plan(fails('Conformance tests / assert (v1.35.8)'));
  const calls = await syncIssue(planned, []);
  assert.deepEqual(calls.create.map(a => a.labels), [['workflow-failure', 'e2e-failure', 'full-matrix']]);
});

test('the action leaves full-matrix on an issue that a later failing run updates', async () => {
  const planned = await plan({ matrix: 'reduced', failed: true });
  const calls = await syncIssue(planned, [issue(5, HELD)]);
  assert.equal(calls.update.length, 1);
  assert.deepEqual(calls.addLabels, []);
  assert.ok(!('labels' in calls.update[0]), 'the update does not set the label list');
  assert.ok(calls.update[0].body.includes('a passing reduced run leaves the issue open'));
});

// ── The reduced matrix, as kubernetes-matrix.yaml computes it ───────────────

/** Runs the Export matrix step of kubernetes-matrix.yaml and returns its outputs. */
function exportMatrix(matrix) {
  const lines = read('.github/workflows/kubernetes-matrix.yaml').split('\n');
  const nodes = block(lines, lines.findIndex(l => l.trim() === 'NODES: |'));
  const script = block(lines, lines.findIndex(l => l.trim() === 'run: |'));
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'matrix-')), 'output');
  fs.writeFileSync(output, '');
  execFileSync('bash', ['-c', script], {
    env: { ...process.env, MATRIX: matrix, NODES: nodes, KIND_VERSION: 'v0', GITHUB_OUTPUT: output },
  });
  return Object.fromEntries(fs.readFileSync(output, 'utf8').trim().split('\n').map(l => {
    const at = l.indexOf('=');
    return [l.slice(0, at), l.slice(at + 1)];
  }));
}
const LISTS = ['versions', 'beta-versions', 'stable-versions', 'smoke-versions'];
const hasJq = spawnSync('jq', ['--version']).status === 0;

test('the label matches what a reduced run really runs', { skip: hasJq ? false : 'needs jq' }, async () => {
  const full = exportMatrix('full');
  const reduced = exportMatrix('reduced');
  const versions = JSON.parse(full.versions);
  const all = list => versions.map(v => job(`Conformance tests / ${list} (${v})`));
  let labelled = 0;
  for (const list of LISTS) {
    for (const version of JSON.parse(full[list])) {
      // A full run in which only this job failed.
      const jobs = LISTS.flatMap(l => JSON.parse(full[l]).map(v => job(`Conformance tests / ${l}-job (${v})`)));
      jobs.find(j => j.name === `Conformance tests / ${list}-job (${version})`).conclusion = 'failure';
      const r = await plan({ failed: true, jobs: [...all('assert'), ...jobs] });
      const inReducedRun = JSON.parse(reduced[list]).includes(version);
      assert.equal(r.fullMatrix, !inReducedRun, `${list} ${version}`);
      labelled += r.fullMatrix ? 1 : 0;
    }
  }
  assert.ok(labelled > 0, 'the full matrix has versions that the reduced one skips');
});

test('the conformance jobs read only the version lists the label knows', () => {
  const used = new Set([...read('.github/workflows/tests-conformance.yaml')
    .matchAll(/k8s-version: \$\{\{ fromJSON\((.*)\) \}\}/g)]
    .flatMap(m => [...m[1].matchAll(/needs\.kubernetes-matrix\.outputs\.([\w-]+)/g)].map(n => n[1])));
  assert.ok(used.size > 0);
  for (const list of used) {
    assert.ok(LISTS.includes(list), `${list} is not covered by the tests above`);
  }
  // The policy library and K6 workflows get no matrix input: they run the same way in both matrices.
  for (const id of ['conformance-policy-library-tests', 'k6-tests']) {
    assert.ok(!jobText(id).includes('matrix:'), id);
  }
});

// ── Batch summary ───────────────────────────────────────────────────────────

const sha = n => n.toString(16).padStart(9, '0').padEnd(40, 'f');
const user = (login, type = 'User') => ({ login, type });
/** A commit of the batch; `pr` is its merged pull request, if any. */
const commit = (n, { pr = null, author = null, title = `change ${n}` } = {}) => ({
  sha: sha(n), html_url: `https://github.com/o/r/commit/${sha(n)}`, author,
  commit: { message: `${title}\n\nbody`, author: { name: 'Some Name' } }, pr,
});
const pull = (number, login, type = 'User') => ({
  number, html_url: `https://github.com/o/r/pull/${number}`, merged_at: 'x', base: { ref: 'main' }, user: user(login, type),
});

/**
 * @param commits  oldest first, as the compare API returns them
 * @param green    the earlier green runs, newest first
 */
async function batch({ matrix = 'reduced', commits = [], green = [{ id: 1, head_sha: sha(0), html_url: 'https://github.com/o/r/actions/runs/1', display_title: 'Tests on main (full)' }] }) {
  const c = core();
  const github = {
    paginate: async (fn, args, map) => {
      const response = await fn(args);
      return map ? map(response, () => {}) : response.data;
    },
    rest: {
      actions: { listWorkflowRuns: async () => ({ data: green }) },
      repos: {
        compareCommitsWithBasehead: async () => ({ data: { commits } }),
        listPullRequestsAssociatedWithCommit: async a =>
          ({ data: [commits.find(x => x.sha === a.commit_sha).pr].filter(Boolean) }),
      },
    },
  };
  const context = {
    repo: { owner: 'o', repo: 'r' }, ref: 'refs/heads/main', sha: sha(999), runId: 77, serverUrl: 'https://github.com',
  };
  await withEnv({ MATRIX: matrix }, () =>
    runScript(stepScript('Summarize batch'), { github, core: c, context, process }));
  return { note: c.outputs['issue-note'], summary: c.summaryText };
}

test('batch: the issue note has the compare link, the pull requests and their authors', async () => {
  const r = await batch({
    commits: [commit(1, { pr: pull(11, 'alice') }), commit(2, { pr: pull(12, 'bob') })],
  });
  assert.ok(r.note.startsWith('### Suspect commits\n\n'));
  assert.ok(r.note.includes(`[\`${sha(0).slice(0, 9)}..${sha(999).slice(0, 9)}\`](https://github.com/o/r/compare/${sha(0)}...${sha(999)})`));
  assert.ok(r.note.includes('2 commits since the [last green run](https://github.com/o/r/actions/runs/1)'));
  // Newest first.
  assert.ok(r.note.includes('- #12 by bob\n- #11 by alice'));
  assert.ok(r.note.includes('\n@bob @alice: this run failed after your changes landed.'));
  // The job summary is still written.
  assert.ok(r.summary.includes('change 1'));
});

test('batch: a full run names the last green full run', async () => {
  const green = [
    { id: 3, head_sha: sha(5), html_url: 'reduced-run', display_title: 'Tests on main (reduced)' },
    { id: 2, head_sha: sha(0), html_url: 'full-run', display_title: 'Tests on main (full)' },
  ];
  const full = await batch({ matrix: 'full', green, commits: [commit(1, { pr: pull(11, 'alice') })] });
  assert.ok(full.note.includes('since the [last green full run](full-run)'));
  // A green full run also covers the reduced matrix, and so does a reduced one.
  const reduced = await batch({ matrix: 'reduced', green, commits: [commit(1, { pr: pull(11, 'alice') })] });
  assert.ok(reduced.note.includes('since the [last green run](reduced-run)'));
});

test('batch: bots are listed but not mentioned', async () => {
  const r = await batch({
    commits: [
      commit(1, { pr: pull(11, 'kyverno-pr-updater[bot]', 'Bot') }),
      commit(2, { pr: pull(12, 'alice') }),
      commit(3, { author: user('dependabot[bot]', 'Bot') }),
    ],
  });
  assert.ok(r.note.includes('- #11 by kyverno-pr-updater[bot]'));
  assert.ok(r.note.includes(`- ${sha(3).slice(0, 9)} by dependabot[bot]`));
  assert.deepEqual(r.note.match(/@[\w[\]-]+/g), ['@alice']);
  const bots = await batch({ commits: [commit(1, { pr: pull(11, 'kyverno-pr-updater[bot]', 'Bot') })] });
  assert.ok(!bots.note.includes('@'));
});

test('batch: a commit without a pull request is listed by its SHA and its author', async () => {
  const r = await batch({ commits: [commit(1, { author: user('carol') }), commit(2)] });
  assert.ok(r.note.includes(`- ${sha(2).slice(0, 9)} by unknown author\n- ${sha(1).slice(0, 9)} by carol`));
  assert.deepEqual(r.note.match(/@\w+/g), ['@carol']);
});

test('batch: a pull request with several commits is listed and mentioned once', async () => {
  const r = await batch({ commits: [1, 2, 3].map(n => commit(n, { pr: pull(11, 'alice') })) });
  assert.equal(r.note.match(/- #11 by alice/g).length, 1);
  assert.equal(r.note.match(/@alice/g).length, 1);
});

test('batch: the list and the mentions are capped, and the note stays small', async () => {
  const commits = Array.from({ length: 150 }, (_, i) =>
    commit(i + 1, { pr: pull(100000 + i, `${'u'.repeat(34)}-${String(i).padStart(4, '0')}`) }));
  const r = await batch({ commits });
  assert.equal(r.note.match(/^- #\d+ by /gm).length, 20);
  assert.ok(r.note.includes('- Only the latest 20 are listed. The [compare view](https://github.com/o/r/compare/'));
  assert.equal(r.note.match(/@u+-\d+/g).length, 10);
  // The latest 100 commits are read, so 100 authors are known.
  assert.ok(r.note.includes(' and 90 more: this run failed'));
  assert.ok(r.note.includes('150 commits since'));
  // The issue body is capped at 65,536 characters; the action's log excerpts take up to 45,000.
  assert.ok(r.note.length < 2500, `${r.note.length} characters`);
  const whole = await plan({ ...fails('Conformance tests / assert (v1.35.8)', 'Conformance tests / assert (v1.36.4)'), batchNote: r.note });
  assert.ok(whole.note.length < 3500, `${whole.note.length} characters`);
});

test('batch: commit titles stay out of the issue note', async () => {
  const title = 'fix: thanks @mallory <!-- workflow-failure:Tests:refs/heads/release-1.20 -->';
  const r = await batch({ commits: [commit(1, { pr: pull(11, 'alice'), title })] });
  assert.ok(!r.note.includes('mallory'));
  assert.ok(!r.note.includes('<!--'));
});

test('batch: no earlier green run, or the same commit again', async () => {
  const none = await batch({ green: [] });
  assert.equal(none.note, '### Suspect commits\n\nNo earlier green reduced run on `main`, so the range of suspect commits is unknown.');
  const same = await batch({ green: [{ id: 1, head_sha: sha(999), html_url: 'run-1', display_title: 'Tests on main (full)' }] });
  assert.equal(same.note, `### Suspect commits\n\nThe [last green run](run-1) tested the same commit \`${sha(999).slice(0, 9)}\`, so this run tests no new commit.`);
  assert.ok(!`${none.note}${same.note}`.includes('@'));
});
