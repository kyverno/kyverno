package result

import (
	"fmt"
)

// Indent acts for indenting in result hierarchy
type Indent string

const (
	// SpaceIndent means 4 spaces
	SpaceIndent Indent = "    "
	// TabIndent is a tab symbol
	TabIndent Indent = "\t"
)

// Result is an interface that is used for result polymorphic behavio
type Result interface {
	String() string
	StringWithIndent(indent string) string
}

// CompositeResult is used for result hierarchy
type CompositeResult struct {
	Message  string
	Reason   Reason
	Children []Result
}

// RuleApplicationResult represents elementary result that is produced by PolicyEngine
// TODO: It can be used to create Kubernetes Results, so make method for this
type RuleApplicationResult struct {
	PolicyRule string
	Reason     Reason
	Messages   []string
}

// StringWithIndent makes result string where each
// line is prepended with specified indent
func (e *RuleApplicationResult) StringWithIndent(indent string) string {
	message := fmt.Sprintf("%s* %s: policy rule - %s:\n", indent, e.Reason.String(), e.PolicyRule)
	childrenIndent := indent + string(SpaceIndent)
	for i, m := range e.Messages {
		message += fmt.Sprintf("%s%d. %s\n", childrenIndent, i+1, m)
	}

	// remove last line feed
	if 0 != len(message) {
		message = message[:len(message)-1]
	}
	return message
}

// String makes result string
// for writing it to logs
func (e *RuleApplicationResult) String() string {
	return e.StringWithIndent("")
}

// StringWithIndent makes result string where each
// line is prepended with specified indent
func (e *CompositeResult) StringWithIndent(indent string) string {
	childrenIndent := indent + string(SpaceIndent)
	message := fmt.Sprintf("%s- %s: %s\n", indent, e.Reason, e.Message)
	for _, res := range e.Children {
		message += (res.StringWithIndent(childrenIndent) + "\n")
	}

	// remove last line feed
	if 0 != len(message) {
		message = message[:len(message)-1]
	}

	return message
}

// String makes result string
// for writing it to logs
func (e *CompositeResult) String() string {
	return e.StringWithIndent("")
}

// Append returns CompositeResult with target and source
// Or appends source to target if it is composite result
func Append(target Result, source Result) *CompositeResult {
	if composite, ok := target.(*CompositeResult); ok {
		composite.Children = append(composite.Children, source)
		return composite
	}

	composite := &CompositeResult{
		Children: []Result{
			target,
			source,
		},
	}

	return composite
}
