# kyverno.io CRDs: 1.20 compatibility snapshot

This directory mixes two kinds of files during the 1.20 legacy-API removal
(#17713, epic #17701): CRDs still generated from Go, and CRDs frozen as
compatibility inputs so the legacy `kyverno.io` Kubernetes APIs stay served
after their Go types are deleted.

## Provenance

Snapshot baseline: commit `22a7827fe` (upstream/main, 2026-10-23). Generator:
`hack/controller-gen`, built from `hack/controller-gen/go.mod`
(`sigs.k8s.io/controller-tools` v0.22.0, `-buildvcs=false`, hence the
`controller-gen.kubebuilder.io/version: (devel)` header in every file). Go
1.27.3. `Makefile`'s `CONTROLLER_GEN_VERSION` variable labels the Helm chart's
own version annotation only; it isn't the generator version and wasn't
touched for this snapshot.

### Corrections since the baseline

- **2026-10-26 (PR #18286, #17707):** `kyverno.io_clusterpolicies.yaml` and
  `kyverno.io_policies.yaml` were hand-edited to restore
  `ImageExtractorConfig.Filter` (`imageExtractors[].filter`). That field was
  added by #16269 only to `release-1.19`, never forward-ported to `main`, so
  the `22a7827fe` baseline itself had already been missing it before this
  file existed. Confirmed against the real v1.19.2 release's CRD (schema
  byte-identical, modulo `description` text, after the fix). Digests in
  `scripts/verify-legacy-crd-snapshot.sh` updated accordingly.

## Classification

| File | Status | Versions | Refreshed by |
|---|---|---|---|
| `kyverno.io_clusterpolicies.yaml` | frozen | v1, v2beta1 | never (compatibility input) |
| `kyverno.io_policies.yaml` | frozen | v1, v2beta1 | never (compatibility input) |
| `kyverno.io_policyexceptions.yaml` | frozen | v2, v2beta1 | never (compatibility input) |
| `kyverno.io_cleanuppolicies.yaml` | frozen | v2, v2beta1 | never (compatibility input) |
| `kyverno.io_clustercleanuppolicies.yaml` | frozen | v2, v2beta1 | never (compatibility input) |
| `kyverno.io_globalcontextentries.yaml` | tracks Go | v2, v2alpha1, v2beta1 | `make codegen-crds-kyverno` |
| `kyverno.io_updaterequests.yaml` | mixed | v1beta1 (frozen), v2 (tracks Go) | `make codegen-crds-kyverno` |

## Rules

- The five frozen files and the UpdateRequest `v1beta1` entry inside
  `kyverno.io_updaterequests.yaml` are compatibility inputs for 1.20. They are
  never regenerated, regardless of what happens to their Go types.
- Every other file and version tracks Go through
  `make codegen-crds-kyverno`, which runs `hack/controller-gen` over
  `api/kyverno/{v2,v2alpha1,v2beta1}` and then `hack/crd-splice` to merge the
  result into this directory. A new kind or version needs no configuration
  here: `crd-splice` is default-safe, so anything not explicitly named below
  tracks Go automatically.
- The frozen set is declared once, as `KYVERNO_CRDS_FROZEN_FILES` and
  `KYVERNO_CRDS_FROZEN_VERSIONS` in the `Makefile`. To freeze or unfreeze
  something, edit those variables and the matching digests in
  `scripts/verify-legacy-crd-snapshot.sh`. `KYVERNO_CRDS_FROZEN_VERSIONS` holds
  space-separated `name=version` entries, one version per entry (repeat the
  entry to freeze several versions of the same file) — this is the same
  single-version syntax `crd-splice`'s `--freeze` and `extract --freeze`
  flags and `verify-legacy-crd-snapshot.sh` all expect.
- `make verify-legacy-crd-snapshot` fails CI if any frozen file, or the
  frozen `v1beta1` block, changes.
- `scripts/verify-legacy-crd-retention.sh` (#17489) is a separate, older
  check in the same `check-ct-lint.yaml` job: it asserts the rendered chart
  still carries `helm.sh/resource-policy: keep` on exactly the five legacy
  policy CRDs. It used to depend on a kubebuilder marker surviving codegen;
  now that those five files are frozen bytes, the annotation is carried in
  the frozen bytes themselves and can no longer be dropped by a codegen or
  marker change. The two checks protect different things (an annotation vs.
  full schema bytes) and both stay in place.

## Local prerequisites

`make codegen-crds-kyverno` itself needs only `controller-gen` and
`crd-splice`, both installed via `make install-tools`. Other `codegen-api-*`
targets (`register-gen`, `deepcopy-gen`) install from
`k8s.io/code-generator@v0.35.0`, which requires Go >= 1.25. On a host whose
base `go` is older than that, `go install` silently builds those tools with
whatever newer toolchain it switches to, which can then fail to parse this
repo's own Go 1.27.3 stdlib. Build them with
`GOTOOLCHAIN=go1.27.3 GOBIN=.tools go install k8s.io/code-generator/cmd/{register-gen,deepcopy-gen}@v0.35.0`
if `make codegen-api-all` errors with a `math/rand/v2` parse failure. CI's
base Go is already 1.27.3 and is unaffected.
