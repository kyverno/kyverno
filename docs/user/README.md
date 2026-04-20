# docs

This folder contains the generated CRD documentation in HTML format. It is referenced from the Kyverno website (https://kyverno.io/docs/crds/).

> **Note for contributors:** Generated HTML files in subdirectories (such as `crd/`) should not be edited manually. For documentation changes, edit the source files in `docs/user/`.

## Building

Follow these steps to generate the docs:

1. Run `make codegen-api-docs`

2. Commit / push the results to git

3. If needed, update the [docs site](https://kyverno.io/docs/crds/).