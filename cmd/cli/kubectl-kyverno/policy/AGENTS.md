# AGENTS.md — cmd/cli/kubectl-kyverno/policy

This package turns policy files into typed policies for the CLI (`load.go`). It has two entry points with
deliberately different failure behaviour; keep them apart when changing either one.

## Strict and lenient loading

- **`Load` / `LoadWithLoader` (strict)** are used by `kyverno test`, `oci push` and `fix policy`. The first path,
  file or document that fails to load fails the whole call (`pendingErr` in `kubectlValidateLoader`), so a broken
  policy can never pass a test run silently.
- **`LoadSkippingInvalidFiles` (lenient)** is used only by `kyverno apply` (`loadPolicies` in
  `commands/apply/command.go`). Failures are recorded in `LoaderResults.NonFatalErrors` under the path they came
  from and everything else still loads: valid documents in the same file, the rest of a directory, and the other
  paths of the same call. apply prints these under "Policies Not Loaded".

The lenient behaviour is threaded through `kubectlValidateLoader(allowLegacyPolicies, lenient)`, `handleListItems`,
`loadWithLoader(..., skipUnreadable, ...)` and `fsLoad(..., skipUnreadable)`, plus the `skipInvalidFiles` wrapper.
Every new branch is gated on that flag; the strict path must keep behaving as before (`make test-cli` exercises it).

## What stays fatal in lenient mode

- **Legacy policy blocks** (`pkgdeprecations.IsLegacyPolicyBlockError`) are returned as errors at every level, so
  a legacy policy anywhere in the input still stops apply with the migration hint.
- **Validator setup failures**: `LoadSkippingInvalidFiles` builds `loaderDelegate` first and wraps a failure in
  `ErrValidatorSetup`. apply checks both through `isFatalPolicyLoadError`. Without this, a broken embedded CRD
  validator would show up as every file being skipped and apply would run with no policies.

## What lenient mode reports and what it ignores

`isPolicyGroupKind` decides whether a document that failed is worth reporting:

- all of `policies.kyverno.io`, matched by group rather than by the kinds and versions this CLI knows, so a
  policy written for a newer Kyverno is reported instead of silently ignored;
- only the policy kinds of `kyverno.io` (`legacyPolicyKinds`), since that group also holds `GlobalContextEntry`
  and `UpdateRequest`;
- only the `*AdmissionPolicy*` kinds of `admissionregistration.k8s.io`.

Anything else (resources, `kyverno-test.yaml`, values and context files, `kustomization.yaml`) is ignored, because
policy folders routinely hold those next to the policies. A document that is not valid YAML is always reported,
since there is no way to tell what it was meant to be. In a directory, hidden entries are skipped before they are
touched and an unreadable entry is reported only if it is a directory or a YAML file.

Document level errors are prefixed with the document's kind and name (`documentError`, `namedError`, also used
for `List` items) so several broken policies in one file can be told apart.
