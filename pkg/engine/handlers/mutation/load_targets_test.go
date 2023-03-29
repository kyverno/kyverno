package mutation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_match(t *testing.T) {
	tests := []struct {
		testName         string
		namespacePattern string
		namePattern      string
		namespace        string
		name             string
		expectedResult   bool
	}{
		{
			testName:         "empty-namespacePattern-namePattern-1",
			namespacePattern: "",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-namePattern-2",
			namespacePattern: "",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-1",
			namespacePattern: "",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-2",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-3",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "empty-namespacePattern-4",
			namespacePattern: "",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-5",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namespacePattern-6",
			namespacePattern: "",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-1",
			namespacePattern: "foo",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-2",
			namespacePattern: "foo",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namePattern-3",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "empty-namePattern-4",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "empty-namePattern-5",
			namespacePattern: "fo*",
			namePattern:      "",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-1",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "",
			name:             "",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-2",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-3",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-4",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-5",
			namespacePattern: "foo",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-6",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-7",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "random",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-8",
			namespacePattern: "fo*",
			namePattern:      "bar",
			namespace:        "",
			name:             "bar",
			expectedResult:   false,
		},
		{
			testName:         "no-empty-pattern-9",
			namespacePattern: "foo",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "bar",
			expectedResult:   true,
		},
		{
			testName:         "no-empty-pattern-10",
			namespacePattern: "foo",
			namePattern:      "ba*",
			namespace:        "foo",
			name:             "random",
			expectedResult:   false,
		},
		// {
		// 	testName:         "",
		// 	namespacePattern: "",
		// 	namePattern:      "",
		// 	namespace:        "",
		// 	name:             "",
		// 	expectedResult:   false,
		// },
	}

	for _, test := range tests {
		res := match(test.namespacePattern, test.namePattern, test.namespace, test.name)
		assert.Equal(t, test.expectedResult, res, fmt.Sprintf("test %s failed", test.testName))
	}
}
