# Namespaced generate scope

This test keeps three admission and controller checks for namespaced `Policy`:
a foreign clone source namespace is rejected, a reference cannot copy a dynamic
name into the target kind, and a valid same-namespace clone produces the expected
ConfigMap. ConfigMap contents are ordinary test values.

The field and expression matrix belongs to Go tests:
`api/kyverno/v1/rule_test.go` covers every dynamic scope field, direct and foreach
generation, variables, references, and accepted dynamic names.
`api/kyverno/v1/generate_clone_test.go` covers same, foreign, empty, and variable
clone source namespaces. `pkg/policy/generate/scope_reference_test.go` exercises
reference substitution through offline policy validation.

`pkg/background/generate/clone_source_test.go` exercises stored policies after
substitution, including cloneList and later foreach elements. It verifies that
forbidden source reads, lists, and updates never reach the client.
