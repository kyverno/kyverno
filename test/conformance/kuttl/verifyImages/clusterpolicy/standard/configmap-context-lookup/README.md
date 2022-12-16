## Description

The configmap context lookup uses informer's cache internally, the background processing should use the same to resolve configmap context without crashing Kyverno.

## Expected Behavior

Policy creation should not cause Kyverno to panic.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5704