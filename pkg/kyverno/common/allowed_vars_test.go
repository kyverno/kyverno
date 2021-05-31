package common

import (
	"testing"

	ut "github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
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
