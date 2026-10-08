# Namespaced image policy credential boundaries

Run against a cluster with Kyverno and NamespacedImageValidatingPolicy installed:

```sh
chainsaw test --test-dir test/conformance/chainsaw/namespaced-image-validating-policies/credential-scope
```

The admission webhook accepts bare and same-namespace Secret references and rejects
foreign and installation-namespace references in both policy credentials and
attestor signature pull secrets. No registry credentials or network requests are
needed. Unit tests exercise runtime dynamic attestors and authorized tenant Secret
GETs. The integration test in `test/integration/ivpol/credential_scope_test.go`
checks policies stored before the fix through the real reconciler and resource
admission handler.
