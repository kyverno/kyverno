package generate

import "fmt"

// ParseFailed stores the resource that failed to parse
type ParseFailed struct {
	spec       interface{}
	parseError error
}

func (e *ParseFailed) Error() string {
	return fmt.Sprintf("failed to parse the resource spec %v: %v", e.spec, e.parseError.Error())
}

//NewParseFailed returns a new ParseFailed error
func NewParseFailed(spec interface{}, err error) *ParseFailed {
	return &ParseFailed{spec: spec, parseError: err}
}

//Violation stores the rule that violated
type Violation struct {
	rule string
	err  error
}

func (e *Violation) Error() string {
	return fmt.Sprintf("creating Violation; error %s", e.err)
}

//NewViolation returns a new Violation error
func NewViolation(rule string, err error) *Violation {
	return &Violation{rule: rule, err: err}
}

// NotFound stores the resource that was not found
type NotFound struct {
	kind      string
	namespace string
	name      string
}

func (e *NotFound) Error() string {
	return fmt.Sprintf("resource %s/%s/%s not present", e.kind, e.namespace, e.name)
}

//NewNotFound returns a new NotFound error
func NewNotFound(kind, namespace, name string) *NotFound {
	return &NotFound{kind: kind, namespace: namespace, name: name}
}

//ConfigNotFound stores the config information
type ConfigNotFound struct {
	config    interface{}
	kind      string
	namespace string
	name      string
}

func (e *ConfigNotFound) Error() string {
	return fmt.Sprintf("configuration %v, not present in resource %s/%s/%s", e.config, e.kind, e.namespace, e.name)
}

//NewConfigNotFound returns a new NewConfigNotFound error
func NewConfigNotFound(config interface{}, kind, namespace, name string) *ConfigNotFound {
	return &ConfigNotFound{config: config, kind: kind, namespace: namespace, name: name}
}
