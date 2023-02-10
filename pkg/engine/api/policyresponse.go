package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ValidationFailureActionOverride struct {
	Action            kyvernov1.ValidationFailureAction
	Namespaces        []string
	NamespaceSelector *metav1.LabelSelector
}

// PolicyResponse policy application response
type PolicyResponse struct {
	// Stats contains policy statistics
	Stats PolicyStats
	// Rules contains policy rules responses
	Rules []RuleResponse
	// ValidationFailureAction audit (default) or enforce
	ValidationFailureAction kyvernov1.ValidationFailureAction
	// ValidationFailureActionOverrides overrides
	ValidationFailureActionOverrides []ValidationFailureActionOverride
}
