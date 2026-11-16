## Description

This test verifies that a pod creation request is validated by performing an HTTP POST call via Kyverno's cel lib `http.post`, and that the call carries the projected ServiceAccount token mounted in the Kyverno pod (`/var/run/secrets/kyverno/apicall/token`, audience `kyverno-svc.kyverno.io`).

The endpoint pod reads the `Authorization` header of the incoming POST and asks the Kubernetes TokenReview API whether that token is authenticated for the `kyverno-svc.kyverno.io` audience. It answers `{"tokenValid": true}` only when TokenReview confirms both, and `{"tokenValid": false}` otherwise. The endpoint pod authenticates to the API server with its own service-account token and only holds `tokenreviews/create`.

## Expected Behavior

- `good-pod` (label `token-test=good`) is matched by `vpol-http-post-token`, which calls `http.post` without an `Authorization` header. The call must carry the projected token, so the endpoint reports `tokenValid=true` and the pod is admitted.
- `bad-pod` (label `token-test=bad-auth`) is matched by `vpol-http-post-token-bad-auth`, which calls `http.post` with an explicit `Authorization: Bearer invalid` header. The token is not valid for the audience, so the endpoint reports `tokenValid=false` and the pod is denied. This is the negative case: a missing or invalid token must not produce the success response.

## Note

No Kubernetes cluster was available in the environment where this fixture was written, so it has not been run. The YAML, the policy expressions and the endpoint's TokenReview request were checked locally, but the in-cluster behaviour is unverified and the maintainers' conformance run is the first execution.
