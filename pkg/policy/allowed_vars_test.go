package policy

import (
	"fmt"
	"testing"

	ut "github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestNotAllowedVars_MatchSection(t *testing.T) {
	var policyWithVarInMatch = []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "enforce-pod-name"
    },
    "spec": {
      "validationFailureAction": "audit",
      "background": true,
      "rules": [
      {
        "name": "validate-name",
        "match": {
        "resources": {
          "kinds": [
          "Pod"
          ],
          "namespaceSelector": {
          "matchExpressions": [
            {
            "key": "{{very.unusual.variable.here}}",
            "operator": "In",
            "values": [
              "managed"
            ]
            }
          ]
          }
        }
        },
        "validate": {
        "message": "The Pod must end with -nginx",
        "pattern": {
          "metadata": {
          "name": "*-nginx"
          }
        }
        }
      }
      ]
    }
    }
  `)

	policy, err := ut.GetPolicy(policyWithVarInMatch)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"validate-name\" should not have variables in match section")
}

func TestNotAllowedVars_ExcludeSection(t *testing.T) {
	var policyWithVarInExclude = []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "enforce-pod-name"
    },
    "spec": {
      "validationFailureAction": "audit",
      "background": true,
      "rules": [
      {
        "name": "validate-name",
        "exclude": {
        "resources": {
          "kinds": [
          "Pod"
          ],
          "namespaceSelector": {
          "matchExpressions": [
            {
            "key": "value",
            "operator": "In",
            "values": [
              "{{very.unusual.variable.here}}"
            ]
            }
          ]
          }
        }
        },
        "validate": {
        "message": "The Pod must end with -nginx",
        "pattern": {
          "metadata": {
          "name": "*-nginx"
          }
        }
        }
      }
      ]
    }
    }
  `)

	policy, err := ut.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"validate-name\" should not have variables in exclude section")
}

func TestNotAllowedVars_ExcludeSection_PositiveCase(t *testing.T) {
	var policyWithVarInExclude = []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "enforce-pod-name"
    },
    "spec": {
      "validationFailureAction": "audit",
      "background": true,
      "rules": [
      {
        "name": "validate-name",
        "exclude": {
        "resources": {
          "kinds": [
          "Pod"
          ],
          "namespaceSelector": {
          "matchExpressions": [
            {
            "key": "value",
            "operator": "In",
            "values": [
              "value1",
              "value2"
            ]
            }
          ]
          }
        }
        },
        "validate": {
        "message": "The Pod must end with -nginx",
        "pattern": {
          "metadata": {
          "name": "*-nginx"
          }
        }
        }
      }
      ]
    }
    }
  `)

	policy, err := ut.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath(t *testing.T) {
	var policyWithVarInExclude = []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "policy-patch-cm"
    },
    "spec": {
      "rules": [
      {
        "name": "pCM1",
        "match": {
        "resources": {
          "name": "config-game",
          "kinds": [
          "ConfigMap"
          ]
        }
        },
        "mutate": {
        "patchesJson6902": "- path: \"{{request.object.path.root}}/data/ship.properties\"\n  op: add\n  value: |\n    type=starship\n    owner=utany.corp\n- path: \"/data/newKey1\"\n  op: add\n  value: newValue1"
        }
      }
      ]
    }
    }`)

	policy, err := ut.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"pCM1\" should not have variables in patchesJSON6902 path section")
}

func TestNotAllowedVars_JSONPatchPath_PositiveCase(t *testing.T) {
	var policyWithVarInExclude = []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "policy-patch-cm"
    },
    "spec": {
      "rules": [
      {
        "name": "pCM1",
        "match": {
        "resources": {
          "name": "config-game",
          "kinds": [
          "ConfigMap"
          ]
        }
        },
        "mutate": {
        "patchesJson6902": "- path: \"/data/ship.properties\"\n  op: add\n  value: |\n    type={{request.object.starship}}\n    owner=utany.corp\n- path: \"/data/newKey1\"\n  op: add\n  value: newValue1"
        }
      }
      ]
    }
    }`)

	policy, err := ut.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath_PositiveCaseWithValue(t *testing.T) {
	var policyYAML = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: mutate-ingress-host
spec:
  rules:
  - name: mutate-rules-host
    match:
      resources:
        kinds:
        - Ingress
        namespaces:
        - test-ingress
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "{{request.object.spec.rules[0].host}}.mycompany.com"
`)

	policyJSON, err := yaml.ToJSON(policyYAML)
	assert.NilError(t, err)

	policy, err := ut.GetPolicy(policyJSON)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_VariableFormats(t *testing.T) {
	tcs := []struct {
		name  string
		input string
		pass  bool
	}{
		{"invalid_var", "not a valid variable", false},
		{"request_object", "request.object.meta", true},
		{"service_account_name", "serviceAccountName", true},
		{"service_account_namespace", "serviceAccountNamespace", true},
		{"self", "@", true},
		{"custom_func_compare", "compare(string, string)", true},
		{"custom_func_contains", "contains(string, string)", true},
		{"custom_func_equal_fold", "equal_fold(string, string)", true},
		{"custom_func_replace", "replace(str string, old string, new string, n float64)", true},
		{"custom_func_replace_all", "replace_all(str string, old string, new string)", true},
		{"custom_func_to_upper", "to_upper(string)", true},
		{"custom_func_to_lower", "to_lower(string)", true},
		{"custom_func_trim", "trim(str string, cutset string)", true},
		{"custom_func_split", "split(str string, sep string)", true},
		{"custom_func_regex_replace_all", "regex_replace_all(regex string, src string|number, replace string|number)", true},
		{"custom_func_regex_replace_all_literal", "regex_replace_all_literal(regex string, src string|number, replace string|number)", true},
		{"custom_func_regex_match", "regex_match(string, string|number)", true},
		{"custom_func_label_match", "label_match(object, object)", true},
		{"abs", "abs(foo, bar)", true},
		{"avg", "avg(foo, bar)", true},
		{"contains", "contains(foo, bar)", true},
		{"ceil", "ceil(foo, bar)", true},
		{"ends_with", "ends_with(foo, bar)", true},
		{"floor", "floor(foo, bar)", true},
		{"join", "join(foo, bar)", true},
		{"keys", "keys(foo, bar)", true},
		{"length", "length(foo, bar)", true},
		{"map", "map(foo, bar)", true},
		{"max", "max(foo, bar)", true},
		{"max_by", "max_by(foo, bar)", true},
		{"merge", "merge(foo, bar)", true},
		{"min", "min(foo, bar)", true},
		{"min_by", "min_by(foo, bar)", true},
		{"not_null", "not_null(foo, bar)", true},
		{"reverse", "reverse(foo, bar)", true},
		{"sort", "sort(foo, bar)", true},
		{"sort_by", "sort_by(foo, bar)", true},
		{"starts_with", "starts_with(foo, bar)", true},
		{"sum", "sum(foo, bar)", true},
		{"to_array", "to_array(foo, bar)", true},
		{"to_string", "to_string(foo, bar)", true},
		{"to_number", "to_number(foo, bar)", true},
		{"type", "type(foo, bar)", true},
		{"values", "values(foo, bar)", true},
		{"self_path_test", "@", true},
	}

	for _, tc := range tcs {
		var policyYAML = []byte(fmt.Sprintf(`
    apiVersion: kyverno.io/v1
    kind: ClusterPolicy
    metadata:
      name: mutate-ingress-host
    spec:
      rules:
      - name: mutate-rules-host
        match:
          resources:
            kinds:
            - Ingress
            namespaces:
            - test-ingress
        preconditions:
          any:
            - key: "{{ %s }}"
              operator: NotEquals
              value: ""
        mutate:
          patchesJson6902: |-
            - op: replace
              path: /spec/rules/0/host
              value: "foo.com"
    `, tc.input))

		policy, err := ut.GetPolicy(policyYAML)
		assert.NilError(t, err)

		err = hasInvalidVariables(policy[0], false)
		if tc.pass {
			assert.NilError(t, err, "%s: not expecting an error", tc.name)
		} else {
			assert.Assert(t, err != nil, "%s: was expecting an error", tc.name)
		}
	}
}
