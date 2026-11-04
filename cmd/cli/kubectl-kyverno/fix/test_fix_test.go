package fix

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFixTest_SortResources(t *testing.T) {
	t.Parallel()

	newResult := func(policy string, resources, generated []string) v1alpha1.TestResult {
		return v1alpha1.TestResult{
			TestResultBase: v1alpha1.TestResultBase{
				Policy: policy,
			},
			TestResultData: v1alpha1.TestResultData{
				Resources:          resources,
				GeneratedResources: generated,
			},
		}
	}

	tests := []struct {
		name     string
		compress bool
		results  []v1alpha1.TestResult
		want     []v1alpha1.TestResult
	}{
		{
			name: "single result",
			results: []v1alpha1.TestResult{
				newResult("policy-a",
					[]string{"zebra", "apple"},
					[]string{"z-generated", "a-generated"}),
			},
			want: []v1alpha1.TestResult{
				newResult("policy-a",
					[]string{"apple", "zebra"},
					[]string{"a-generated", "z-generated"}),
			},
		},
		{
			name: "different policies",
			results: []v1alpha1.TestResult{
				newResult("policy-b",
					[]string{"zebra", "apple"},
					[]string{"z-generated", "a-generated"}),
				newResult("policy-a",
					[]string{"orange", "banana"},
					[]string{"o-generated", "b-generated"}),
			},
			want: []v1alpha1.TestResult{
				newResult("policy-a",
					[]string{"banana", "orange"},
					[]string{"b-generated", "o-generated"}),
				newResult("policy-b",
					[]string{"apple", "zebra"},
					[]string{"a-generated", "z-generated"}),
			},
		},
		{
			name:     "compressed results",
			compress: true,
			results: []v1alpha1.TestResult{
				newResult("policy-a",
					[]string{"zebra", "banana"},
					[]string{"z-generated", "b-generated"}),
				newResult("policy-a",
					[]string{"banana", "apple"},
					[]string{"b-generated", "a-generated"}),
			},
			want: []v1alpha1.TestResult{
				newResult("policy-a",
					[]string{"apple", "banana", "zebra"},
					[]string{"a-generated", "b-generated", "z-generated"}),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := v1alpha1.Test{
				Results: tc.results,
			}
			originalResults := slices.Clone(input.Results)
			for i := range originalResults {
				originalResults[i].Resources = slices.Clone(input.Results[i].Resources)
				originalResults[i].GeneratedResources = slices.Clone(input.Results[i].GeneratedResources)
			}

			fixed, _, err := FixTest(input, tc.compress)
			if err != nil {
				t.Fatalf("FixTest() returned an error: %v", err)
			}

			if !reflect.DeepEqual(fixed.Results, tc.want) {
				t.Errorf("results mismatch:\ngot:  %#v\nwant: %#v",
					fixed.Results, tc.want)
			}

			if !reflect.DeepEqual(input.Results, originalResults) {
				t.Error("FixTest() modified the input results")
			}
		})
	}
}

func TestFixTest_GlobalContextEntries_hasData(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		wantWarning bool // whether "has no data source" warning is expected
	}{
		{
			name:        "valid JSON object — no warning",
			raw:         []byte(`{"key":"value"}`),
			wantWarning: false,
		},
		{
			name:        "null literal — warning (treated as missing)",
			raw:         []byte(`null`),
			wantWarning: true,
		},
		{
			name:        "empty bytes — warning",
			raw:         []byte{},
			wantWarning: true,
		},
		{
			name:        "whitespace only — warning (must match ValidateGlobalContextEntries)",
			raw:         []byte("   \t\n"),
			wantWarning: true,
		},
		{
			name:        "whitespace around null — warning",
			raw:         []byte("  null  "),
			wantWarning: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dataField *runtime.RawExtension
			if tc.raw != nil {
				dataField = &runtime.RawExtension{Raw: tc.raw}
			}

			test := v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				GlobalContextEntries: []v1alpha1.GlobalContextEntryValue{
					{Name: "g", Data: dataField},
				},
			}

			_, messages, err := FixTest(test, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			warnFound := false
			for _, m := range messages {
				if strings.Contains(m, "has no data source") {
					warnFound = true
					break
				}
			}

			if tc.wantWarning && !warnFound {
				t.Errorf("expected 'has no data source' warning but got none; messages: %v", messages)
			}
			if !tc.wantWarning && warnFound {
				t.Errorf("unexpected 'has no data source' warning; messages: %v", messages)
			}
		})
	}
}
