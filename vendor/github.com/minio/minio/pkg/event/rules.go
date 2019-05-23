/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
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

package event

import (
	"strings"

	"github.com/minio/minio/pkg/wildcard"
)

// NewPattern - create new pattern for prefix/suffix.
func NewPattern(prefix, suffix string) (pattern string) {
	if prefix != "" {
		if !strings.HasSuffix(prefix, "*") {
			prefix += "*"
		}

		pattern = prefix
	}

	if suffix != "" {
		if !strings.HasPrefix(suffix, "*") {
			suffix = "*" + suffix
		}

		pattern += suffix
	}

	pattern = strings.Replace(pattern, "**", "*", -1)

	return pattern
}

// Rules - event rules
type Rules map[string]TargetIDSet

// Add - adds pattern and target ID.
func (rules Rules) Add(pattern string, targetID TargetID) {
	rules[pattern] = NewTargetIDSet(targetID).Union(rules[pattern])
}

// Match - returns TargetIDSet matching object name in rules.
func (rules Rules) Match(objectName string) TargetIDSet {
	targetIDs := NewTargetIDSet()

	for pattern, targetIDSet := range rules {
		if wildcard.MatchSimple(pattern, objectName) {
			targetIDs = targetIDs.Union(targetIDSet)
		}
	}

	return targetIDs
}

// Clone - returns copy of this rules.
func (rules Rules) Clone() Rules {
	rulesCopy := make(Rules)

	for pattern, targetIDSet := range rules {
		rulesCopy[pattern] = targetIDSet.Clone()
	}

	return rulesCopy
}

// Union - returns union with given rules as new rules.
func (rules Rules) Union(rules2 Rules) Rules {
	nrules := rules.Clone()

	for pattern, targetIDSet := range rules2 {
		nrules[pattern] = nrules[pattern].Union(targetIDSet)
	}

	return nrules
}

// Difference - returns diffrence with given rules as new rules.
func (rules Rules) Difference(rules2 Rules) Rules {
	nrules := make(Rules)

	for pattern, targetIDSet := range rules {
		if nv := targetIDSet.Difference(rules2[pattern]); len(nv) > 0 {
			nrules[pattern] = nv
		}
	}

	return nrules
}
