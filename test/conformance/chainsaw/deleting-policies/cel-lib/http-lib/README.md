# DeletingPolicy Test — HTTP Library Integration

This test validates Kyverno's support for the `http` custom CEL library within a `DeletingPolicy`. It ensures that external HTTP GET requests can be used as part of condition expressions to decide whether or not to delete a resource.

---

## Test Objective

Delete a `Pod` only if an HTTP GET request to a remote URL returns a matching value.

Specifically, the policy deletes the pod **only if**:

```cel
http.Get("http://test-api-service.default.svc.cluster.local:80").metadata.labels.app == "test"
```