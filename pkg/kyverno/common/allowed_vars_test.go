package common

import (
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

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.Error(t, err, "Rule \"validate-name\" should not have variables in match section")
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

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.Error(t, err, "Rule \"validate-name\" should not have variables in exclude section")
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

	err = PolicyHasNonAllowedVariables(*policy[0])
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

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.Error(t, err, "Rule \"pCM1\" should not have variables in patchesJSON6902 path section")
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

	err = PolicyHasNonAllowedVariables(*policy[0])
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

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.NilError(t, err)
}

func TestNotAllowedVars_InvalidValue(t *testing.T) {
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
    preconditions:
      any:
        - key: "{{ not a valid variable }}"
          operator: NotEquals
          value: ""
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "foo.com"
`)

	policy, err := ut.GetPolicy(policyYAML)
	assert.NilError(t, err)

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.Assert(t, err != nil)
}

func TestNotAllowedVars_Functions(t *testing.T) {
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
    preconditions:
      any:
        - key: '{{ join(",", [1,2,3]) }}'
          operator: NotEquals
          value: ""
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "foo.com"
`)

	policy, err := ut.GetPolicy(policyYAML)
	assert.NilError(t, err)

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.NilError(t, err)
}

func TestNotAllowedVars_CustomFunctions(t *testing.T) {
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
    preconditions:
      any:
        - key: '{{ split("1,2,3", ",")  }}'
          operator: NotEquals
          value: ""
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "foo.com"
`)

	policy, err := ut.GetPolicy(policyYAML)
	assert.NilError(t, err)

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.NilError(t, err)
}

func TestNotAllowedVars_FunctionsUnderscore(t *testing.T) {
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
    preconditions:
      any:
        - key: '{{ to_upper("Hello, world") }}'
          operator: Equals
          value: "HELLO, WORLD"
    mutate:
      patchesJson6902: |-
        - op: replace
          path: /spec/rules/0/host
          value: "foo.com"
`)

	policy, err := ut.GetPolicy(policyYAML)
	assert.NilError(t, err)

	err = PolicyHasNonAllowedVariables(*policy[0])
	assert.NilError(t, err)
}
