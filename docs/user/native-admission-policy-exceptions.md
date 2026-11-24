<!-- coderabbit-ai-generated:v1 -->

# Blanket exceptions for generated native admission policies

<!-- TODO(kyverno/kyverno#18709): Update the Policy Exceptions documentation on the Kyverno website with an example of a blanket exception for a policy with native admission policy generation enabled. -->

A CEL `PolicyException` with no `spec.matchConditions`, `spec.images`, or `spec.allowedValues` exempts every request from a referenced Kyverno `ValidatingPolicy` or `MutatingPolicy`. When Kyverno generates a `ValidatingAdmissionPolicy` or `MutatingAdmissionPolicy` for that policy, it adds a match condition with the expression `false`. This prevents the generated policy from applying to any request.

Context: [kyverno/kyverno#18709](https://github.com/kyverno/kyverno/pull/18709).
