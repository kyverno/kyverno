package manifest

func PrintValidate() {
	print(`
name: <test_name>
policies:
  - <path/to/policy1.yaml>
  - <path/to/policy2.yaml>
resources:
  - <path/to/resource1.yaml>
  - <path/to/resource2.yaml>
variables: <variable_file> (OPTIONAL)
results:
  - policy: <name> (For Namespaced [Policy] files, format is <policy_namespace>/<policy_name>)
    rule: <name>
    resource: <name>
    namespace: <name> (OPTIONAL)
    kind: <name>
    patchedResource: <path/to/patched/resource.yaml>
    result: <pass|fail|skip>`)
}
