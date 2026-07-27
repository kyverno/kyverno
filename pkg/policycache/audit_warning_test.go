package policycache

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mixedAuditEnforcePolicy reproduces the policy from issue #16558: one emitWarning ClusterPolicy with
// two deny rules, an Audit rule that matches namespaces without a debug/enforce label and an Enforce
// rule that matches namespaces with it. For a namespace that matches only the Audit rule the audit
// warning was silently dropped: because the policy has an enforce rule it is stored under
// ValidateEnforce, which strips audit rules, and it is excluded from ValidateAuditWarn.
const mixedAuditEnforcePolicy = `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {"name": "debug-warn-on-missing-label"},
  "spec": {
    "background": false,
    "emitWarning": true,
    "rules": [
      {
        "name": "require-debug-label-audit",
        "match": {"any": [{"resources": {"kinds": ["ConfigMap"], "operations": ["CREATE","UPDATE"], "namespaceSelector": {"matchExpressions": [{"key": "debug/enforce", "operator": "DoesNotExist"}]}}}]},
        "validate": {
          "failureAction": "Audit",
          "message": "ConfigMap is missing the required label debug/required=true",
          "deny": {"conditions": {"any": [{"key": "{{ request.object.metadata.labels.\"debug/required\" || '' }}", "operator": "NotEquals", "value": "true"}]}}
        }
      },
      {
        "name": "require-debug-label-enforce",
        "match": {"any": [{"resources": {"kinds": ["ConfigMap"], "operations": ["CREATE","UPDATE"], "namespaceSelector": {"matchLabels": {"debug/enforce": "true"}}}}]},
        "validate": {
          "failureAction": "Enforce",
          "message": "ConfigMap is missing the required label debug/required=true",
          "deny": {"conditions": {"any": [{"key": "{{ request.object.metadata.labels.\"debug/required\" || '' }}", "operator": "NotEquals", "value": "true"}]}}
        }
      }
    ]
  }
}`

func TestGetPolicies_AuditWarn_MixedPolicyExposesAuditRule(t *testing.T) {
	var policy kyvernov1.ClusterPolicy
	assert.NilError(t, json.Unmarshal([]byte(mixedAuditEnforcePolicy), &policy))

	pc := NewCache()
	assert.NilError(t, pc.Set("debug-warn-on-missing-label", &policy, TestResourceFinder{}))

	// default namespace: no debug/enforce label, so only the audit rule is relevant.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	auditWarn := pc.GetPolicies(ValidateAuditWarn, configmapsGVR, "", ns)

	// The audit rule of the mixed policy must reach the audit-warning set so it can emit a warning.
	var auditRules []string
	for _, p := range auditWarn {
		for _, r := range p.GetSpec().Rules {
			auditRules = append(auditRules, r.Name)
		}
	}
	assert.Assert(t, contains(auditRules, "require-debug-label-audit"),
		"the audit rule must be present in the ValidateAuditWarn set, got rules: %v", auditRules)
	// only the audit rule should warn; the enforce rule stays on the enforce path.
	assert.Assert(t, !contains(auditRules, "require-debug-label-enforce"),
		"the enforce rule must not be pulled into the audit-warning set, got rules: %v", auditRules)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
