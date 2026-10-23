# AGENTS.md — `cmd/cli/kubectl-kyverno/commands/apply`

This file documents the **output-mode and stream-separation contract** for the
`kyverno apply` command. Read it before adding, changing, or combining output
flags to avoid reintroducing precedence bugs or mixed-stream corruption.

---

## Output-mode overview

`kyverno apply` has **three distinct output channels**:

| Channel | Written to | Controlled by |
|---------|-----------|---------------|
| **Mutated / generated resource files** | File or directory on disk | `--output-dir` (deprecated alias `--output`/`-o`) |
| **Machine-readable report / exceptions** | `stdout` | `-p`/`--policy-report` or `--generate-exceptions` combined with `--output-format (json\|yaml)` |
| **Human-readable results summary** | `stdout` | Default; `--output-format table` / `-t`/`--table` for tabular layout |

These channels are **mutually exclusive** on `stdout`. Never write two channels
to the same stream.

---

## Flag precedence and validation rules

All validation happens in the `RunE` closure in `Command()`, **before** calling
`applyCommandHelper`. The order is:

1. `--output` + `--output-dir` simultaneously → error (both target the same
   `MutateLogPath` field; using both is ambiguous).
2. `--table` sets `OutputFormat = "table"`; if `--output-format` is also given
   with a different value, it is a conflict → error.
3. `OutputFormat` must be one of `""`, `"table"`, `"json"`, `"yaml"` → error on
   anything else.
4. `--output-format table` is incompatible with `-p`/`--policy-report`,
   `--generate-exceptions`, and `-i`/`--stdin` → error.
5. `-p`/`--policy-report` and `--generate-exceptions` are mutually exclusive.
6. `--output-format json|yaml` without `-p` or `--generate-exceptions` → warning
   (written to stderr); the flag has no effect in that mode.
7. `--explain` is incompatible with any mode that owns stdout for machine-readable
   output: `-p`, `--generate-exceptions`, `--stdin`, and `--output-format table`.
   The engine trace is human-readable text; mixing it with structured output makes
   both streams unparseable. See `checkExplainCompatible()`.
8. Format keywords (`json`, `yaml`, `yml`, `table`, `markdown`, `junit`) are
   rejected as `--output-dir` / `--output` values to prevent accidentally creating
   files or directories named after format keywords on disk. This validation is
   performed in `checkArguments()` and `checkMutateLogPath()`.

---

## Adding a new output-format value

1. Add the value to the `switch` in `RunE` (the `case "", "table", "json", "yaml":`
   block).
2. Handle the new format in `printReports` and/or `printExceptions` in `print.go`.
3. Add it to the format-keyword rejection list in `checkMutateLogPathSafe()` if it
   collides with a user-visible format name.
4. Update this file.

---

## Adding a new output mode

1. Add a new `bool` (or string) field to `ApplyCommandConfig`.
2. Add mutual-exclusivity checks in `RunE` against all existing modes that own
   stdout.
3. Add a `checkExplainCompatible()` entry if the new mode writes structured output
   to stdout.
4. Add unit tests in `output_flags_test.go` covering the conflict cases.
5. Update this file.

---

## Stream separation rules (invariants)

- **stdout** must contain exactly **one** of:
  - the human-readable violations / summary (default mode)
  - the table (table mode)
  - a policy report document (policy-report mode)
  - a list of policy exceptions (generate-exceptions mode)
- **stderr** is used for warnings, deprecation notices, and (in structured-output
  modes) skipped/invalid-policy diagnostics. The diagnostics are routed to stderr
  in `-p`/`--policy-report` and `--generate-exceptions` modes so the JSON or YAML
  document on stdout is never prefixed with human-readable prose.
- The **"Applying N policy rule(s)..." banner** is suppressed in table mode
  (`--output-format table` / `-t`) so the table is the sole content on stdout.
- **Files on disk** receive mutated/generated resource YAML via `--output-dir`
  and are always written as YAML regardless of `--output-format`.
- `--explain` trace goes to stdout in default mode only; it is blocked in all
  structured-output modes to prevent corruption.
