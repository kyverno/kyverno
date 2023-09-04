package test

var longHelp = `
The test command provides a facility to test resources against policies by comparing expected results, declared ahead of time in a test manifest file, to actual results reported by Kyverno. Users provide the path to the folder containing a kyverno-test.yaml file where the location could be on a local filesystem or a remote git repository.
`

var exampleHelp = `
# Test a git repository containing Kyverno test cases.
kyverno test https://github.com/kyverno/policies/pod-security --git-branch main
<snip>

Executing require-non-root-groups...
applying 1 policy to 2 resources...

│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
│ # │ POLICY                  │ RULE                     │ RESOURCE                         │ RESULT │
│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
│ 1 │ require-non-root-groups │ check-runasgroup         │ default/Pod/fs-group0            │ Pass   │
│ 2 │ require-non-root-groups │ check-supplementalGroups │ default/Pod/fs-group0            │ Pass   │
│ 3 │ require-non-root-groups │ check-fsGroup            │ default/Pod/fs-group0            │ Pass   │
│ 4 │ require-non-root-groups │ check-supplementalGroups │ default/Pod/supplemental-groups0 │ Pass   │
│ 5 │ require-non-root-groups │ check-fsGroup            │ default/Pod/supplemental-groups0 │ Pass   │
│ 6 │ require-non-root-groups │ check-runasgroup         │ default/Pod/supplemental-groups0 │ Pass   │
│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
<snip>

# Test a local folder containing test cases.
kyverno test .

Executing limit-containers-per-pod...
applying 1 policy to 4 resources...

│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│
│ # │ POLICY                   │ RULE                                 │ RESOURCE                    │ RESULT │
│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│
│ 1 │ limit-containers-per-pod │ limit-containers-per-pod-bare        │ default/Pod/myapp-pod-1     │ Pass   │
│ 2 │ limit-containers-per-pod │ limit-containers-per-pod-bare        │ default/Pod/myapp-pod-2     │ Pass   │
│ 3 │ limit-containers-per-pod │ limit-containers-per-pod-controllers │ default/Deployment/mydeploy │ Pass   │
│ 4 │ limit-containers-per-pod │ limit-containers-per-pod-cronjob     │ default/CronJob/mycronjob   │ Pass   │
│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│

Test Summary: 4 tests passed and 0 tests failed

# Test some specific test cases out of many test cases in a local folder.
kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

Executing test-simple...
applying 1 policy to 1 resource...

│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│
│ # │ POLICY              │ RULE              │ RESOURCE                                │ RESULT │
│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│
│ 1 │ disallow-latest-tag │ require-image-tag │ default/Pod/test-require-image-tag-pass │ Pass   │
│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│

Test Summary: 1 tests passed and 0 tests failed



**TEST FILE STRUCTURE**:

The kyverno-test.yaml has four parts:
	"policies"   --> List of policies which are applied.
	"resources"  --> List of resources on which the policies are applied.
	"variables"  --> Variable file path containing variables referenced in the policy (OPTIONAL).
	"results"    --> List of results expected after applying the policies to the resources.

** TEST FILE FORMAT**:

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
  patchedResource: <path/to/patched/resource.yaml> (For mutate policies/rules only)
  result: <pass|fail|skip>

**VARIABLES FILE FORMAT**:

policies:
- name: <policy_name>
  rules:
  - name: <rule_name>
    # Global variable values
    values:
      foo: bar
  resources:
  - name: <resource_name_1>
    # Resource-specific variable values
    values:
      foo: baz
  - name: <resource_name_2>
    values:
      foo: bin
# If policy is matching on Kind/Subresource, then this is required
subresources:
  - subresource:
      name: <name of subresource>
      kind: <kind of subresource>
      group: <group of subresource>
      version: <version of subresource>
    parentResource:
      name: <name of parent resource>
      kind: <kind of parent resource>
      group: <group of parent resource>
      version: <version of parent resource>

**RESULT DESCRIPTIONS**:

pass  --> The resource is either validated by the policy or, if a mutation, equals the state of the patched resource.
fail  --> The resource fails validation or the patched resource generated by Kyverno is not equal to the input resource provided by the user.
skip  --> The rule is not applied.

For more information visit https://kyverno.io/docs/kyverno-cli/#test
`
