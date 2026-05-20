package compiler

import (
	apiservercel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/cel"
)

var RequestType = BuildRequestType()

func BuildRequestType() *cel.DeclType {
	base := apiservercel.BuildRequestType()
	base.Fields[apiservercel.ObjectVarName] = cel.NewDeclField(apiservercel.ObjectVarName, cel.DynType, false, nil, nil)
	base.Fields[apiservercel.OldObjectVarName] = cel.NewDeclField(apiservercel.OldObjectVarName, cel.DynType, false, nil, nil)

	return base
}
