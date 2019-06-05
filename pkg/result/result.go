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

// Result is an interface that is used for result polymorphic behavior
type Result interface {
	String() string
	StringWithIndent(indent string) string
	GetReason() Reason
	ToError() error
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

func NewRuleApplicationResult(ruleName string) RuleApplicationResult {
	return RuleApplicationResult{
		PolicyRule: ruleName,
		Reason:     Success,
		Messages:   []string{},
	}
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

func (e *RuleApplicationResult) ToError() error {
	if e.Reason != Success {
		return fmt.Errorf(e.String())
	}
	return nil
}

func (e *RuleApplicationResult) GetReason() Reason {
	return e.Reason
}

// Adds formatted message to this result
func (rar *RuleApplicationResult) AddMessagef(message string, a ...interface{}) {
	rar.Messages = append(rar.Messages, fmt.Sprintf(message, a...))
}

// Sets the Reason Failed and adds formatted message to this result
func (rar *RuleApplicationResult) FailWithMessagef(message string, a ...interface{}) {
	rar.Reason = Failed
	rar.AddMessagef(message, a...)
}

// Takes messages and higher reason from another RuleApplicationResult
func (e *RuleApplicationResult) MergeWith(other *RuleApplicationResult) {
	if other != nil {
		e.Messages = append(e.Messages, other.Messages...)
	}
	if other.Reason > e.Reason {
		e.Reason = other.Reason
	}
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

func (e *CompositeResult) ToError() error {
	if e.Reason != Success {
		return fmt.Errorf(e.String())
	}
	return nil
}

func (e *CompositeResult) GetReason() Reason {
	return e.Reason
}

func NewPolicyApplicationResult(policyName string) Result {
	return &CompositeResult{
		Message: fmt.Sprintf("policy - %s:", policyName),
		Reason:  Success,
	}
}

func NewAdmissionResult(requestUID string) Result {
	return &CompositeResult{
		Message: fmt.Sprintf("For resource with UID - %s:", requestUID),
		Reason:  Success,
	}
}

// Append returns CompositeResult with target and source
// Or appends source to target if it is composite result
// If the source reason is more important than target reason,
// target takes the reason of the source.
func Append(target Result, source Result) Result {
	targetReason := target.GetReason()
	if targetReason < source.GetReason() {
		targetReason = source.GetReason()
	}

	if composite, ok := target.(*CompositeResult); ok {
		composite.Children = append(composite.Children, source)
		composite.Reason = targetReason
		return composite
	}

	composite := &CompositeResult{
		Children: []Result{
			target,
			source,
		},
		Reason: targetReason,
	}

	return composite
}
