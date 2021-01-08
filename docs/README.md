# docs

This folder containers the generated CRD documentation in HTML format. It is referenced from the Kyverno website (https://kyverno.io/docs/crds/).

## Building

Follow these steps to generate the docs:

1. Install [gen-crd-api-reference-docs](https://github.com/ahmetb/gen-crd-api-reference-docs)

```shell
clone https://github.com/ahmetb/gen-crd-api-reference-docs
cd gen-crd-api-reference-docs
go build
mv gen-crd-api-reference-docs $GOPATH/bin
```

2. Generate the HTML

```shell
gen-crd-api-reference-docs -api-dir ./pkg/api/kyverno/v1 \
    -config docs/config.json  \
    -template-dir docs/template/ \
    -out-file docs/crd/v1/index.html
```

3. If needed, update the [docs site](https://kyverno.io/docs/crds/).