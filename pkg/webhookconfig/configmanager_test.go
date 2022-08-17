package webhookconfig

import (
	"testing"

	"gotest.tools/assert"
)

var (
	emptyInternalRules []interface{}
	emptyAPIRules      []interface{}

	configmapsInternalRules []interface{}
	configmapsAPIRules      []interface{}

	configmapsSecretsInternalRules []interface{}
	configmapsSecretsAPIRules      []interface{}

	secretsConfigmapsInternalRules []interface{}
	secretsConfigmapsAPIRules      []interface{}

	badAPIRules []interface{}
)

func init() {
	// No rules.
	// Internal representation is a rule with no selectors.
	// API server representation is no rule (nil).
	emptyInternalRules = []interface{}{map[string]interface{}{}}
	emptyAPIRules = nil

	// Rule selecting configmaps.
	// API server representation matches the internal
	// representation but has extra fields "operations" and "scope",
	// and is of interface types instead of strings.
	configmapsInternalRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []string{""},
			"apiVersions": []string{"v1"},
			"resources":   []string{"configmaps"},
		},
	}
	configmapsAPIRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []interface{}{""},
			"apiVersions": []interface{}{"v1"},
			"resources":   []interface{}{"configmaps"},
			"operations":  []interface{}{"CREATE", "UPDATE", "DELETE", "CONNECT"},
			"scope":       "*",
		},
	}

	// Rule selecting configmaps and secrets.
	// API server representation matches the internal
	// representation but has extra fields "operations" and "scope",
	// and is of interface types instead of strings.
	configmapsSecretsInternalRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []string{""},
			"apiVersions": []string{"v1"},
			"resources":   []string{"configmaps", "secrets"},
		},
	}
	configmapsSecretsAPIRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []interface{}{""},
			"apiVersions": []interface{}{"v1"},
			"resources":   []interface{}{"configmaps", "secrets"},
			"operations":  []interface{}{"CREATE", "UPDATE", "DELETE", "CONNECT"},
			"scope":       "*",
		},
	}

	// Same as previous but reversing the order of configmaps and secrets.
	secretsConfigmapsInternalRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []string{""},
			"apiVersions": []string{"v1"},
			"resources":   []string{"secrets", "configmaps"},
		},
	}
	secretsConfigmapsAPIRules = []interface{}{
		map[string]interface{}{
			"apiGroups":   []interface{}{""},
			"apiVersions": []interface{}{"v1"},
			"resources":   []interface{}{"secrets", "configmaps"},
			"operations":  []interface{}{"CREATE", "UPDATE", "DELETE", "CONNECT"},
			"scope":       "*",
		},
	}

	// API rules with missing fields.
	badAPIRules = []interface{}{
		map[string]interface{}{
			"apiGroups": []interface{}{""},
		},
	}
}

func TestRulesEqual(t *testing.T) {
	tests := []struct {
		name      string
		internal  []interface{}
		apiserver []interface{}
		equal     bool
		shouldErr bool
	}{
		// Both empty. Should be equal.
		{"empty-equal", emptyInternalRules, emptyAPIRules, true, false},

		// Both rules select configmaps. Should be equal.
		{"configmaps-equal", configmapsInternalRules, configmapsAPIRules, true, false},

		// Both rules select configmaps and secrets. Should be equal.
		{"cm-secrets-equal", configmapsSecretsInternalRules, configmapsSecretsAPIRules, true, false},

		// Both rules select secrets and configmaps (reversed compared to previous). Should be equal.
		{"secrets-cm-equal", secretsConfigmapsInternalRules, secretsConfigmapsAPIRules, true, false},

		// Internal empty, API has one rule. Not equal.
		{"internal-empty-api-single", emptyInternalRules, configmapsSecretsAPIRules, false, false},

		// Internal is updated from nothing to configmaps. Not equal.
		{"add-configmaps", configmapsInternalRules, emptyAPIRules, false, false},

		// Internal is updated from configmaps to configmaps and secrets. Not equal.
		{"add-secrets", configmapsSecretsInternalRules, configmapsAPIRules, false, false},

		// Order of configmaps and secrets is switched. Not equal.
		{"order-switched", configmapsSecretsInternalRules, secretsConfigmapsAPIRules, false, false},

		// Malformed API rules, if modified by user or something like that. Not equal.
		{"bad-api-rules", configmapsInternalRules, badAPIRules, false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			equal, err := webhookRulesEqual(test.apiserver, test.internal)
			assert.Equal(t, err != nil, test.shouldErr)
			assert.Equal(t, equal, test.equal)
		})
	}
}
