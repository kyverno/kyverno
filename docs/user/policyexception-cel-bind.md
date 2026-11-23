<!-- coderabbit-ai-generated:v1 -->

# CEL bindings in PolicyException match conditions

Kyverno admission accepts `cel.bind` in `policies.kyverno.io` `PolicyException` `spec.matchConditions`, including when an update leaves an existing expression unchanged. A name defined by `cel.bind` is visible only within the binding's body. This aligns admission validation with the existing CLI and policy engine behavior.

<!-- TODO: Add a worked example to the Kyverno website's Policy Exceptions guide for [kyverno/kyverno#18764](https://github.com/kyverno/kyverno/pull/18764). -->
