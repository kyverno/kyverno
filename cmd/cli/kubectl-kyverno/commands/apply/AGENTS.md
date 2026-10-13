# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

`kyverno apply` loads policies leniently and reports what it could not load instead of dropping it. The loader side
of this contract is described in `cmd/cli/kubectl-kyverno/policy/AGENTS.md`; this file covers the apply side.

## Loading policies

- `loadPolicies` calls `policy.LoadSkippingInvalidFiles` for every policy path: once per local, OCI or stdin path,
  and once per YAML file returned by `gitutils.ListYamls` for git sources. Keep `kyverno test`, `oci push` and
  `fix policy` on the strict `policy.Load`; a broken policy must keep failing those.
- `isFatalPolicyLoadError` decides what stops apply: a blocked legacy policy or `policy.ErrValidatorSetup`.
  Everything else is recorded with `recordPolicyLoadError` / `recordPolicyLoadErrors` into
  `ApplyCommandConfig.policyLoadErrors`, which `applyCommandHelper` resets on every run and hands back through
  `SkippedInvalidPolicies.loadErrors`.

## Printing what was not loaded

- `printPolicyLoadErrors` prints the "Policies Not Loaded" section, one `path: reason` line per entry.
- `Command` sends that section to stdout normally, but to stderr when stdout is meant for another program
  (`--policy-report`, `--generate-exceptions`, `--stdin`), so the report, the generated exceptions or the mutated
  resource on stdout stay parseable. Add any new machine readable output mode to that condition.
- Load errors do not change the exit code and are not counted by `--warnings-as-errors`, which only covers
  deprecation warnings recorded by `recordPolicyWarnings`.
