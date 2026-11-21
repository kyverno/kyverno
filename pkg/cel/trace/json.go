package trace

import (
	"bytes"
	"encoding/json"
)

// APIVersion versions the JSON form of a trace: a DecisionList (kyverno apply --explain
// --output-format json) and a DecisionLogEntry (the admission log with --enableDecisionTraces).
// Both use the same field names for the same things, so a consumer that reads one can read the
// other; a log entry only leaves fields out. Values are still the strings Render prints, for
// example map[team:payments], not JSON values.
const APIVersion = "trace.kyverno.io/v1alpha1"

const (
	// KindDecisionList is a list of decisions with the summary of their results.
	KindDecisionList = "DecisionList"
	// KindDecisionLogEntry is one decision as it is written to the admission log, see LogEntry.
	KindDecisionLogEntry = "DecisionLogEntry"
)

// DecisionList is the decisions of one run, in the order they were made, and the summary of the
// results, as kyverno apply --explain --output-format json writes them.
type DecisionList struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Items      []*Decision `json:"items"`
	Summary    Summary     `json:"summary"`
}

// Summary counts the results of a run, as the text output's last line does.
type Summary struct {
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
	Skip  int `json:"skip"`
}

// NewDecisionList returns items and summary as a DecisionList of this APIVersion.
func NewDecisionList(items []*Decision, summary Summary) DecisionList {
	if items == nil {
		// an empty list, not null
		items = []*Decision{}
	}
	return DecisionList{APIVersion: APIVersion, Kind: KindDecisionList, Items: items, Summary: summary}
}

// PolicyRef names the policy a decision is about.
type PolicyRef struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

// ResourceRef names the resource a decision is about. It is nil for a JSON payload, which has no
// kind or name.
type ResourceRef struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func policyRef(d *Decision) PolicyRef {
	return PolicyRef{Name: d.PolicyName, Kind: d.PolicyKind}
}

func resourceRef(d *Decision) *ResourceRef {
	if d.ResourceKind == "" && d.ResourceName == "" && d.ResourceNamespace == "" {
		return nil
	}
	return &ResourceRef{Kind: d.ResourceKind, Name: d.ResourceName, Namespace: d.ResourceNamespace}
}

// decisionFields is Decision without its methods, so that encoding it does not call them again.
type decisionFields Decision

// decisionJSON is a Decision as it is written: its policy and resource fields as objects, then the
// rest of its fields.
type decisionJSON struct {
	Policy   *PolicyRef   `json:"policy,omitempty"`
	Resource *ResourceRef `json:"resource,omitempty"`
	*decisionFields
}

// MarshalJSON writes d with every value and error that may be Secret data replaced, the same way
// Render and LogValues show it (see redaction), so that encoding a decision cannot leak what they
// hide. Values are not cut to maxValueLen: the JSON is read by programs, not on a terminal. &, <
// and > are written as they are, not escaped for HTML, since CEL expressions are full of &&.
func (d Decision) MarshalJSON() ([]byte, error) {
	shown := decisionFields(*newRedaction(&d).decision(&d))
	policy := policyRef(&d)
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(decisionJSON{Policy: &policy, Resource: resourceRef(&d), decisionFields: &shown}); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// UnmarshalJSON reads a decision MarshalJSON wrote. What decides which values are hidden (Inputs)
// is not written, and the hidden values stay hidden.
func (d *Decision) UnmarshalJSON(data []byte) error {
	in := decisionJSON{decisionFields: (*decisionFields)(d)}
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	if in.Policy != nil {
		d.PolicyName, d.PolicyKind = in.Policy.Name, in.Policy.Kind
	}
	if in.Resource != nil {
		d.ResourceKind, d.ResourceName, d.ResourceNamespace = in.Resource.Kind, in.Resource.Name, in.Resource.Namespace
	}
	return nil
}
