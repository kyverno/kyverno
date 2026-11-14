# AGENTS.md — pkg/utils/api

`image.go` extracts the images a resource uses into `map[category]map[key]ImageInfo`. The legacy kyverno.io
engine that used to call it was deleted in #18265, but the extraction is still live: `MutateDigest` for
ImageValidatingPolicy (`pkg/image/verification/evaluator/policy.go`) builds its JSON patches from it, and is its
only non-test caller. Read this before changing the extractors.

## The categories and kinds mirror the built-in CEL extractors

`BuildStandardExtractors` produces `initContainers`, `containers`, `ephemeralContainers` and `imageVolumes`, the
same names the built-in CEL extractors use in `pkg/cel/compiler/images.go`, and `registeredExtractors` covers the
same eight kinds as `getImageExtractorsFromGVR` there (Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet,
ReplicationController, Job, CronJob). The two sides split one job: CEL decides which images a policy matches and
checks (`validationConfigurations.required`, `verifyDigest`), this package decides which of them `mutateDigest`
can pin. **Change a field or a kind on one side only and a policy with both `verifyDigest` and `mutateDigest`
refuses images it was told to pin**: CEL sees the tag, nothing here patches it. Change both together.

## Keys and pointers

Container categories are keyed by the container's `name`. `imageVolumes` is keyed by the JSON pointer of the
reference (`/spec/.../volumes/<i>/image/reference`), because the volume's `image` object has no name of its own;
an extractor with an empty `Key` falls back to the pointer in `extract`. A null `volumes` list, and a volume with no
`image`, a null `image` or no non-blank `reference`, are skipped: `extract` returns early on a nil object and on a
missing or blank value.
Kubernetes requires `reference` only on Pods, so workload templates can legitimately leave it out; the CEL
`imageVolumes` filters skip the same volumes rather than fail the whole resource. `ImageInfo.Pointer` is the path
`MutateDigest` replaces, so it must name the string that holds the reference, never its parent object.

## An unparseable reference fails the whole extraction

`extract` returns `invalid image` for any value `imageutils.GetImageInfo` rejects, and `ExtractImagesFromResource`
returns that error with no partial result, so `MutateDigest` pins nothing on that resource. This applies to image
volume references exactly as to container images; do not make one category lenient on its own.
