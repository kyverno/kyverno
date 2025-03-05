package policy

import (
	apiservercel "k8s.io/apiserver/pkg/admission/plugin/cel"
)

var (
	NamespaceType = apiservercel.BuildNamespaceType()
	RequestType   = apiservercel.BuildRequestType()
)
