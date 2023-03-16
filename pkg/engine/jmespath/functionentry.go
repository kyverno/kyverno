package jmespath

import (
	"fmt"
	"strings"

	"github.com/jmespath-community/go-jmespath"
)

var (
	jpObject      = jmespath.JpObject
	jpString      = jmespath.JpString
	jpNumber      = jmespath.JpNumber
	jpArray       = jmespath.JpArray
	jpArrayString = jmespath.JpArrayString
	jpAny         = jmespath.JpAny
	jpBool        = jmespath.JpType("bool")
)

type (
	jpType  = jmespath.JpType
	argSpec = jmespath.ArgSpec
)

type FunctionEntry struct {
	jmespath.FunctionEntry
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
