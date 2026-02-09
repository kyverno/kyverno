package compiler

import (
	apiservercel "k8s.io/apiserver/pkg/admission/plugin/cel"
)

var RequestType = apiservercel.BuildRequestType()
