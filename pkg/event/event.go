package event

import (
	"fmt"
)

// Indent acts for indenting in event hierarchy
type Indent string

const (
	// SpaceIndent means 4 spaces
	SpaceIndent Indent = "    "
	// TabIndent is a tab symbol
	TabIndent Indent = "\t"
)

// KyvernoEvent is an interface that is used for event polymorphic behavio
type KyvernoEvent interface {
	String() string
	StringWithIndent(indent string) string
}

// CompositeEvent is used for event hierarchy
type CompositeEvent struct {
	Message  string
	Reason   Reason
	Children []KyvernoEvent
}

// RuleEvent represents elementary event that is produced by PolicyEngine
// TODO: It can be used to create Kubernetes Events, so make method for this
type RuleEvent struct {
	PolicyRule string
	Reason     Reason
	Messages   []string
}

// StringWithIndent makes event string where each
// line is prepended with specified indent
func (e *RuleEvent) StringWithIndent(indent string) string {
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

// String makes event string
// for writing it to logs
func (e *RuleEvent) String() string {
	return e.StringWithIndent("")
}

// StringWithIndent makes event string where each
// line is prepended with specified indent
func (e *CompositeEvent) StringWithIndent(indent string) string {
	childrenIndent := indent + string(SpaceIndent)
	message := fmt.Sprintf("%s-%s: %s", indent, e.Reason, e.Message)
	for _, event := range e.Children {
		message += (event.StringWithIndent(childrenIndent) + "\n")
	}

	// remove last line feed
	if 0 != len(message) {
		message = message[:len(message)-1]
	}

	return message
}

// String makes event string
// for writing it to logs
func (e *CompositeEvent) String() string {
	return e.StringWithIndent("")
}

// Append returns CompositeEvent with target and source
// Or appends source to target if it is composite event
func Append(target KyvernoEvent, source KyvernoEvent) *CompositeEvent {
	if composite, ok := target.(*CompositeEvent); ok {
		composite.Children = append(composite.Children, source)
		return composite
	}

	composite := &CompositeEvent{
		Children: []KyvernoEvent{
			target,
			source,
		},
	}

	return composite
}
