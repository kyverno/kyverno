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
	"reflect"
	"testing"
)

func TestRulesMapClone(t *testing.T) {
	rulesMapCase1 := make(RulesMap)
	rulesMapToAddCase1 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	rulesMapCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	rulesMapToAddCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})

	rulesMapCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	rulesMapToAddCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	testCases := []struct {
		rulesMap      RulesMap
		rulesMapToAdd RulesMap
	}{
		{rulesMapCase1, rulesMapToAddCase1},
		{rulesMapCase2, rulesMapToAddCase2},
		{rulesMapCase3, rulesMapToAddCase3},
	}

	for i, testCase := range testCases {
		result := testCase.rulesMap.Clone()

		if !reflect.DeepEqual(result, testCase.rulesMap) {
			t.Fatalf("test %v: result: expected: %v, got: %v", i+1, testCase.rulesMap, result)
		}

		result.Add(testCase.rulesMapToAdd)
		if reflect.DeepEqual(result, testCase.rulesMap) {
			t.Fatalf("test %v: result: expected: not equal, got: equal", i+1)
		}
	}
}

func TestRulesMapAdd(t *testing.T) {
	rulesMapCase1 := make(RulesMap)
	rulesMapToAddCase1 := make(RulesMap)
	expectedResultCase1 := make(RulesMap)

	rulesMapCase2 := make(RulesMap)
	rulesMapToAddCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	expectedResultCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	rulesMapCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	rulesMapToAddCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	expectedResultCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	expectedResultCase3.add([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	testCases := []struct {
		rulesMap       RulesMap
		rulesMapToAdd  RulesMap
		expectedResult RulesMap
	}{
		{rulesMapCase1, rulesMapToAddCase1, expectedResultCase1},
		{rulesMapCase2, rulesMapToAddCase2, expectedResultCase2},
		{rulesMapCase3, rulesMapToAddCase3, expectedResultCase3},
	}

	for i, testCase := range testCases {
		testCase.rulesMap.Add(testCase.rulesMapToAdd)

		if !reflect.DeepEqual(testCase.rulesMap, testCase.expectedResult) {
			t.Fatalf("test %v: result: expected: %v, got: %v", i+1, testCase.expectedResult, testCase.rulesMap)
		}
	}
}

func TestRulesMapRemove(t *testing.T) {
	rulesMapCase1 := make(RulesMap)
	rulesMapToAddCase1 := make(RulesMap)
	expectedResultCase1 := make(RulesMap)

	rulesMapCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	rulesMapToAddCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	expectedResultCase2 := make(RulesMap)

	rulesMapCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	rulesMapCase3.add([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})
	rulesMapToAddCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	expectedResultCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	testCases := []struct {
		rulesMap       RulesMap
		rulesMapToAdd  RulesMap
		expectedResult RulesMap
	}{
		{rulesMapCase1, rulesMapToAddCase1, expectedResultCase1},
		{rulesMapCase2, rulesMapToAddCase2, expectedResultCase2},
		{rulesMapCase3, rulesMapToAddCase3, expectedResultCase3},
	}

	for i, testCase := range testCases {
		testCase.rulesMap.Remove(testCase.rulesMapToAdd)

		if !reflect.DeepEqual(testCase.rulesMap, testCase.expectedResult) {
			t.Fatalf("test %v: result: expected: %v, got: %v", i+1, testCase.expectedResult, testCase.rulesMap)
		}
	}
}

func TestRulesMapMatch(t *testing.T) {
	rulesMapCase1 := make(RulesMap)

	rulesMapCase2 := NewRulesMap([]Name{ObjectCreatedAll}, "*", TargetID{"1", "webhook"})

	rulesMapCase3 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})

	rulesMapCase4 := NewRulesMap([]Name{ObjectCreatedAll}, "2010*.jpg", TargetID{"1", "webhook"})
	rulesMapCase4.add([]Name{ObjectCreatedAll}, "*", TargetID{"2", "amqp"})

	testCases := []struct {
		rulesMap       RulesMap
		eventName      Name
		objectName     string
		expectedResult TargetIDSet
	}{
		{rulesMapCase1, ObjectCreatedPut, "2010/photo.jpg", NewTargetIDSet()},
		{rulesMapCase2, ObjectCreatedPut, "2010/photo.jpg", NewTargetIDSet(TargetID{"1", "webhook"})},
		{rulesMapCase3, ObjectCreatedPut, "2000/photo.png", NewTargetIDSet()},
		{rulesMapCase4, ObjectCreatedPut, "2000/photo.png", NewTargetIDSet(TargetID{"2", "amqp"})},
	}

	for i, testCase := range testCases {
		result := testCase.rulesMap.Match(testCase.eventName, testCase.objectName)

		if !reflect.DeepEqual(result, testCase.expectedResult) {
			t.Fatalf("test %v: result: expected: %v, got: %v", i+1, testCase.expectedResult, result)
		}
	}
}

func TestNewRulesMap(t *testing.T) {
	rulesMapCase1 := make(RulesMap)
	rulesMapCase1.add([]Name{ObjectAccessedGet, ObjectAccessedHead}, "*", TargetID{"1", "webhook"})

	rulesMapCase2 := make(RulesMap)
	rulesMapCase2.add([]Name{ObjectAccessedGet, ObjectAccessedHead, ObjectCreatedPut}, "*", TargetID{"1", "webhook"})

	rulesMapCase3 := make(RulesMap)
	rulesMapCase3.add([]Name{ObjectRemovedDelete}, "2010*.jpg", TargetID{"1", "webhook"})

	testCases := []struct {
		eventNames     []Name
		pattern        string
		targetID       TargetID
		expectedResult RulesMap
	}{
		{[]Name{ObjectAccessedAll}, "", TargetID{"1", "webhook"}, rulesMapCase1},
		{[]Name{ObjectAccessedAll, ObjectCreatedPut}, "", TargetID{"1", "webhook"}, rulesMapCase2},
		{[]Name{ObjectRemovedDelete}, "2010*.jpg", TargetID{"1", "webhook"}, rulesMapCase3},
	}

	for i, testCase := range testCases {
		result := NewRulesMap(testCase.eventNames, testCase.pattern, testCase.targetID)

		if !reflect.DeepEqual(result, testCase.expectedResult) {
			t.Fatalf("test %v: result: expected: %v, got: %v", i+1, testCase.expectedResult, result)
		}
	}
}
