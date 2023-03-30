# docs

This folder containers the generated CRD documentation in HTML format. It is referenced from the Kyverno website (https://kyverno.io/docs/crds/).

## Building

Follow these steps to generate the docs:

1. Run `make codegen-api-docs`

2. Commit / push the results to git

3. If needed, update the [docs site](https://kyverno.io/docs/crds/).