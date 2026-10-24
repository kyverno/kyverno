# Runtime-created custom-resource global context

Regression coverage for #15828. Run against an already deployed, healthy Kyverno
installation in the `kyverno` namespace:

```sh
chainsaw test --test-dir test/conformance/chainsaw/globalcontext/runtime-custom-resource
```

The test creates the CRD, backing objects, and GlobalContextEntry after controller
startup. A ValidatingPolicy accepts ConfigMaps whose value is present in the
cached custom resources and rejects an absent value. A second custom resource
proves live watch updates. Controller pod UIDs and container restart counts must
remain unchanged. Admission attempts use Chainsaw's bounded apply retries rather
than fixed sleeps.

The Go controller regression separately records the backing GVR's LIST and WATCH
requests. This cluster test proves admission behavior; it does not inspect API
server audit records or assert the contents of every controller replica's store.
