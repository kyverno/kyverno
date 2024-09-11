package jmespath

import (
	"fmt"
	"strings"

	"github.com/jmespath-community/go-jmespath/pkg/functions"
)

var (
	jpObject      = functions.JpObject
	jpString      = functions.JpString
	jpNumber      = functions.JpNumber
	jpArray       = functions.JpArray
	jpArrayString = functions.JpArrayString
	jpAny         = functions.JpAny
	jpBool        = functions.JpType("bool")
)

type (
	jpType  = functions.JpType
	argSpec = functions.ArgSpec
)

type FunctionEntry struct {
	functions.FunctionEntry
	Note       string
	ReturnType []jpType
}

func (f FunctionEntry) String() string {
	if f.Name == "" {
		return ""
	}
	args := make([]string, 0, len(f.Arguments))
	for _, a := range f.Arguments {
		var aTypes []string
		for _, t := range a.Types {
			aTypes = append(aTypes, string(t))
		}
		args = append(args, strings.Join(aTypes, "|"))
	}
	returnArgs := make([]string, 0, len(f.ReturnType))
	for _, ra := range f.ReturnType {
		returnArgs = append(returnArgs, string(ra))
	}
	output := fmt.Sprintf("%s(%s) %s", f.Name, strings.Join(args, ", "), strings.Join(returnArgs, ","))
	if f.Note != "" {
		output += fmt.Sprintf(" (%s)", f.Note)
	}
	return output
}
