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

package iampolicy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/minio/minio/pkg/policy"
	"github.com/minio/minio/pkg/policy/condition"
)

// Statement - iam policy statement.
type Statement struct {
	SID        policy.ID           `json:"Sid,omitempty"`
	Effect     policy.Effect       `json:"Effect"`
	Actions    ActionSet           `json:"Action"`
	Resources  ResourceSet         `json:"Resource"`
	Conditions condition.Functions `json:"Condition,omitempty"`
}

// IsAllowed - checks given policy args is allowed to continue the Rest API.
func (statement Statement) IsAllowed(args Args) bool {
	check := func() bool {
		if !statement.Actions.Match(args.Action) {
			return false
		}

		resource := args.BucketName
		if args.ObjectName != "" {
			if !strings.HasPrefix(args.ObjectName, "/") {
				resource += "/"
			}

			resource += args.ObjectName
		} else {
			resource += "/"
		}

		if !statement.Resources.Match(resource, args.ConditionValues) {
			return false
		}

		return statement.Conditions.Evaluate(args.ConditionValues)
	}

	return statement.Effect.IsAllowed(check())
}

// isValid - checks whether statement is valid or not.
func (statement Statement) isValid() error {
	if !statement.Effect.IsValid() {
		return fmt.Errorf("invalid Effect %v", statement.Effect)
	}

	if len(statement.Actions) == 0 {
		return fmt.Errorf("Action must not be empty")
	}

	if len(statement.Resources) == 0 {
		return fmt.Errorf("Resource must not be empty")
	}

	if err := statement.Resources.Validate(); err != nil {
		return err
	}

	for action := range statement.Actions {
		if !statement.Resources.objectResourceExists() && !statement.Resources.bucketResourceExists() {
			return fmt.Errorf("unsupported Resource found %v for action %v", statement.Resources, action)
		}

		keys := statement.Conditions.Keys()
		keyDiff := keys.Difference(actionConditionKeyMap[action])
		if !keyDiff.IsEmpty() {
			return fmt.Errorf("unsupported condition keys '%v' used for action '%v'", keyDiff, action)
		}
	}

	return nil
}

// MarshalJSON - encodes JSON data to Statement.
func (statement Statement) MarshalJSON() ([]byte, error) {
	if err := statement.isValid(); err != nil {
		return nil, err
	}

	// subtype to avoid recursive call to MarshalJSON()
	type subStatement Statement
	ss := subStatement(statement)
	return json.Marshal(ss)
}

// UnmarshalJSON - decodes JSON data to Statement.
func (statement *Statement) UnmarshalJSON(data []byte) error {
	// subtype to avoid recursive call to UnmarshalJSON()
	type subStatement Statement
	var ss subStatement

	if err := json.Unmarshal(data, &ss); err != nil {
		return err
	}

	s := Statement(ss)
	if err := s.isValid(); err != nil {
		return err
	}

	*statement = s

	return nil
}

// Validate - validates Statement is for given bucket or not.
func (statement Statement) Validate() error {
	return statement.isValid()
}

// NewStatement - creates new statement.
func NewStatement(effect policy.Effect, actionSet ActionSet, resourceSet ResourceSet, conditions condition.Functions) Statement {
	return Statement{
		Effect:     effect,
		Actions:    actionSet,
		Resources:  resourceSet,
		Conditions: conditions,
	}
}
