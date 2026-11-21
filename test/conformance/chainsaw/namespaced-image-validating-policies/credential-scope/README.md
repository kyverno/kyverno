# Namespaced image policy credential boundaries

Run against a cluster with Kyverno and NamespacedImageValidatingPolicy installed.
Run it from the suite directory so `.chainsaw.yaml` applies, as CI does:

```sh
cd test/conformance/chainsaw/namespaced-image-validating-policies
chainsaw test --config .chainsaw.yaml credential-scope
```

Note that commenting `/conformance` on a pull request does **not** exercise this
directory: `.github/workflows/comment-conformance.yaml` checks out the default
branch rather than the PR ref, so it builds and tests `main`. The suite runs for
real post-merge via `.github/workflows/check-tests.yaml`.

The admission webhook accepts bare and same-namespace Secret references and rejects
foreign and installation-namespace references in both policy credentials and
attestor signature pull secrets. No registry credentials or network requests are
needed. Unit tests exercise runtime dynamic attestors and authorized tenant Secret
GETs. The integration test in `test/integration/ivpol/credential_scope_test.go`
checks policies stored before the fix through the real reconciler and resource
admission handler.
