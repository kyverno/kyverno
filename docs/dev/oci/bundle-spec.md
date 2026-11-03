# Kyverno CEL Policy Bundle OCI Specification

This is the canonical, normative specification for the Kyverno CEL Policy Bundle OCI format. If you're
implementing a writer (`kyverno oci push`) or a reader (`kyverno oci pull`, `kyverno test oci://...`,
`kyverno apply oci://...`, `kyverno oci inspect`, `kyverno oci resolve`, `kyverno oci validate`, or a third-party
tool), this document is the contract. Don't restate these rules elsewhere in this repository — if you're changing
them, this is the only file to edit. User-facing how-to material (pushing, pulling, and the Flux example) lives on
[kyverno.io](https://kyverno.io) instead; see `docs/dev/oci/README.md` for the split.

This document defines the wire format. `docs/context/shared/api-versioning.md` defines the policy API's own
versioning rules and is the authority this document defers to for the accepted-version table in
[3. Supported resources](#3-supported-resources).

## 1. Status and scope

**Format version:** `1.0.0`, shipping in Kyverno 1.20.

**This specification supersedes the format that Kyverno CLIs from 1.9 through 1.19 shipped.** That format wrote one
bare-YAML layer per resource, under different media types, with no config-blob content and no manifest-level index.
The rest of this document describes format 1.0, a different, incompatible shape: a single `tar+gzip` content layer
plus a populated JSON config blob. [8. Format versioning and compatibility](#8-format-versioning-and-compatibility)
states exactly how a reader tells the two apart and what it must do with a pre-1.20 image.

**What this supersedes, precisely.** The per-resource-layer shape dates to `cbbd8488c`, "feat: oci pull/push
support for policie(s)" (kyverno/kyverno#5026, 2022), which first wrote one `static.NewLayer` per resource under
`application/vnd.cncf.kyverno.policy.layer.v1+yaml`. That commit, not any recent work, is what this document
replaces. The 2022 proposal behind it had already reserved a `tar+gzip` layer and a populated config blob for the
multi-resource case; only the single-document-per-layer variant was ever built.

Two implementation issues merged to `main` ahead of this specification, and **neither of them chose that layer
shape**:

- **#17661** (`6620b5aa9`) narrowed `kyverno oci` to CEL resources: it added the accepted-kind table, legacy-kind
  rejection, the duplicate-identity check, exception-to-policy reference validation, and CEL compile-on-push. It
  inherited `cbbd8488c`'s layer shape unchanged. Its validation logic carries forward into format 1.0 largely
  intact — the `Validate` stage this document assumes is substantially that work, relocated. Only its tests change,
  because `buildImage` is replaced by the `Assemble`/`Validate`/`Write` split.
- **#17666** (`b7eb373f9`) shipped `oci://` as the source scheme for `kyverno test` and `kyverno apply`, via
  `pull.ToTempDir`. That scheme and plumbing aren't superseded at all: they keep working unchanged under format
  1.0, because a conformant `Read` still produces a local tree that `policy.Load` can walk, which is the same
  contract `ToTempDir` already relies on.

**The format is committed as of 1.20.** `kyverno oci` ships ungated, without requiring `KYVERNO_EXPERIMENTAL`, as
of 1.20, which settles the question of whether #17660 may change a format that shipped in released CLIs — it
does, and this specification is the format 1.20 ships. The pre-1.20 bare-YAML-per-layer artifacts, under media
types `application/vnd.cncf.kyverno.config.v1+json` and `application/vnd.cncf.kyverno.policy.layer.v1+yaml`, are
superseded and rejected with an actionable "legacy image format" error; see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility).

**This specification also supersedes a 2022 design proposal.**
`kyverno/KDP:proposals/store_kyverno_policies_in_oci_registries.md` (May 2022, tracking kyverno/kyverno#3154) is
the proposal the current `kyverno oci push`/`pull` implementation came from. It anticipated a `tar+gzip` layer and
a populated config blob for the multi-resource case, but only the single-bare-YAML-per-layer variant was ever
built, and the config stayed empty. This document completes that design rather than reversing it; see
[4. Artifact structure](#4-artifact-structure) for the one naming choice where this format diverges from what that
proposal reserved, and why.

The tests that change as a consequence of adopting this format, so a reviewer can check the blast radius without
re-deriving it:

- `cmd/cli/kubectl-kyverno/commands/oci/push/options_test.go`: `TestBuildImageValidCELPolicyAndException` asserts
  a manifest with one layer per resource and per-layer `io.kyverno.image.kind` and `io.kyverno.image.name`
  annotations, and needs full rewriting for the single content-layer shape. The other seven `TestBuildImage*`
  tests (`TestBuildImageDuplicateIdentity`, `TestBuildImageInvalidExceptionReference`,
  `TestBuildImageRejectsNonFatalErrors`, `TestBuildImageRejectsVAPResources`,
  `TestBuildImageExceptionOnlyBundleAlwaysChecksRefs`, `TestBuildImageRejectsUnsupportedAPIVersion`,
  `TestBuildImageRejectsInvalidExceptionCELExpression`) assert only error behavior from `buildImage(results)`, not
  layers or annotations; they still need updating because `buildImage` itself is replaced by
  `Assemble`/`Validate`/`Write`, not because of any layer-shape assertion they make.
  `TestExecuteRejectsLegacyKind` drives `execute` end to end through `buildImage` and needs the same update for
  the same reason.
- `cmd/cli/kubectl-kyverno/commands/oci/pull/options_test.go`: every `TestExtractAndSavePolicies*` test builds a
  bare-YAML layer with `static.NewLayer` and asserts synthesized filenames such as
  `validatingpolicy-require-labels.yaml`. `TestExtractAndSavePoliciesKindPrefixedFilenames`,
  `TestExtractAndSavePoliciesNamespacedResourceIncludesNamespaceInFilename`, and
  `TestExtractAndSavePoliciesRejectsDuplicateAcrossLayers` test behavior (synthesized names, a multi-layer
  scenario) that no longer exists under this format.
- `cmd/cli/kubectl-kyverno/commands/oci/internal/annotations_test.go`: every test in this file (for example
  `TestAnnotationsValidatingPolicy`, `TestAnnotationsNamespaceOmittedWhenEmpty`) asserts the per-layer annotation
  helper this specification retires (see [4. Artifact structure](#4-artifact-structure)).

### Normative language

This document uses MUST, MUST NOT, SHOULD, and MAY as defined in RFC 2119: MUST and MUST NOT are absolute
requirements; SHOULD is a strong recommendation that a writer or reader can deviate from only with a specific
reason; MAY is genuinely optional. "Writer" means anything that produces a conformant bundle; "reader" means a
conformant reader, bound by the "Reader MUST" list in [9. Conformance](#9-conformance), as distinct from a
generic consumer of the OCI artifact. Definitions of these and other terms are in
[2. Terminology](#2-terminology).

`design.md` section 6, the design record for this specification, called for lowercase "must"/"should"/"may" to
stay within the Google Developer Documentation Style Guide the project otherwise follows. This document uses RFC
2119 capitals instead, a deliberate, instructed departure from that line, because a normative checklist this large
(see [9. Conformance](#9-conformance)) needs "writer MUST"/"reader MUST" to read unambiguously as a checkable
requirement rather than as ordinary prose. This paragraph is the record of that departure; don't re-raise it as a
style inconsistency.

### API family

This format carries only resources in the `policies.kyverno.io` API group, the CEL-based policy and exception
kinds. It doesn't carry the legacy `kyverno.io` kinds (`Policy`, `ClusterPolicy`, `PolicyException`) or the legacy
`kyverno.io` cleanup kinds (`CleanupPolicy`, `ClusterCleanupPolicy`). There's no migration path from a legacy
bundle to this format; a legacy policy must be converted to its CEL equivalent, following the deprecation guidance
in `docs/context/shared/api-versioning.md`, before it can go into a bundle.

## 2. Terminology

- **Bundle**: the complete OCI artifact this specification describes — one manifest, one config blob, one content
  layer, referenced by a single digest.
- **Bundle root**: the top-level directory an author pushes, or the single file if the input is one file.
- **Descriptor**: the optional, author-written `kyverno-bundle.yaml` input at the bundle root. Never archived.
  See [7. Bundle descriptor](#7-bundle-descriptor).
- **Config** (config blob): the tool-generated JSON blob referenced by the manifest's `config` field. Carries the
  normalized descriptor plus the resource index. See [6. Config blob](#6-config-blob).
- **Content layer**: the single `tar+gzip` layer at `layers[0]` that carries the bundle's files. See
  [5. Content layer](#5-content-layer).
- **Resource**: one Kubernetes-style YAML document inside the content layer that is one of the kinds in
  [3. Supported resources](#3-supported-resources).
- **Set**: a named grouping of resources recorded in the config's `sets[]`, typed `policies` or `exceptions`. See
  [6. Config blob](#6-config-blob) for the schema and format-level rules, and
  [7. Bundle descriptor](#7-bundle-descriptor) for how a writer assigns resources to sets.
- **Identity**: the tuple `Kind/namespace/name`, where `namespace` is `metadata.namespace` exactly as written
  (conventionally empty for a cluster-scoped kind, but not normalized by the format if an author sets one anyway;
  see [3. Supported resources](#3-supported-resources)) — that a writer and reader use to detect duplicate
  resources. Deliberately excludes the API version.
- **Reader**: a **conformant reader** is anything bound by the "Reader MUST" list in
  [9. Conformance](#9-conformance) — `kyverno oci pull`, `kyverno test oci://...`, `kyverno apply oci://...`,
  `kyverno oci inspect`, `kyverno oci resolve`, and `kyverno oci validate` are all conformant readers, and any
  third-party tool that hands a bundle's contents to a policy engine is expected to be one too. A **generic
  consumer** — Flux's `OCIRepository` controller, `crane`, `oras`, and similar tools that extract or inspect the
  artifact without evaluating policies against it — is not bound by the "Reader MUST" list; design D11 scopes
  re-validation and the index cross-check to the readers whose output feeds policy evaluation, not to every
  possible consumer of the OCI artifact. Unqualified "reader" in this document means a conformant reader unless a
  sentence says "generic consumer." `kyverno oci validate` is both a conformant reader, in reference mode
  (`--image`), and a writer-policy checker: it additionally applies the checks in the "Writer-policy checks"
  subsection of [9. Conformance](#9-conformance), which bind no other reader.
- **Writer**: anything that produces a bundle, today only `kyverno oci push`.

## 3. Supported resources

A conformant bundle carries only resources whose group is `policies.kyverno.io` and whose kind is one of these 11:

| Kind | Namespaced |
|---|---|
| `ValidatingPolicy` | No |
| `NamespacedValidatingPolicy` | Yes |
| `MutatingPolicy` | No |
| `NamespacedMutatingPolicy` | Yes |
| `GeneratingPolicy` | No |
| `NamespacedGeneratingPolicy` | Yes |
| `DeletingPolicy` | No |
| `NamespacedDeletingPolicy` | Yes |
| `ImageValidatingPolicy` | No |
| `NamespacedImageValidatingPolicy` | Yes |
| `PolicyException` | Yes |

Issue `#17668` is expected to add a test that parses this table out of the Markdown and compares it against the
consolidated kind table in the CLI's loader (see `design.md` R5); no such test exists yet. Until it does, keep the
table's shape stable so that test can be added without reshaping the table: one kind per row, no merged cells, no
footnotes inside cells. Put anything else about a kind, such as the Envoy and HTTP note below, in prose outside the
table.

**A `ValidatingPolicy` whose `spec.evaluation.mode` is `Envoy` or `HTTP` is still `Kind: ValidatingPolicy`.** These
modes configure how a `ValidatingPolicy` evaluates a request; they aren't separate kinds, and the resource index
records them as `ValidatingPolicy`.

### Accepted API versions

**This table states two different things, and they currently disagree: what the format permits a version to be,
and what the 1.20 CLI's `kyverno oci` implementation actually accepts.** A resource's `apiVersion` MUST be one the
format permits, and its kind MUST be one of the 11 above; separately, whether a given conformant CLI build accepts
a permitted version yet is a statement about that build, not about the format.

| Version | Format status | 1.20 CLI accepts it? | Earliest release serving this version |
|---|---|---|---|
| `policies.kyverno.io/v1beta1` | Permitted (storage version) | **Yes** | 1.16 |
| `policies.kyverno.io/v1` | Permitted (served, stable-named) | **Yes** | 1.17 |
| `policies.kyverno.io/v1alpha1` | Rejected | No | 1.14 |

**The 1.20 CLI accepts `policies.kyverno.io/v1beta1` and `policies.kyverno.io/v1`.**
`internal.AcceptedAPIVersions` in `cmd/cli/kubectl-kyverno/commands/oci/internal` is this table's code form, built
from `policiesv1beta1.GroupVersion` and `policiesv1.GroupVersion`; `internal.IsAcceptedAPIVersion` is what
`validateDocuments` and `validateResults`' `checkResource` both call. A bundle whose resources are `v1beta1`, `v1`,
or a mix of the two round-trips through the 1.20 CLI. `v1alpha1` stays rejected: every `v1alpha1` type in
`github.com/kyverno/api` carries `+kubebuilder:deprecatedversion`, and the alpha tier carries no compatibility
guarantee (see the stability table in `docs/context/shared/api-versioning.md`).

`v1beta1` and `v1` have identical JSON schemas in `github.com/kyverno/api`, and the CLI's loader converts both
into one set of Go types, so a bundle can carry either, or both, without loss: a `v1` document stays `v1` in the
archive and in the index, and a writer MUST NOT rewrite `apiVersion`.

This table's "Format status" column is governed by the same rules as `docs/context/shared/api-versioning.md`, not
by this document's own semver: a version becomes format-permitted once it's served, and stops being
format-permitted following its tier's deprecation window from that document (2 minor releases for beta, 3 for
stable). A version becoming format-permitted is necessary, but not sufficient, for a given CLI release to accept
it — the "1.20 CLI accepts it?" column, or its equivalent for a later release, is the one an author checks before
relying on a version working. When `v1` becomes the storage version, nothing in this format's container changes,
and `v1beta1` stays format-permitted until its own deprecation window elapses (see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility) for the container/contents
distinction this observation depends on). Bundle contents are stored byte-for-byte (see
[5. Content layer](#5-content-layer)), so once a CLI's accepted set does include both, accepting the wider,
format-permitted set costs that CLI nothing beyond comparing more strings.

### Explicit rejections

A writer and a reader both MUST reject:

- Legacy `kyverno.io` kinds: `Policy`, `ClusterPolicy`, `CleanupPolicy`, `ClusterCleanupPolicy`, and the legacy
  `kyverno.io` `PolicyException`.
- `admissionregistration.k8s.io` kinds: `ValidatingAdmissionPolicy`, `ValidatingAdmissionPolicyBinding`,
  `MutatingAdmissionPolicy`, `MutatingAdmissionPolicyBinding`.
- Any kind not in the table above.
- Any `policies.kyverno.io` resource whose `apiVersion` the running release doesn't accept — for the 1.20 CLI,
  anything other than `v1beta1` or `v1` (see "Accepted API versions" above).

### Identity

A resource's identity is `Kind/namespace/name`, with the API version deliberately excluded: once a CLI accepts
more than one version for the same kind, a `v1` and a `v1beta1` document with the same kind, namespace, and name
would name the same cluster object, and a bundle carrying both would be a duplicate, not two resources. This is a
format-level rule, stated for whatever set of versions a given CLI accepts; the 1.20 CLI's accepted set is both
`v1beta1` and `v1` (see "Accepted API versions" above), so a `v1` and a `v1beta1` document with the same kind,
namespace, and name are a duplicate identity today. `namespace` is the resource's `metadata.namespace` field taken exactly as written, not forced
blank for a cluster-scoped kind: `ValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`, `DeletingPolicy`, and
`ImageValidatingPolicy` are cluster-scoped and conventionally carry no `metadata.namespace`, but if an author's
manifest sets one anyway, that value is part of the resource's identity, matching the existing push
implementation's identity check. A writer and a reader both MUST reject a bundle containing two resources with
the same identity.

## 4. Artifact structure

A format 1.0 bundle is a plain OCI 1.0 image manifest: `schemaVersion` `2`, `mediaType`
`application/vnd.oci.image.manifest.v1+json`, no `artifactType`, and no `subject`. A reader MUST reject a manifest
whose `schemaVersion` isn't `2` or whose `mediaType` isn't `application/vnd.oci.image.manifest.v1+json` — for
example, a Docker schema-2 manifest (`application/vnd.docker.distribution.manifest.v2+json`) carrying the expected
config and layer media types is not a conformant bundle. The config's media type is the artifact's identity,
following the OCI 1.1 fallback rule that `config.mediaType` is the artifact type when `artifactType` is absent.
This is the shape Helm and Flux both ship at scale today. A future minor version of this format may set
`artifactType` once the CLI's OCI library supports writing it; a reader MUST NOT require `artifactType` to be
present, and MUST accept a manifest whether or not it carries one.

### Layer

The manifest has **exactly one layer**, at `layers[0]`, with media type
`application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip`. A reader MUST reject a manifest with zero layers, more
than one layer, or whose single layer isn't at index 0. There's no per-resource layer and no per-layer annotation
in this format; see [5. Content layer](#5-content-layer) for what the layer carries.

### Config

The manifest's `config` field points at a blob with media type `application/vnd.cncf.kyverno.bundle.config.v1+json`,
described in [6. Config blob](#6-config-blob).

### Media types

| Purpose | Media type | Status |
|---|---|---|
| Config blob | `application/vnd.cncf.kyverno.bundle.config.v1+json` | Current |
| Content layer | `application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip` | Current |
| Legacy config blob | `application/vnd.cncf.kyverno.config.v1+json` | Retired |
| Legacy per-resource layer | `application/vnd.cncf.kyverno.policy.layer.v1+yaml` | Retired |

A reader that recognizes either retired media type MUST fail with an actionable "legacy image format" error
instead of attempting to parse the image as format 1.0; see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility).

The `vnd.cncf.kyverno` tree continues the convention Kyverno's OCI support has used since it was first added, and
matches the vendor-tree convention Helm (`vnd.cncf.helm.*`) and Flux (`vnd.cncf.flux.*`) use as CNCF projects. The
`bundle` segment and the `content`/`config` suffixes follow Helm's `chart.content` naming pattern. This format
does not reuse the layer media type a 2022 design proposal for this feature reserved
(`application/vnd.cncf.kyverno.policy.layer.v1.tar+gzip`, in `store_kyverno_policies_in_oci_registries.md`,
tracking kyverno/kyverno#3154): that proposal paired the reserved layer type with the *same* config media type the
legacy per-resource format uses, which would leave a reader unable to tell a legacy image from a new one by media
type alone, and no artifact was ever published under the reserved name. See [1. Status and scope](#1-status-and-scope)
for this proposal's relationship to this document.

### Manifest annotations

The manifest carries these OCI standard annotations, when known:

| Annotation | Meaning |
|---|---|
| `org.opencontainers.image.source` | Repository URL the bundle was built from |
| `org.opencontainers.image.revision` | Source revision (for example, a commit SHA) |
| `org.opencontainers.image.created` | Creation timestamp, RFC 3339. Omitted by default; see below |
| `org.opencontainers.image.version` | Bundle version |
| `org.opencontainers.image.title` | Bundle name |
| `org.opencontainers.image.description` | Bundle description |

Flux reads `source`, `revision`, and `created` for its own summaries. The manifest also carries these
Kyverno-specific annotations:

| Annotation | Meaning |
|---|---|
| `io.kyverno.bundle.format-version` | This format's version, for example `1.0.0` |
| `io.kyverno.bundle.name` | Bundle name |
| `io.kyverno.bundle.version` | Bundle version |
| `io.kyverno.bundle.kyverno-version` | Kyverno version constraint the bundle targets |

A writer MAY add further annotations from the descriptor's `spec.annotations` (see
[7. Bundle descriptor](#7-bundle-descriptor)), but a writer MUST NOT let `spec.annotations` set a key under either
reserved **prefix**, `io.kyverno.bundle.` or `org.opencontainers.image.`: this reservation covers the whole
prefix, not only the specific keys a writer currently computes (listed above) — a descriptor naming, for example,
`io.kyverno.bundle.custom` or any other key under either prefix is discarded the same as one naming
`io.kyverno.bundle.version` directly, even though this format defines no such key today. A key outside both
prefixes is unaffected. `org.opencontainers.image.created` is **omitted unless** the descriptor sets
`spec.created` or the environment sets `SOURCE_DATE_EPOCH`, so that a default push produces a reproducible
manifest.

**Without a descriptor, a writer MUST NOT derive `io.kyverno.bundle.name`, `io.kyverno.bundle.version`, or
`org.opencontainers.image.version` from the push reference (repository path or tag) or from any other environment
value; it omits them.** A reference is not reliably a name or a semver version, and deriving one would make the
manifest depend on where the bundle happens to be pushed, defeating the digest-reproducibility guarantee in
[5. Content layer](#5-content-layer). See [6. Config blob](#6-config-blob) for the same rule applied to the config
blob's `name` and `version` fields.

**The config blob is canonical; annotations are discoverability duplicates and MUST agree with it.** A reader that
needs authoritative bundle metadata reads the config, not the annotations.

## 5. Content layer

The content layer is a gzip-compressed tar archive (`tar+gzip`) of the bundle root.

### What a writer archives

- Every regular `.yaml` or `.yml` file under the bundle root, at its original relative path with forward-slash
  separators, with its original bytes: multi-document files stay intact, comments are preserved, and nothing is
  re-serialized. `.json` files aren't archived; a writer reading the input tree only reads `.yaml` and `.yml`
  files, matching the CLI's existing loader.
- Every non-empty document (as the reference splitter in [6. Config blob](#6-config-blob) returns them) in every
  archived file MUST be one of the resources in
  [3. Supported resources](#3-supported-resources). A writer establishes this by reading each split document's
  raw `apiVersion` and `kind` fields directly — the same `TypeMeta` fields any loader reads, but inspected by the
  writer itself before any document is handed to a resource loader — rather than by inferring a rejection from
  loader behavior after the fact. This direct, per-document check is what lets a writer reject two kinds of input
  precisely: a document whose group is `cli.kyverno.io` (the CLI's own configuration group: `Test`, `Values`,
  `UserInfo`, `Context`, `ClusterResource`; every one of those kinds has an embedded CRD), and a `v1 List`
  document (see [Archive rules](#archive-rules)). A writer MUST reject both anywhere in the bundle root, not only
  at the reserved descriptor filename. A count-based heuristic (comparing documents read to resources loaded) is
  not sufficient here and MUST NOT be used as the detection mechanism: a file holding a `v1 List` of two
  `ValidatingPolicy` items next to a file holding one `cli.kyverno.io` `Test` document produces two documents and
  two loaded resources either way, so a count comparison can't tell a conformant bundle from one smuggling a
  forbidden `List` past a silently dropped `Test` file. **Today**, `cmd/cli/kubectl-kyverno/policy/load.go`'s
  general-purpose loader already errors on a `cli.kyverno.io` document (`policy type not supported` from
  `processDocumentItem`), and that error is fatal to the whole directory load, not a per-file, non-fatal warning:
  it aborts `fsLoad` for the entire tree, so `kyverno oci push` fails at `policy.Load`, in `execute`, before
  `buildImage` is ever reached. A loader change that makes the loader skip `cli.kyverno.io` documents when reading
  a raw source tree for `apply` or `test` is not yet implemented, tracked under epic
  [#17645](https://github.com/kyverno/kyverno/issues/17645), which is why the writer's own detection here can't
  rely on the loader erroring — it needs to hold regardless of what a future loader does with the same input on a
  different read path.
- Single-file input archives that one file at the root of the layer.

### What a writer excludes

These are files a writer silently skips — not archived, not loaded, and not an error:

- Hidden files and directories (any path component starting with `.`).
- Any file that isn't `.yaml` or `.yml`, for example a `README.md`. A non-`.yaml`/`.yml` file is invisible to the
  writer's document-loading path entirely, the same way it's invisible to the CLI's own loader today
  (`pkg/utils/git/predicate.go`), so there's nothing to reject: the writer never reads its contents.
- The bundle descriptor, `kyverno-bundle.yaml`, **at the bundle root only**. This exact filename is **reserved**:
  a writer MUST NOT archive it and MUST NOT pass it to the resource loader; at the root, it's simply excluded, the
  same as any other file this list skips.

This is a `.yaml`/`.yml` file a writer rejects instead — the push fails, it isn't a silent skip:

- The reserved descriptor filename, `kyverno-bundle.yaml`, found **anywhere below the bundle root** (not at the
  root itself). A writer MUST reject a file of that name below the root as a bundle-layout error: the descriptor
  is a bundle-root-only input, not a name an author can reuse deeper in the tree.
- Any other `.yaml`/`.yml` file whose document isn't one of the resources in
  [3. Supported resources](#3-supported-resources) — for example, a `kustomization.yaml`. Because
  `kustomization.yaml` has the `.yaml` extension, a writer reads it like any other file in the tree and inspects
  its `apiVersion`/`kind` (see [What a writer archives](#what-a-writer-archives)); its group,
  `kustomize.config.k8s.io`, isn't one of the accepted kinds, so the writer rejects it under writer MUST 5, the
  same way it rejects any other unsupported resource. **A `kustomization.yaml` is not silently skipped**: an
  author who wants Flux to generate one at reconcile time keeps it out of the bundle root entirely, rather than
  relying on the writer to drop it quietly.

### Archive rules

A writer MUST reject: absolute paths, `..` path segments, and duplicate paths. A reader restores each entry at
its relative path under the target directory and MUST refuse any entry that would escape that directory.

**A symlink is rejected only where it could stand in for archived content, or where following it would silently
lose content; elsewhere it's excluded like any other path this format doesn't archive.** [What a writer
excludes](#what-a-writer-excludes) already lists hidden paths and non-`.yaml`/`.yml` paths as silently skipped, not
an error, and that rule doesn't carve out an exception for a path that happens to be a symlink: a symlinked
`README.md`, or a hidden `.venv -> /opt/venv`, carries no bundle content either way, so rejecting it would be a
new, gratuitous failure this format never asked for, and matches the pre-1.20 push, which read straight through
both without rejecting either. Concretely, a writer MUST reject:

- A non-hidden symlink whose own name has a `.yaml` or `.yml` extension: a real file at that path would be
  archived content, so a symlink standing in for one is rejected under the same rule that governs the content it
  would otherwise represent.
- A non-hidden symlink that resolves to a directory, regardless of its own name's extension. This case is
  rejected unconditionally, because it's the one a naive skip would lose silently: `fs.DirEntry.Type()` (and
  equivalent APIs) report a symlink's own type bit, not the type of whatever it points at, so a symlinked
  directory whose name carries no recognized extension — for example `common -> ../shared` — would otherwise fall
  through both the directory branch (never descended into, since the entry isn't itself a directory) and the
  extension branch (no recognized extension), and be silently dropped with no error, taking everything under it
  with it. A writer MUST reject this case even though the symlink's own name isn't `.yaml`/`.yml`.
- Any other non-regular entry that isn't a symlink at all, for example a device entry.

A hidden symlink, and a non-hidden symlink that is neither of the two rejected shapes above (for example a
symlink to a regular non-YAML file, or a broken symlink whose name isn't `.yaml`/`.yml`), is excluded the same way
any other hidden or non-YAML path is: not archived, not an error. A symlink that is excluded for being hidden or
non-YAML can't affect the artifact, because it's never read.

This rule governs paths *within* the bundle root. The bundle root itself, if it's a symlink (or sits under a
symlinked parent directory), is resolved to its real path before a writer applies any of the above — that's a
convenience for locating the input tree, not an exception to the reject/exclude rules above, which still apply,
unchanged, to every path found once the root is resolved.

**A hard link is not rejected, and a writer MUST NOT be relied on to detect one.** Telling a hard-linked regular
file apart from an ordinary one requires reading platform-specific stat data (`Nlink`) that a portable,
`CGO_ENABLED=0` implementation can't reach uniformly across the platforms this format's writers build for.
This costs nothing in practice: link structure isn't part of what this format preserves (see
[Round-trip guarantee](#round-trip-guarantee)), and two paths that are hard-linked on disk archive as two
ordinary, independent tar entries with identical content, which is a well-formed bundle either way — the artifact
is byte-identical to one built from two unlinked files with the same content.

**`v1 List` documents are rejected in format 1.x.** A writer identifies a `v1 List` document by the same raw
`apiVersion`/`kind` inspection described in [What a writer archives](#what-a-writer-archives), before any loader
unwrapping occurs (the CLI's general-purpose loader unwraps a `List` into its items when it reads one, which is
exactly why the writer must catch it first: this format's index needs one archived document to map to exactly one
index entry with one `path` and one `documentIndex`, see [6. Config blob](#6-config-blob) and
[9. Conformance](#9-conformance)). Allowing `List` documents is classified as a **major** version change (see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility)), because a 1.0 reader would
otherwise reject a 1.x bundle it should accept; a future format version that allows them is therefore a major
version, not a minor one.

There is no size or count limit a writer must observe on the content layer in format 1.0, except that a writer
MUST reject an input tree that yields zero resources: an input holding only a `kyverno-bundle.yaml`, only files a
writer excludes (see [What a writer excludes](#what-a-writer-excludes)), or nothing at all, doesn't produce a
bundle. This is a deliberate departure from today's push, which happily emits a zero-layer image for empty input.

**A conformant reader MUST bound extraction against a gzip decompression bomb.** A content layer's on-wire
(compressed) size gives no guarantee about its decompressed size, and the registry is writable by anyone with
push rights, so a reader that decompresses without a limit can be made to exhaust memory or disk from a small
blob. A reader MUST refuse, before accumulating the rejected bytes in memory:

- Any single entry whose decompressed size exceeds 10 MiB.
- A content layer whose total decompressed size, summed across all entries, exceeds 100 MiB.
- A content layer containing more than 10,000 entries.

Each rejection MUST name the limit that was hit and the offending entry. These are reader-side safety bounds, not
a writer-side format limit: a writer MAY emit a content layer within these bounds without needing to know they
exist, and format 1.0 places no smaller limit on what a writer produces.

### Determinism

**Byte-for-byte digest reproducibility in format 1.0 is defined per writer implementation, not across
independent implementations.** Given the same writer binary, the same version of that writer, the same input
tree, the same descriptor (or the absence of one), and the same `SOURCE_DATE_EPOCH` (set to the same value in
both runs, or unset in both), the writer MUST produce byte-identical content-layer, config, and manifest digests
every time. This format does not require byte-identical tar bytes — compressed or uncompressed — between two
different writer implementations: tar has more than one valid on-wire encoding for the same file metadata
(trailer padding length, PAX extended-header naming and emission threshold, device-field encoding on regular
files, and more all vary legitimately by implementation), so no set of metadata rules makes two independent tar
writers byte-identical, and this format doesn't claim one that would.

What a cross-implementation comparison — for example, checking that a second writer produces an equivalent
bundle to a first — MUST be able to verify instead is the **extracted entry set and its metadata**, not the raw
tar bytes: given two bundles produced by two different conformant writers from the same input tree (same
descriptor, same `SOURCE_DATE_EPOCH`), extracting both content layers under
[Round-trip guarantee](#round-trip-guarantee) MUST yield the same set of paths, the same file bytes at each path,
and the same mode, uid, gid, and mtime for each entry as reported by a standard tar reader. That's the checkable
cross-implementation guarantee this format makes; layer-digest equality across independent implementations isn't.

A writer, to hold up its own side of per-writer digest reproducibility:

- Sorts entries by path.
- Archives regular files only.
- Sets mode `0644` on every entry.
- Sets uid and gid to `0`, and uname and gname to empty strings.
- Sets mtime to Unix epoch 0 (`1970-01-01T00:00:00Z`) on every entry, as reported by a tar reader — not the
  wall-clock time the archive was built.
- Compresses the tar stream with gzip, with the header timestamp field (`MTIME`) set to 0 and no stored filename
  (`FNAME` unset).

A writer SHOULD use PAX or GNU long-name extensions for a path exceeding the 100-byte USTAR limit. This format
doesn't pin the tar variant (USTAR, PAX, or GNU) as a MUST: for an ordinary entry it isn't observable on the wire
(a compliant PAX writer emits no extended header at all unless a field overflows USTAR, so nothing in the archive
distinguishes it from a USTAR writer), and it isn't part of the cross-implementation guarantee stated above,
which is defined over extracted metadata, not archive-format internals.

`fluxcd/pkg/oci@v0.45.0/client/build.go` is the reference implementation for the entry-metadata rules this format
shares with it: zero `ModTime`, uid and gid `0`, entries visited in lexical order, and forward-slash names. This
format tightens that reference with a fixed file mode and a regular-files-only rule; it doesn't reuse that
function directly because it archives every file with no loader gate and produces no config.

Combined with the config's `created` field being omitted by default (see [6. Config blob](#6-config-blob) and
[4. Artifact structure](#4-artifact-structure)) and with the config and manifest never deriving fields from the
push reference (see [6. Config blob](#6-config-blob) and [4. Artifact structure](#4-artifact-structure)), a given
writer implementation and version, run twice against the same input tree with the same descriptor and the same
`SOURCE_DATE_EPOCH` setting, produces the same content-layer digest, config digest, and manifest digest both
times, regardless of which reference it's pushed to. A golden-digest test MUST fix the writer implementation and
version, not only the input tree; it MUST NOT assume a *different* writer implementation reproduces the same
digest.

### Round-trip guarantee

Extracting the content layer reproduces the set of archived files, byte-for-byte, at their original relative
paths. The config's `resources[].path` and `resources[].documentIndex` fields (see
[6. Config blob](#6-config-blob)) let a tool such as `kyverno oci inspect` or `kyverno oci validate` locate a
specific document without extracting the archive.

## 6. Config blob

The config blob, media type `application/vnd.cncf.kyverno.bundle.config.v1+json`, is a JSON document the writer
generates. It's not something an author writes by hand; the author-facing input is the descriptor (see
[7. Bundle descriptor](#7-bundle-descriptor)).

### Schema

| Field | Type | Required in 1.0 | Meaning |
|---|---|---|---|
| `formatVersion` | string (semver) | **Yes** | This format's version, for example `1.0.0` |
| `name` | string | No | Bundle name |
| `version` | string (semver) | No | Bundle version |
| `description` | string | No | Free-text description |
| `kyvernoVersion` | string (semver constraint, for example `">=1.20"`; `Masterminds/semver` dialect) | No | Kyverno version constraint the bundle targets |
| `source` | object (`url`, `revision`) | No | Where the bundle's source lives |
| `created` | string (RFC 3339) | No | Creation time; omitted unless set explicitly (see below) |
| `sets` | array of set objects | **Yes** | See "Set object" below |
| `resources` | array of resource-index entries | **Yes** | See "Resource-index entry" below |

A writer MUST reject a descriptor whose `kyvernoVersion` isn't a valid constraint in this dialect. A reader MUST
pass `kyvernoVersion` through as opaque metadata (reader MUST 5 already states that the config is authoritative
for compatibility) and MAY compare it against a Kyverno version. `kyverno oci validate` is a conformant reader
that additionally applies the writer-policy checks on top of its reader role, so it, unlike `kyverno oci pull`,
rejects a published bundle whose `kyvernoVersion` doesn't parse. `kyverno oci push` and `kyverno oci validate`
compare `kyvernoVersion` against a Kyverno version, as CLI behavior over the format, not a format-level
requirement: `kyverno oci push` and a plain `kyverno oci validate` compare it against the running CLI's own
version and warn on a mismatch; `kyverno oci validate --kyverno-version X` compares it against the explicit
target `X` instead and fails, rather than warns, if `X` doesn't satisfy the constraint.

**`formatVersion`, `sets`, and `resources` are the fields a writer MUST populate in format 1.0.** `name`,
`version`, `description`, `kyvernoVersion`, `source`, and `created` are optional, and a writer without a
descriptor MUST omit them rather than guess a value: a repository path is not reliably a bundle name and a tag is
not reliably a semver version, and deriving either from the push reference would make the config depend on where
the bundle happens to be pushed, defeating the digest-reproducibility guarantee in
[5. Content layer](#5-content-layer) (retagging or mirroring the same bundle would then change its config). A
writer MUST NOT derive any of these six fields from the push reference or from any environment value other than
the already-stated `SOURCE_DATE_EPOCH` handling for `created`.

`sets` is required, but it's still computable without a descriptor: **without a descriptor, a writer MUST assign
every resource to one of exactly two default sets, `policies` and `exceptions`, by kind** (`PolicyException`
resources go to `exceptions`; every other kind in [3. Supported resources](#3-supported-resources) goes to
`policies`). This default is a pure function of the archived content, not of the reference or the environment, so
it doesn't require descriptor-parsing logic to implement, and it keeps `sets` in the required column without
coupling the writer's minimum implementation to reading `kyverno-bundle.yaml`. See
[7. Bundle descriptor](#7-bundle-descriptor) for how a descriptor overrides this default.

`created` is **omitted unless** the descriptor sets `spec.created` or the environment sets `SOURCE_DATE_EPOCH`,
which is what keeps a default push's config digest reproducible (see
[5. Content layer](#5-content-layer)).

A reader MUST ignore fields it doesn't recognize, so a future minor version can add optional fields without
breaking a 1.0 reader.

### Set object

| Field | Type | Meaning |
|---|---|---|
| `name` | string | Set name |
| `type` | string, `policies` or `exceptions` | What kind of resource the set groups |

**Sets are format-level metadata, not layers or separate artifacts.** A bundle MAY contain policies, exceptions,
or both; the format doesn't require separating them. Each index entry names the set it belongs to, and each set
has a `type` of `policies` or `exceptions` in format 1.0. Whether a tool defaults to keeping policies and
exceptions in separate bundles, with an explicit option to mix them, is CLI behavior over this format, not a
constraint this format imposes. Whether that default means separate *bundles* or separate *sets within one
bundle* is a reading epic [#17645](https://github.com/kyverno/kyverno/issues/17645) still needs to confirm
(raised while implementing [#17663](https://github.com/kyverno/kyverno/issues/17663)). The sets-within-one-bundle
reading is already what `kyverno oci push` implements, and it's what the rest of this document and
[7. Bundle descriptor](#7-bundle-descriptor) assume.

A future minor version may add a new `type` value (see [8. Format versioning and compatibility](#8-format-versioning-and-compatibility)).
**A reader MUST accept a set whose `type` it doesn't recognize**, treating it as opaque metadata the same way it
ignores an unrecognized config field, rather than rejecting the bundle.

**Exception references to policies outside the bundle are legal at the format level.** A `PolicyException`'s
policy references don't have to resolve within the same bundle for the bundle to be well-formed; whether a given
tool resolves them against another bundle, and how, is validation-tooling behavior. `kyverno oci push` enforces
in-bundle resolution as its own strict default (see [9. Conformance](#9-conformance)), and this document records
that as the default a writer applies, not as a limit on what a well-formed bundle can contain.

### Resource-index entry

| Field | Type | Meaning |
|---|---|---|
| `apiVersion` | string | The resource's `apiVersion`, unmodified |
| `kind` | string | The resource's `kind` |
| `namespace` | string | The resource's `metadata.namespace`, exactly as written (see [3. Supported resources](#3-supported-resources), "Identity") |
| `name` | string | The resource's `metadata.name` |
| `path` | string | Relative path of the archived file this document came from |
| `documentIndex` | integer | Zero-based position of this document among the non-empty documents the reference splitter returns for that file (see "Digest input" below) |
| `digest` | string | `sha256:` followed by the hex digest of the document's bytes as the reference splitter returns them (see below) |
| `set` | string | Name of the set this resource belongs to; MUST name an entry in `sets[]` |

### Digest input: the reference splitter's output bytes

**The `sha256` digest is computed over a document's bytes exactly as returned by this format's reference document
splitter, not over a byte range read directly out of the archived file.** The reference splitter is
`ext/yaml.SplitDocuments` (`ext/yaml/split.go`), which wraps `k8s.io/apimachinery/pkg/util/yaml`'s `YAMLReader`.
This format defines document boundaries by delegating to that one implementation, rather than by stating an
independent byte-range rule, because the splitter does not simply slice the input between `---` lines: it
normalizes line endings to `\n`, appends a trailing `\n` to a final line that lacks one, and — the detail an
independently written byte-range rule gets wrong — when two document-separator lines appear back to back with
nothing but whitespace between them, the *second* separator line isn't consumed as a separator; it becomes
literal text at the start of the document that follows it. `ext/yaml/split_test.go` is the normative set of test
vectors for this behavior (its own test names flag the double-separator case as a documented quirk, and this
format takes that documented behavior as authoritative rather than redefining cleaner semantics of its own):

| Input | Digest inputs (documents, in order) |
|---|---|
| `enabled: true` (no trailing newline) | `enabled: true\n` |
| `enabled: true\n---\ndisabled: false` | `enabled: true\n`, then `disabled: false\n` |
| `enabled: true\n---\n---\ndisabled: false` | `enabled: true\n`, then `---\ndisabled: false\n` |

In the third row, the two consecutive separators do not produce a third, empty document: the first separator ends
the first document, and the second separator — because it arrives while the accumulator for the next document is
still empty — is written into that next document's bytes instead of being consumed. **A writer or a reader MUST
use this exact splitting behavior when computing a digest or a `documentIndex`; `ext/yaml.SplitDocuments` is the
reference implementation and `ext/yaml/split_test.go` is the conformance oracle for it**, not a hand-written
byte-range slice — the two disagree exactly in the cases above, and disagreement here means the digests a writer
and a reader compute for the same bytes differ, which fails the index cross-check in
[9. Conformance](#9-conformance) even though the underlying file is byte-for-byte identical on both sides.

**`documentIndex`** is the zero-based position of a document in the ordered list this splitter returns for its
file, **after** the splitter drops documents that are empty (blank lines or `#`-comment lines only, matching
`ext/yaml.IsEmptyDocument`). An empty document consumes no `documentIndex` value and contributes no `resources[]`
entry; a writer and a reader that both apply the reference splitter agree on `documentIndex` for every non-empty
document without needing any further rule.

## 7. Bundle descriptor

`kyverno-bundle.yaml` is an **optional, author-written input** at the bundle root. It's a Kubernetes-shaped
resource, following the shape the CLI's own `Test`, `Values`, and `UserInfo` types already use
(`cmd/cli/kubectl-kyverno/apis/v1alpha1`, group `cli.kyverno.io`):

```yaml
apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: my-policies
spec:
  version: 1.2.0
  description: Baseline policies for team A
  formatVersion: 1.0.0    # optional pin to an older supported format version
  kyvernoVersion: ">=1.20"
  source:
    url: https://github.com/example/policies
    revision: main
  created: "2026-09-28T00:00:00Z"   # optional; RFC 3339, or set SOURCE_DATE_EPOCH instead
  sets:
    - name: policies
      type: policies
      paths:
        - "policies/**"
    - name: exceptions
      type: exceptions
      paths:
        - "exceptions/**"
  annotations:
    example.com/team: team-a
```

`metadata.name` is the bundle name. `spec` fields (`version`, `description`, `formatVersion`, `kyvernoVersion`,
`source`, `created`, `sets`, `annotations`) map onto the config blob's fields of the same name (see
[6. Config blob](#6-config-blob)) and, where noted in [4. Artifact structure](#4-artifact-structure), onto
manifest annotations. `spec.created`, specifically, is the only way other than `SOURCE_DATE_EPOCH` to set the
config's `created` field and `org.opencontainers.image.created`; leaving both unset is what keeps a default push
reproducible, per [4. Artifact structure](#4-artifact-structure) and [6. Config blob](#6-config-blob).
`spec.sets[].paths` are glob patterns relative to the bundle root that assign archived resources to a named set,
overriding the by-kind default described in [6. Config blob](#6-config-blob). **This document fixes only the
format-level contract on set assignment, not the assignment algorithm itself**: every `resources[]` entry MUST
have a non-empty `set` value that names an entry in `sets[]` (see [6. Config blob](#6-config-blob), "Resource-index
entry"), and every `sets[]` entry SHOULD be named by at least one `resources[]` entry.

The remainder of this section states how `kyverno oci push` resolves `spec.sets[].paths`, as writer behavior over
this format, not as a new obligation on readers; `crossCheckIndex` and reader MUST 8 keep their existing meaning.

**Glob dialect.** `paths` entries compile with `gobwas/glob`, `/` as the path separator, matched against the
whole archived path. `*` and `?` don't cross a `/`; `**` does. `\` is an escape character, not a separator. A
pattern like `**/x.yaml` matches `team/x.yaml` but not a root-level `x.yaml`, because `**/` still requires at
least one path segment before the filename; write `{x.yaml,**/x.yaml}` to match both. A path MUST NOT be empty,
start with `/`, start with `./`, or end with `/`; a writer rejects each of these and, for the trailing-`/` case,
suggests appending `**` to match everything under that directory.

**Matching and type as a filter.** A set *matches* a document when any of its compiled patterns matches the
document's archived path. A set *claims* a document when it matches and the document's kind fits the set's
`type`: a `policies`-typed set claims only documents that aren't `PolicyException`; an `exceptions`-typed set
claims only documents that are. Treating `type` as a filter, not an assertion, lets one glob cover a
multi-document file that mixes a policy and its exception, each claimed by the typed set that fits it.

**Resolution rules:**

- A document claimed by zero sets falls back to the by-kind default (`policies` or `exceptions`) from
  [6. Config blob](#6-config-blob).
- A document claimed by two or more sets of fitting type is a push error naming the document and the sets.
- A set literally named `policies` MUST have `type: policies`; a set named `exceptions` MUST have
  `type: exceptions`.
- Every declared set MUST claim at least one indexed document, counted after type filtering; this applies to the
  two reserved default names as well as any other set name.

A mixed-type set (one set holding both policies and exceptions) isn't representable in format 1.0.0; see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility) for how a future minor version
could add one.

The descriptor itself follows these shape rules: it's exactly one non-empty YAML document; its `apiVersion` and
`kind` MUST be exactly `cli.kyverno.io/v1alpha1` and `PolicyBundle`; an unknown field anywhere in the document is
rejected; `metadata.namespace` and other `ObjectMeta` fields besides `metadata.name` are accepted and ignored;
`spec.created`, when set, MUST parse as RFC 3339 and is carried through to the config verbatim, not reformatted.
Only the exact filename `kyverno-bundle.yaml` at the bundle root is a descriptor — `kyverno-bundle.yml` is
ordinary content, and since its group is `cli.kyverno.io`, writer MUST 4 rejects it as a bundle-layout error, the
same way it rejects any other `cli.kyverno.io` document. A *directory* named `kyverno-bundle.yaml` isn't a
descriptor either; a writer treats it as ordinary content under that directory name, the same as if no descriptor
were present, so a tree that used that name for content before this section existed keeps working.

**Every string field under `metadata.name`, `metadata.annotations`, and `spec` MUST be quoted when it could
otherwise be read as a YAML number or boolean** (for example `version: "1.20"`, not `version: 1.20`; `version:
"yes"`, not `version: yes`): YAML parses an unquoted `1.20` as the number `1.2` and an unquoted `yes` as the
boolean `true`, either of which would otherwise reach the config blob silently reformatted. This applies
everywhere in that scope, including through a YAML alias or a `<<` merge key, not only to directly written
scalars, and it applies to both sides of an annotation entry: an unquoted annotation key (for example
`spec.annotations: {1.20: release}`) coerces exactly the same way an unquoted value does. A writer MUST reject
an unquoted numeric or boolean scalar anywhere in this scope, as a key or as a value, rather than publish the
reformatted value.

**Without a descriptor, a writer
MUST NOT derive `name`, `version`, `description`, `kyvernoVersion`, `source`, or `created` from the push
reference or the environment; it omits them, and falls back to the by-kind `policies`/`exceptions` default for
`sets`** (see [6. Config blob](#6-config-blob) for why `sets` alone can be required without a descriptor). This
keeps the config a pure function of the content tree and the descriptor, so the same bundle produces the same
config and manifest digests regardless of which reference it's pushed to; see
[5. Content layer](#5-content-layer).

### The descriptor is never archived

The descriptor filename is reserved and excluded from the content layer; see
[5. Content layer](#5-content-layer). `kyverno oci pull` restores the content layer only — the round-trip
guarantee is defined over the content layer, not the descriptor. Whether a reader also re-emits a normalized
descriptor on pull is tooling behavior, not a format requirement.

### Why the descriptor is Kubernetes-shaped, and what that costs

A Kubernetes-shaped descriptor gets embedded-CRD schema validation for free, the same way `Test`, `Values`, and
`UserInfo` do today. The cost is that `kyverno-bundle.yaml`, sitting in the bundle root alongside the resources it
describes, is a file a general-purpose Kubernetes tool doesn't know how to handle: `kubectl apply -f <dir>` and a
Flux `GitRepository` pointed at the raw source tree both fail with an error naming an unknown kind, because
neither has the CLI's embedded `PolicyBundle` CRD.

This specification's guidance: the OCI artifact — whose content layer never carries the descriptor — is the
intended GitOps input, not the raw source tree. An author who also wants to `kubectl apply` or Flux-reconcile the
raw tree directly should either list resources explicitly in a `kustomization.yaml` (kept out of the OCI bundle;
see [5. Content layer](#5-content-layer)) or leave the descriptor out, since it's optional.

Separately, `kyverno apply <bundle-root>` and `kyverno test` reading the raw source tree face the same problem
through the CLI's own general-purpose loader, which is not a format concern but is worth stating here since it
follows directly from the descriptor's shape: **today**, that loader returns a fatal error
(`policy type not supported`, from `cmd/cli/kubectl-kyverno/policy/load.go`'s `processDocumentItem`) for any
`cli.kyverno.io` document it encounters, and that error fails the whole directory load, not just the one file —
`kyverno apply <bundle-root>` skips **every** policy in the directory, logging one "skipping invalid YAML file"
entry for the directory as a whole, not one entry for the descriptor alone. A `kyverno-bundle.yaml` sitting in the
source tree therefore already breaks `apply` on the whole tree today, which is a stronger argument for a
loader-wide skip rule than a per-file skip would be, not a weaker one. That loader change, making the loader skip
`cli.kyverno.io` documents outright when reading a source tree for `apply` or `test`, is not yet implemented,
tracked under epic [#17645](https://github.com/kyverno/kyverno/issues/17645), the same way it isn't implemented
for how a `kyverno-test.yaml` sitting next to policies is handled today. That's CLI
read-path behavior over the source tree, not a statement about bytes in the registry or about what makes a bundle
valid, so it isn't a bundle conformance requirement — see [9. Conformance](#9-conformance).

## 8. Format versioning and compatibility

This format's version is a semver string, `1.0.0` for Kyverno 1.20. It's carried in three places: the config's
`formatVersion` field, the manifest annotation `io.kyverno.bundle.format-version`, and the `.v1` segment of both
media types (the major version only). It versions the **container**: manifest shape, media types, config schema,
archive layout rules, the identity rule, and annotation keys. It does **not** version bundle contents (the bundle
has its own `version`) or the accepted API-version table in [3. Supported resources](#3-supported-resources), which
`docs/context/shared/api-versioning.md` governs independently.

- **Major** version change: removing or renaming a config field, changing the archive layout or determinism rules,
  changing the identity rule, or changing the content layer's position or count.
- **Minor** version change: adding an optional config field or annotation, adding `artifactType`, or adding a new
  set type.
- **Patch** version change: a clarification with no byte-level effect.

### Compatibility guarantee

A reader that supports major version N reads any `N.x` bundle and ignores fields it doesn't recognize. A reader
MUST reject a bundle whose major version it doesn't support, with an error naming both the bundle's version and
the versions the reader supports. A writer emits the newest format version it supports unless the descriptor pins
`spec.formatVersion` to an older version the writer also supports; a writer MUST refuse a pin it can't honor.
**`spec.formatVersion`'s pin is a full semver string** (for example `1.0.0`, not `1.0` or `1`) and MUST match,
exactly, a version string in the writer's own list of supported format versions — a writer doesn't treat `1.0` as
a major.minor prefix that selects the newest matching patch release; if the writer supports `1.0.0` and the
descriptor pins `1.0`, that's a pin the writer can't honor and MUST refuse, the same as pinning an unsupported
version outright.

| Kyverno CLI release | Supported major format version |
|---|---|
| 1.20 | 1 (`1.0.0` is the only version this document defines; see [1. Status and scope](#1-status-and-scope)) |

### Legacy-image detection

A reader MUST detect a pre-1.20 image by its media types (see [4. Artifact structure](#4-artifact-structure)) and
fail with an actionable "legacy image format" error, rather than attempting to parse the image and failing with a
confusing downstream error. This applies equally to:

- **Released CLIs, 1.9 through 1.19.** Every image those CLIs produced carries the retired media types listed in
  [4. Artifact structure](#4-artifact-structure). No release of Kyverno ever produced a format-1.0 bundle before
  1.20, and there is no migration tool; an author republishes from source with a 1.20 or later CLI.
- **Unreleased `main` builds between #17661 landing and format 1.0 landing.** A `main` build in that window
  produced `cbbd8488c`'s per-resource-layer shape, restricted to CEL kinds, under the same media types as a
  released 1.19 CLI. No 1.20 release ever shipped that intermediate shape, so it gets the same "legacy image
  format" error, not special handling.

Retiring the old media types outright, instead of dual-publishing under both, is safe to the extent it's been
checked: there is **no known first-party publisher** of bundles under the retired media types anywhere in the
Kyverno organization as of this writing. That's a statement about what a code search across public repositories
can see, not a claim that no consumer exists anywhere — a private pipeline wouldn't show up in that search.

### Known limitation: pre-1.20 readers

A pre-1.20 CLI (1.9 through 1.19) skips any layer whose media type it doesn't recognize and, given a format-1.0
bundle, silently prints `Done.` with an empty output directory instead of failing. No choice this format makes can
change an already-shipped reader's behavior, and adding a legacy-typed tripwire layer to a format-1.0 bundle would
violate the single-layer rule in [4. Artifact structure](#4-artifact-structure). This is a stated limitation, not
a defect this format works around; the 1.20 release notes should carry the same statement.

## 9. Conformance

A numbered checklist a `kyverno oci validate` implementation checks, and conformance tests assert. Split by who
the requirement binds.

### Writer MUST

1. Emit an OCI 1.0 image manifest (`application/vnd.oci.image.manifest.v1+json`, no `artifactType`, no
   `subject`) with exactly one layer, at index 0, with media type
   `application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip` (4, 5).
2. Emit a config blob with media type `application/vnd.cncf.kyverno.bundle.config.v1+json`, `formatVersion` set,
   `sets[]` set (by descriptor or by the by-kind default; derived from the descriptor's `spec.sets[].paths` under
   the rules in [7. Bundle descriptor](#7-bundle-descriptor)), and `resources[]` populated for every archived
   document (6).
3. Archive only `.yaml`/`.yml` files, at their original relative paths and original bytes, excluding hidden
   paths and the reserved `kyverno-bundle.yaml` descriptor filename at the bundle root, and rejecting the same
   filename anywhere else under the bundle root as a bundle-layout error (5, 7).
4. Reject any document whose group is `cli.kyverno.io`, found anywhere in the bundle root, identified by
   inspecting each document's raw `apiVersion` before it reaches the resource loader — not by a document-count
   heuristic, which a `List`-plus-`Test` combination defeats (5).
5. Reject any resource whose kind isn't one of the 11 in [3. Supported resources](#3-supported-resources), or
   whose `apiVersion` isn't in the accepted-version table for the running release (3).
6. Reject a bundle containing two resources with the same identity, `Kind/namespace/name`, where `namespace` is
   `metadata.namespace` taken as written (3).
7. Reject any `v1 List` document, identified by the same raw `apiVersion`/`kind` inspection as item 4, before
   loader unwrapping (5).
8. Reject absolute paths, `..` segments, and duplicate archive paths; reject a symlink whose name would
   otherwise be archived content (a non-hidden `.yaml`/`.yml` path) or that resolves to a directory, regardless
   of its name; reject any other non-regular, non-symlink entry, such as a device entry — see
   [Archive rules](#archive-rules) for the full symlink rule and why a hard link isn't rejected. Reject an input
   tree that yields zero resources (5).
9. Produce, for a single writer implementation and version run repeatedly against the same input tree, the same
   descriptor, and the same `SOURCE_DATE_EPOCH` setting, byte-identical content-layer, config, and manifest
   digests: sorted paths, regular files only, mode `0644`, uid and gid `0`, empty uname and gname, mtime Unix
   epoch 0, gzip with a zeroed header timestamp and no stored filename. Two different writer implementations
   aren't required to produce identical tar bytes, compressed or uncompressed, only the same extracted entry set
   and metadata (mode, uid, gid, mtime, file bytes) at each path (5).
10. Omit `org.opencontainers.image.created` and the config's `created` field unless the descriptor sets
    `spec.created` or `SOURCE_DATE_EPOCH` is set (4, 6).
11. Compute every resource-index `digest` and `documentIndex` using the reference document splitter
    (`ext/yaml.SplitDocuments`), not an independently written byte-range or line-counting rule (6).
12. CEL-compile every policy before emitting the bundle. `kyverno oci push` additionally resolves every
    `PolicyException`'s policy references within the bundle as its own strict default (6); this is a writer
    policy, not a format-level requirement, because the format permits an exception to reference a policy outside
    the bundle.
13. Not derive `name`, `version`, `description`, `kyvernoVersion`, `source`, `created`, `io.kyverno.bundle.name`,
    `io.kyverno.bundle.version`, or `org.opencontainers.image.version` from the push reference or any
    environment value other than `SOURCE_DATE_EPOCH` for `created`; omit these fields and annotations when no
    descriptor supplies them (4, 6, 7).
14. Not let the descriptor's `spec.annotations` set any key under the reserved `io.kyverno.bundle.` or
    `org.opencontainers.image.` prefix — the whole prefix is reserved, not only the keys a writer currently
    computes; strip any such key from `spec.annotations` before merging it, so the writer-computed value for a
    key it does compute always wins, and a key under either prefix it does not compute is simply discarded (4).
15. Keep every `io.kyverno.bundle.*` and relevant `org.opencontainers.image.*` manifest annotation in agreement
    with the config blob's corresponding field; the config is canonical, annotations are duplicates for
    discoverability, and a writer MUST NOT emit an annotation the config contradicts (4).
16. Reject a descriptor whose `kyvernoVersion` isn't a valid semver constraint (6).

### Writer-policy checks

`kyverno oci push` and `kyverno oci validate` additionally apply these checks as their own strict defaults, not
as format requirements:

- Resolve every `PolicyException`'s policy references within the bundle (already covered by writer MUST 12).
- Reject a policy whose `spec.matchConstraints.namespaceSelector` or `objectSelector` isn't a well-formed label
  selector.
- Reject a `PolicyException` whose compensating-control `spec.validations` expressions fail to compile.
- Warn when a compensating control targets a kind that ignores it.
- Warn when a `PolicyException`'s `spec.expiresAt` is earlier than the wall clock at validation time;
  `kyverno oci validate --strict` turns this warning into a failure.

None of these checks bind a reader, and `kyverno oci pull` doesn't run them. Expiry is a warning, not a
rejection, for two reasons: the Kubernetes API server accepts an expired exception on create and the engine
simply ignores it, so a bundle's writer-policy check is stricter than the cluster it targets, not a mirror of
it; and an operator who pulls a vendor-authored bundle and re-pushes it to an internal registry, without
authority to edit its content, must still be able to push it even if an exception inside it has expired.
`SOURCE_DATE_EPOCH` is not the validation clock these checks use; the check always reads the wall clock. Whether
a selector matches any live resource isn't checkable offline, and these checks don't attempt it.

### Reader MUST

1. Require an OCI 1.0 image manifest (`schemaVersion` `2`, `mediaType`
   `application/vnd.oci.image.manifest.v1+json`) with exactly one layer, at index 0, with the content media type,
   and a config with the config media type; fail with the legacy-image error if either retired media type is
   present, or a malformed-bundle error otherwise (4, 8).
2. Extract the archive under the path rules in [5. Content layer](#5-content-layer), refusing any entry that would
   escape the target directory, and bounding extraction against a gzip decompression bomb: refuse any single
   entry whose decompressed size exceeds 10 MiB, refuse a content layer whose total decompressed size exceeds
   100 MiB, and refuse a content layer with more than 10,000 entries, naming the limit and the offending entry in
   the error (5).
3. Re-run the same validation a writer runs over the extracted documents: the raw `apiVersion`/`kind` rejection
   of `cli.kyverno.io` documents and `v1 List` documents (writer MUST 4 and 7 — a reader that instead hands
   extracted documents straight to a general-purpose loader and checks only the loader's output kinds can be
   fooled by a `List` the loader silently unwraps), the kind and version checks, the identity check, and, for a
   reader that will evaluate the policies (for example, `kyverno test oci://...` or `kyverno apply oci://...`),
   CEL compilation.
4. Cross-check the index against the archive: every archived document has exactly one index entry whose `path`,
   `documentIndex`, `digest`, `apiVersion`, `kind`, `namespace`, and `name` all match (the digest computed with
   the same reference splitter a writer uses, per writer MUST 11), and every index entry resolves to an archived
   document. Any mismatch — including an index entry that names a different `apiVersion`, `kind`, `namespace`, or
   `name` than the archived document it otherwise matches by path and digest — is a hard failure, not a warning.
   A reader MUST also reject an index entry whose `set` is empty or doesn't name an entry in the config's
   `sets[]` (see [6. Config blob](#6-config-blob), "Resource-index entry").
5. Treat the config as authoritative for bundle metadata (name, version, sets, compatibility) and the archive as
   authoritative for contents.
6. Reject a bundle whose major format version the reader doesn't support, naming both versions in the error.
7. Detect and reject a pre-1.20 (legacy) image by media type, per
   [8. Format versioning and compatibility](#8-format-versioning-and-compatibility).
8. Accept a set whose `type` value it doesn't recognize, treating it as opaque metadata rather than rejecting the
   bundle (6).

A reader MUST NOT trust the config alone and skip re-validating the archive: the registry is writable by anyone
with push rights and by tools other than `kyverno oci push`, so an index that disagrees with the archive's actual
contents is a tamper or tooling signal to reject, not a nuisance to ignore. This is what keeps `kyverno test
oci://...` and `kyverno apply oci://...` — the readers a hand-built or tampered manifest could otherwise fool — as
safe as the equivalent local-directory command.

## 10. Consumption by generic tooling

This chapter is informative, not normative. [`docs/dev/oci/flux-example.md`](./flux-example.md) is a worked,
digest-pinned example of consuming a bundle with Flux. A user-facing, end-to-end GitOps guide based on it is in
progress on the `kyverno/website` documentation as part of #17667.

### Flux `OCIRepository`

Flux's `OCIRepository` controller extracts the first layer of an artifact by default, and requires that layer to
be `tar+gzip` for its default `extract` operation. Because this format's single content layer sits at index 0 with
media type `application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip`, Flux's default configuration (no
`layerSelector`) already extracts it correctly. An `OCIRepository` SHOULD still set
`spec.layerSelector.mediaType: application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip` explicitly: it selects
today's content layer unambiguously instead of relying on Flux's `layers[0]` fallback, and it fails visibly
rather than silently extracting the wrong layer if the artifact ever carries more than one. This does not give
forward compatibility across a major format change: the major version is carried in the media type's `.v1`
segment, so a format version that adds a second layer ships a new `.v2` media type — see
[8. Format versioning and compatibility](#8-format-versioning-and-compatibility) — and a selector pinned to
`.v1` then matches nothing, requiring the `OCIRepository` to be updated by hand.

### `crane` and `oras`

Both tools fetch a manifest's config alongside the manifest itself in their default inspection commands (`crane
config`, `oras manifest fetch-config`), so a bundle's config blob (see [6. Config blob](#6-config-blob)) is visible
to either without an extra round trip.

### Digest pinning

A reference to a specific bundle SHOULD use its digest form (`<repo>@sha256:...`), not only a tag, wherever the
reference needs to be immutable — a tag can be moved to point at a different bundle; a digest can't. `kyverno
test` and `kyverno apply` accept the `oci://<ref>` scheme for either form. Because config and manifest content
never derives from the push reference (see [4. Artifact structure](#4-artifact-structure) and
[6. Config blob](#6-config-blob)), retagging or mirroring the same bundle to a different reference doesn't change
its digest, which is what makes the digest form meaningful as a stable identity independent of where the bundle
is hosted.

`kyverno oci resolve` is the way to obtain this digest. It applies reader MUST 1, 6, and 7
([9. Conformance](#9-conformance)) — manifest shape, config parse, and major-version and legacy-image checks —
before printing, so it never pins a non-bundle, a legacy pre-1.20 image, or an artifact whose format major
version this CLI doesn't support. It does not apply reader MUST 2-4: it never fetches or extracts the content
layer, so it can't check the archive against the index. A bundle whose manifest and config are well-formed but
whose archive is malformed or disagrees with the index passes `resolve` and still fails `pull`. `--digest-only`
prints the bare digest, the form `spec.ref.digest` on a Flux `OCIRepository` expects. For an artifact that isn't a
Kyverno bundle, `crane digest` is the equivalent way to pin a reference.

### Signing and referrers

Cosign and Notation both work against this format today, via the tag-scheme referrers convention any OCI 1.0
registry supports. OCI 1.1 `artifactType` and `subject`-based referrers aren't adopted by this format version; see
[4. Artifact structure](#4-artifact-structure).

## 11. Worked example

An input tree:

```
policies/
  require-labels.yaml
exceptions/
  team-a-exceptions.yaml
kyverno-bundle.yaml
```

`kyverno-bundle.yaml`:

```yaml
apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  version: 1.0.0
  sets:
    - name: policies
      type: policies
      paths: ["policies/**"]
    - name: exceptions
      type: exceptions
      paths: ["exceptions/**"]
```

Pushing this tree produces a manifest along these lines (digests below are placeholders, not real values):

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.cncf.kyverno.bundle.config.v1+json",
    "digest": "sha256:<config-digest>",
    "size": 512
  },
  "layers": [
    {
      "mediaType": "application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip",
      "digest": "sha256:<content-digest>",
      "size": 2048
    }
  ],
  "annotations": {
    "io.kyverno.bundle.format-version": "1.0.0",
    "io.kyverno.bundle.name": "baseline",
    "io.kyverno.bundle.version": "1.0.0",
    "org.opencontainers.image.title": "baseline",
    "org.opencontainers.image.version": "1.0.0"
  }
}
```

(`org.opencontainers.image.title` and `.version` are shown here because the descriptor supplies `metadata.name`
and `spec.version`; per [4. Artifact structure](#4-artifact-structure), a writer without those descriptor fields
would omit them rather than leave them blank.)

And a config blob:

```json
{
  "formatVersion": "1.0.0",
  "name": "baseline",
  "version": "1.0.0",
  "sets": [
    { "name": "policies", "type": "policies" },
    { "name": "exceptions", "type": "exceptions" }
  ],
  "resources": [
    {
      "apiVersion": "policies.kyverno.io/v1beta1",
      "kind": "ValidatingPolicy",
      "namespace": "",
      "name": "require-labels",
      "path": "policies/require-labels.yaml",
      "documentIndex": 0,
      "digest": "sha256:<resource-digest>",
      "set": "policies"
    },
    {
      "apiVersion": "policies.kyverno.io/v1beta1",
      "kind": "PolicyException",
      "namespace": "team-a",
      "name": "check-pod",
      "path": "exceptions/team-a-exceptions.yaml",
      "documentIndex": 0,
      "digest": "sha256:<resource-digest>",
      "set": "exceptions"
    }
  ]
}
```

The content layer, listed with `tar -tzf`:

```
exceptions/team-a-exceptions.yaml
policies/require-labels.yaml
```

The descriptor, `kyverno-bundle.yaml`, isn't in that listing: it's an input, not an archived file.

## 12. Changelog

- `1.0.0`: first version of this format, replacing the per-resource-layer format Kyverno 1.9 through 1.19 shipped.
  - Set-derivation rules for the descriptor ([7. Bundle descriptor](#7-bundle-descriptor)) specified by #17663.
    This has no byte-level effect on any config a 1.0.0 reader already accepts. It doesn't take a patch bump
    because 1.0.0 hasn't shipped in a release yet; the first released version carries these rules from the
    start.
  - Accepted-version table widened to `v1`, `kyvernoVersion` constraint semantics, and the writer-policy checks
    specified by #17664. No byte-level effect on any 1.0.0 bundle; no bump because 1.0.0 hasn't shipped in a
    release yet.
  - `kyverno oci` graduated out of the experimental gate: it no longer requires `KYVERNO_EXPERIMENTAL`.
    [1. Status and scope](#1-status-and-scope) records this format as committed as of 1.20. No byte-level effect
    on any 1.0.0 bundle; no bump because 1.0.0 hasn't shipped in a release yet.
  - `kyverno oci resolve` and the Flux example cross-reference added to
    [10. Consumption by generic tooling](#10-consumption-by-generic-tooling) by #17667. Informative only; no
    normative or byte-level change.
