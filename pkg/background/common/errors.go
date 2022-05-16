package common

import "fmt"

// ParseFailedError stores the resource that failed to parse
type ParseFailedError struct {
	spec       interface{}
	parseError error
}

func (e *ParseFailedError) Error() string {
	return fmt.Sprintf("failed to parse the resource spec %v: %v", e.spec, e.parseError.Error())
}

// NewParseFailed returns a new ParseFailed error
func NewParseFailed(spec interface{}, err error) *ParseFailedError {
	return &ParseFailedError{spec: spec, parseError: err}
}

// ViolationError stores the rule that violated
type ViolationError struct {
	rule string
	err  error
}

func (e *ViolationError) Error() string {
	return fmt.Sprintf("creating Violation; error %s", e.err)
}

// NewViolation returns a new Violation error
func NewViolation(rule string, err error) *ViolationError {
	return &ViolationError{rule: rule, err: err}
}

// NotFoundError stores the resource that was not found
type NotFoundError struct {
	kind      string
	namespace string
	name      string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource %s/%s/%s not present", e.kind, e.namespace, e.name)
}

// NewNotFound returns a new NotFound error
func NewNotFound(kind, namespace, name string) *NotFoundError {
	return &NotFoundError{kind: kind, namespace: namespace, name: name}
}

// ConfigNotFoundError stores the config information
type ConfigNotFoundError struct {
	config    interface{}
	kind      string
	namespace string
	name      string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("configuration %v, not present in resource %s/%s/%s", e.config, e.kind, e.namespace, e.name)
}

//NewConfigNotFound returns a new NewConfigNotFound error
func NewConfigNotFound(config interface{}, kind, namespace, name string) *ConfigNotFoundError {
	return &ConfigNotFoundError{config: config, kind: kind, namespace: namespace, name: name}
}
