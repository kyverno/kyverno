package policy

import (
	"fmt"
	"strings"
	"testing"

	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestNotAllowedVars_MatchSection(t *testing.T) {
	policyWithVarInMatch := []byte(`{
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyWithVarInMatch)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"validate-name\" should not have variables in match section")
}

func TestNotAllowedVars_ExcludeSection(t *testing.T) {
	policyWithVarInExclude := []byte(`{
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"validate-name\" should not have variables in exclude section")
}

func TestNotAllowedVars_ExcludeSection_PositiveCase(t *testing.T) {
	policyWithVarInExclude := []byte(`{
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath(t *testing.T) {
	policyWithVarInExclude := []byte(`{
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.Error(t, err, "rule \"pCM1\" should not have variables in patchesJSON6902 path section")
}

func TestNotAllowedVars_JSONPatchPath_ContextRootPositive(t *testing.T) {
	policyManifest := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "policy-patch-cm"
    },
    "spec": {
      "rules": [
      {
        "name": "pCM1",
        "context": [
          {
            "name": "source",
            "configMap":{
              "name":"global-config",
              "namespace":"default"
            }
          }
        ],
        "match": {
        "resources": {
          "name": "config-game",
          "kinds": [
          "ConfigMap"
          ]
        }
        },
        "mutate": {
	"patchStrategicMerge": {
	  "data": "{{ source.data }}"
	}
	}
      }
      ]
    }
    }`)

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyManifest)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath_ContextSubPositive(t *testing.T) {
	policyManifest := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "policy-patch-cm"
    },
    "spec": {
      "rules": [
      {
        "name": "pCM2",
        "context": [
          {
            "name": "source",
            "configMap":{
              "name":"source-yaml-to-json",
              "namespace":"default"
            }
          }
        ],
        "match": {
        "resources": {
          "name": "config-game",
          "kinds": [
          "ConfigMap"
          ]
        }
        },
        "mutate": {
        "patchesJson6902": "- op: replace\n  path: /spec/strategy\n  value: {{ source.data.strategy }}"
        }
      }
      ]
    }
    }`)

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyManifest)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath_PositiveCase(t *testing.T) {
	policyWithVarInExclude := []byte(`{
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyWithVarInExclude)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestNotAllowedVars_JSONPatchPath_PositiveCaseWithValue(t *testing.T) {
	policyYAML := []byte(`
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

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyJSON)
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
		{"custom_func_trim_prefix", "trim_prefix(str string, prefix string)", true},
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
		policyYAML := []byte(fmt.Sprintf(`
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

		policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyYAML)
		assert.NilError(t, err)

		err = hasInvalidVariables(policy[0], false)
		if tc.pass {
			assert.NilError(t, err, "%s: not expecting an error", tc.name)
		} else {
			assert.Assert(t, err != nil, "%s: was expecting an error", tc.name)
		}
	}
}

func TestNotAllowedVars_Attestations(t *testing.T) {
	policyYAML := []byte(`
---
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: attest-bom
spec:
  validationFailureAction: enforce
  webhookTimeoutSeconds: 30
  failurePolicy: Fail  
  rules:
    - name: check-attestations
      match:
        resources:
          kinds:
            - Pod
      verifyImages:
      - imageReferences:
        - "ghcr.io/test/demo-java-tomcat:*"
        attestations:
        - predicateType: "https://cyclonedx.org/BOM/v1"
        - predicateType: "https://trivy.aquasec.com/scan/v2"
          conditions:
          - all:
            - key: "{{ scanner }}"
              operator: Equals
              value: trivy
        - predicateType: https://example.com/provenance/v1
`)

	policyJSON, err := yaml.ToJSON(policyYAML)
	assert.NilError(t, err)

	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyJSON)
	assert.NilError(t, err)

	err = hasInvalidVariables(policy[0], false)
	assert.NilError(t, err)
}

func TestAllowedVariables_AnchoredRejectsSubstringBypass(t *testing.T) {
	tcs := []struct {
		name       string
		input      string
		background bool
		pass       bool
	}{
		{"bg_request_object", "request.object.metadata.name", true, true},
		{"bg_element", "element", true, true},
		{"bg_element0", "element0", true, true},
		{"bg_element0_env", "element0.env", true, true},
		{"bg_element1_foo", "element1.foo", true, true},
		{"bg_elementIndex", "elementIndex", true, true},
		{"bg_elementIndex0", "elementIndex0", true, true},
		{"bg_elementIndex1", "elementIndex1", true, true},
		{"bg_elementIndex0_env", "elementIndex0.env", true, true},
		{"bg_at", "@", true, true},
		{"bg_images", "images.containers.nginx", true, true},
		{"bg_image_path", "image.registry", true, true},
		{"bg_length", "length(request.object)", true, true},
		{"bg_reject_invalid_request", "invalid_request.object_test", true, false},
		{"bg_reject_foorequest", "foorequest.object", true, false},
		{"bg_reject_elementss", "elementss", true, false},
		{"bg_reject_elementevil", "elementevil", true, false},
		{"bg_reject_elementIndexevil", "elementIndexevil", true, false},
		{"bg_reject_myelement", "myelement", true, false},
		{"bg_reject_myelement0", "myelement0", true, false},
		{"bg_reject_prefix_images", "prefix_images_suffix", true, false},
		{"adm_serviceAccountName", "serviceAccountName", false, true},
		{"adm_element0_env", "element0.env", false, true},
		{"adm_elementIndex1", "elementIndex1", false, true},
		{"adm_element1_foo", "element1.foo", false, true},
		{"adm_elementIndexevil", "elementIndexevil", false, false},
		{"adm_reject_elementss", "elementss", false, false},
		{"adm_reject_elementevil", "elementevil", false, false},
		{"adm_reject_myelement0", "myelement0", false, false},
		{"adm_reject_foorequest", "foorequest.object", false, false},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			policyYAML := []byte(fmt.Sprintf(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: anchored-vars
spec:
  background: %t
  rules:
  - name: check
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      any:
        - key: "{{ %s }}"
          operator: NotEquals
          value: ""
    validate:
      message: ok
      pattern:
        metadata:
          name: "*"
`, tc.background, tc.input))
			policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyYAML)
			assert.NilError(t, err)
			err = hasInvalidVariables(policy[0], tc.background)
			if tc.pass {
				assert.NilError(t, err, "%s: expected pass", tc.name)
			} else {
				assert.Assert(t, err != nil, "%s: expected reject", tc.name)
			}
		})
	}
}

func TestAllowedVariables_BackgroundRejectsUserInfo(t *testing.T) {
	policyYAML := []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: bg-userinfo
spec:
  background: true
  rules:
  - name: check
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "{{ request.userInfo.username }}"
      pattern:
        metadata:
          name: "*"
`)
	policy, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyYAML)
	assert.NilError(t, err)
	err = ValidateVariables(policy[0], true)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "request.userInfo") || strings.Contains(err.Error(), "not allowed"))
}
