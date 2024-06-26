## Description

The configmap context lookup uses informer's cache internally, the background processing should use the same to resolve configmap context without crashing Kyverno.

## Expected Behavior

Policy is created successfully and the report is generated properly.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5704