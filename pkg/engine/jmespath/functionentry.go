package jmespath

import (
	"fmt"
	"strings"

	gojmespath "github.com/kyverno/go-jmespath"
)

var (
	jpObject      = gojmespath.JpObject
	jpString      = gojmespath.JpString
	jpNumber      = gojmespath.JpNumber
	jpArray       = gojmespath.JpArray
	jpArrayString = gojmespath.JpArrayString
	jpAny         = gojmespath.JpAny
	jpBool        = gojmespath.JpType("bool")
)

type (
	jpType  = gojmespath.JpType
	argSpec = gojmespath.ArgSpec
)

type FunctionEntry struct {
	gojmespath.FunctionEntry
	Note       string
	ReturnType []jpType
}

func (f FunctionEntry) String() string {
	if f.Name == "" {
		return ""
	}
	var args []string
	for _, a := range f.Arguments {
		var aTypes []string
		for _, t := range a.Types {
			aTypes = append(aTypes, string(t))
		}
		args = append(args, strings.Join(aTypes, "|"))
	}
	var returnArgs []string
	for _, ra := range f.ReturnType {
		returnArgs = append(returnArgs, string(ra))
	}
	output := fmt.Sprintf("%s(%s) %s", f.Name, strings.Join(args, ", "), strings.Join(returnArgs, ","))
	if f.Note != "" {
		output += fmt.Sprintf(" (%s)", f.Note)
	}
	return output
}
