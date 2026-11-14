// Helpers for testing the inline `script:` of an actions/github-script step.
// The script is read from the workflow or action file itself, so the tests
// run the code that ships, against a stubbed GitHub API.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const AsyncFunction = (async () => {}).constructor;

/** The first `script: |` block of a workflow or action file. */
export function inlineScript(file) {
  const lines = fs.readFileSync(path.join(root, file), 'utf8').split('\n');
  const start = lines.findIndex(l => l.trimEnd().endsWith('script: |'));
  if (start < 0) {
    throw new Error(`no inline script in ${file}`);
  }
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

/** Runs a script the way github-script does, with the given globals. */
export function runScript(source, globals) {
  return new AsyncFunction(...Object.keys(globals), source)(...Object.values(globals));
}

/** A `core` stub that records what was logged. */
export function fakeCore() {
  const log = [];
  let failure = null;
  return {
    log,
    get failure() { return failure; },
    info: m => log.push(`info: ${m}`),
    notice: m => log.push(`notice: ${m}`),
    warning: m => log.push(`warning: ${m}`),
    setFailed: m => { failure = m; },
  };
}
