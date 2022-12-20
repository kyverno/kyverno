## Description

The configmap context lookup uses informer's cache internally, the background processing should use the same to resolve configmap context without crashing Kyverno.

This is the second test for configmap lookup, see `test/conformance/kuttl/validate/clusterpolicy/standard/audit/configmap-context-lookup/README.md` for another.

## Expected Behavior

Policy is expected to be successfully created AND not result in an internal panic.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5704