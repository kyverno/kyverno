## Description

This test checks a POST operation to the Kubernetes API server for a SubjectAccessReview. It checks for delete access to the namespace of the request, and allows or denies the request.

## Expected Behavior

The test resource should be allowed to be created in the test namespace but not in the `default` namespace, as Kyverno cannot delete it.

## Reference Issues

https://github.com/kyverno/kyverno/issues/1717

https://github.com/kyverno/kyverno/issues/6857
