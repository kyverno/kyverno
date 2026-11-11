# Image policy credential compatibility

Run `kyverno test test/cli/test-image-validating-policy/credential-scope`.

The accepted controls exercise bare and explicit tenant Secret names and retain
cluster-policy foreign Secret references, for both policy credentials and attestor
pull secrets. The policies match no registry image, so the test needs no external
registry or real Secret. Rejected namespace references are exercised by the
Chainsaw admission fixtures and Go compiler/runtime tests.
