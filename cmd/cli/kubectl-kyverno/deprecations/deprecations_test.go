package deprecations

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckUserInfo(t *testing.T) {
	tests := []struct {
		name        string
		resource    *v1alpha1.UserInfo
		wantResult  bool
		wantWarning bool
	}{
		{
			name:        "nil resource",
			resource:    nil,
			wantResult:  false,
			wantWarning: false,
		},
		{
			name:        "missing apiVersion",
			resource:    &v1alpha1.UserInfo{TypeMeta: metav1.TypeMeta{Kind: "UserInfo"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "missing kind",
			resource:    &v1alpha1.UserInfo{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "valid resource",
			resource:    &v1alpha1.UserInfo{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1", Kind: "UserInfo"}},
			wantResult:  false,
			wantWarning: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got := CheckUserInfo(&out, "user-infos.yaml", tt.resource)
			if got != tt.wantResult {
				t.Errorf("CheckUserInfo() = %v, want %v", got, tt.wantResult)
			}
			if gotWarning := strings.Contains(out.String(), "WARNING"); gotWarning != tt.wantWarning {
				t.Errorf("warning emitted = %v, want %v (output: %q)", gotWarning, tt.wantWarning, out.String())
			}
		})
	}
}

func TestCheckValues(t *testing.T) {
	tests := []struct {
		name        string
		resource    *v1alpha1.Values
		wantResult  bool
		wantWarning bool
	}{
		{
			name:        "nil resource",
			resource:    nil,
			wantResult:  false,
			wantWarning: false,
		},
		{
			name:        "missing apiVersion",
			resource:    &v1alpha1.Values{TypeMeta: metav1.TypeMeta{Kind: "Values"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "missing kind",
			resource:    &v1alpha1.Values{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "valid resource",
			resource:    &v1alpha1.Values{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1", Kind: "Values"}},
			wantResult:  false,
			wantWarning: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got := CheckValues(&out, "values.yaml", tt.resource)
			if got != tt.wantResult {
				t.Errorf("CheckValues() = %v, want %v", got, tt.wantResult)
			}
			if gotWarning := strings.Contains(out.String(), "WARNING"); gotWarning != tt.wantWarning {
				t.Errorf("warning emitted = %v, want %v (output: %q)", gotWarning, tt.wantWarning, out.String())
			}
		})
	}
}

func TestCheckTest(t *testing.T) {
	tests := []struct {
		name        string
		resource    *v1alpha1.Test
		wantResult  bool
		wantWarning bool
	}{
		{
			name:        "nil resource",
			resource:    nil,
			wantResult:  false,
			wantWarning: false,
		},
		{
			name:        "missing apiVersion",
			resource:    &v1alpha1.Test{TypeMeta: metav1.TypeMeta{Kind: "Test"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "missing kind",
			resource:    &v1alpha1.Test{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1"}},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "deprecated name field set",
			resource:    &v1alpha1.Test{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1", Kind: "Test"}, Name: "my-test"},
			wantResult:  true,
			wantWarning: true,
		},
		{
			name:        "valid resource",
			resource:    &v1alpha1.Test{TypeMeta: metav1.TypeMeta{APIVersion: "cli.kyverno.io/v1alpha1", Kind: "Test"}},
			wantResult:  false,
			wantWarning: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got := CheckTest(&out, "test.yaml", tt.resource)
			if got != tt.wantResult {
				t.Errorf("CheckTest() = %v, want %v", got, tt.wantResult)
			}
			if gotWarning := strings.Contains(out.String(), "WARNING"); gotWarning != tt.wantWarning {
				t.Errorf("warning emitted = %v, want %v (output: %q)", gotWarning, tt.wantWarning, out.String())
			}
		})
	}
}

func TestCheckNilWriterDoesNotPanic(t *testing.T) {
	// The Check* functions accept a nil writer and must not panic when the
	// resource uses a deprecated schema.
	if !CheckUserInfo(nil, "user-infos.yaml", &v1alpha1.UserInfo{TypeMeta: metav1.TypeMeta{Kind: "UserInfo"}}) {
		t.Error("CheckUserInfo() with nil writer = false, want true")
	}
	if !CheckValues(nil, "values.yaml", &v1alpha1.Values{TypeMeta: metav1.TypeMeta{Kind: "Values"}}) {
		t.Error("CheckValues() with nil writer = false, want true")
	}
	if !CheckTest(nil, "test.yaml", &v1alpha1.Test{TypeMeta: metav1.TypeMeta{Kind: "Test"}}) {
		t.Error("CheckTest() with nil writer = false, want true")
	}
}
