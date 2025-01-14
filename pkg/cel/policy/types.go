package policy

import "k8s.io/apiserver/pkg/admission/plugin/cel"

var (
	namespaceType = cel.BuildNamespaceType()
	requestType   = cel.BuildRequestType()
)
