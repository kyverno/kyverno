package fix

import (
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
