package policy

import "k8s.io/apiserver/pkg/admission/plugin/cel"

var (
	requestType = cel.BuildRequestType()
)
