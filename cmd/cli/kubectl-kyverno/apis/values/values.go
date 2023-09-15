package values

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values/v1alpha1"
)

type (
	Values            = v1alpha1.ValuesSpec
	Subresource       = v1alpha1.Subresource
	Policy            = v1alpha1.Policy
	Rule              = v1alpha1.Rule
	Resource          = v1alpha1.Resource
	NamespaceSelector = v1alpha1.NamespaceSelector
)
