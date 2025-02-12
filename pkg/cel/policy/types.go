package policy

import (
	apiservercel "k8s.io/apiserver/pkg/admission/plugin/cel"
)

var (
	namespaceType = apiservercel.BuildNamespaceType()
	requestType   = apiservercel.BuildRequestType()
)
