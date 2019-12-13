/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lifecycle

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"
)

var (
	errLifecycleTooManyRules      = errors.New("Lifecycle configuration allows a maximum of 1000 rules")
	errLifecycleNoRule            = errors.New("Lifecycle configuration should have at least one rule")
	errLifecycleOverlappingPrefix = errors.New("Lifecycle configuration has rules with overlapping prefix")
)

// Action represents a delete action or other transition
// actions that will be implemented later.
type Action int

const (
	// NoneAction means no action required after evaluting lifecycle rules
	NoneAction Action = iota
	// DeleteAction means the object needs to be removed after evaluting lifecycle rules
	DeleteAction
)

// Lifecycle - Configuration for bucket lifecycle.
type Lifecycle struct {
	XMLName xml.Name `xml:"LifecycleConfiguration"`
	Rules   []Rule   `xml:"Rule"`
}

// IsEmpty - returns whether policy is empty or not.
func (lc Lifecycle) IsEmpty() bool {
	return len(lc.Rules) == 0
}

// ParseLifecycleConfig - parses data in given reader to Lifecycle.
func ParseLifecycleConfig(reader io.Reader) (*Lifecycle, error) {
	var lc Lifecycle
	if err := xml.NewDecoder(reader).Decode(&lc); err != nil {
		return nil, err
	}
	if err := lc.Validate(); err != nil {
		return nil, err
	}
	return &lc, nil
}

// Validate - validates the lifecycle configuration
func (lc Lifecycle) Validate() error {
	// Lifecycle config can't have more than 1000 rules
	if len(lc.Rules) > 1000 {
		return errLifecycleTooManyRules
	}
	// Lifecycle config should have at least one rule
	if len(lc.Rules) == 0 {
		return errLifecycleNoRule
	}
	// Validate all the rules in the lifecycle config
	for _, r := range lc.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	// Compare every rule's prefix with every other rule's prefix
	for i := range lc.Rules {
		if i == len(lc.Rules)-1 {
			break
		}
		// N B Empty prefixes overlap with all prefixes
		otherRules := lc.Rules[i+1:]
		for _, otherRule := range otherRules {
			if strings.HasPrefix(lc.Rules[i].Filter.Prefix, otherRule.Filter.Prefix) ||
				strings.HasPrefix(otherRule.Filter.Prefix, lc.Rules[i].Filter.Prefix) {
				return errLifecycleOverlappingPrefix
			}
		}
	}
	return nil
}

// FilterRuleActions returns the expiration and transition from the object name
// after evaluating all rules.
func (lc Lifecycle) FilterRuleActions(objName string) (Expiration, Transition) {
	for _, rule := range lc.Rules {
		if strings.ToLower(rule.Status) != "enabled" {
			continue
		}
		if strings.HasPrefix(objName, rule.Filter.Prefix) {
			return rule.Expiration, Transition{}
		}
	}
	return Expiration{}, Transition{}
}

// ComputeAction returns the action to perform by evaluating all lifecycle rules
// against the object name and its modification time.
func (lc Lifecycle) ComputeAction(objName string, modTime time.Time) Action {
	var action = NoneAction
	exp, _ := lc.FilterRuleActions(objName)
	if !exp.IsDateNull() {
		if time.Now().After(exp.Date.Time) {
			action = DeleteAction
		}
	}
	if !exp.IsDaysNull() {
		if time.Now().After(modTime.Add(time.Duration(exp.Days) * 24 * time.Hour)) {
			action = DeleteAction
		}
	}
	return action
}
