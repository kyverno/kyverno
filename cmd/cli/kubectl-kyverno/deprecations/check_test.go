package deprecations

import (
	"bytes"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func TestCheckUserInfo(t *testing.T) {
	tests := []struct {
		name     string
		resource *v1alpha1.UserInfo
		want     bool
		wantMsg  bool
	}{
		{
			name:     "nil resource",
			resource: nil,
			want:     false,
			wantMsg:  false,
		},
		{
			name: "missing apiVersion and kind",
			resource: &v1alpha1.UserInfo{},
			want:    true,
			wantMsg: true,
		},
		{
			name: "missing kind only",
			resource: &v1alpha1.UserInfo{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
			},
			want:    true,
			wantMsg: true,
		},
		{
			name: "valid resource with apiVersion and kind",
			resource: &v1alpha1.UserInfo{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "UserInfo",
				},
			},
			want:    false,
			wantMsg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := CheckUserInfo(&buf, "test.yaml", tt.resource)
			if got != tt.want {
				t.Errorf("CheckUserInfo() = %v, want %v", got, tt.want)
			}
			hasMsg := buf.Len() > 0
			if hasMsg != tt.wantMsg {
				t.Errorf("CheckUserInfo() wrote message = %v, want %v", hasMsg, tt.wantMsg)
			}
		})
	}
}

func TestCheckValues(t *testing.T) {
	tests := []struct {
		name     string
		resource *v1alpha1.Values
		want     bool
		wantMsg  bool
	}{
		{
			name:     "nil resource",
			resource: nil,
			want:     false,
			wantMsg:  false,
		},
		{
			name:    "missing apiVersion and kind",
			resource: &v1alpha1.Values{},
			want:    true,
			wantMsg: true,
		},
		{
			name: "valid resource",
			resource: &v1alpha1.Values{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Values",
				},
			},
			want:    false,
			wantMsg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := CheckValues(&buf, "values.yaml", tt.resource)
			if got != tt.want {
				t.Errorf("CheckValues() = %v, want %v", got, tt.want)
			}
			hasMsg := buf.Len() > 0
			if hasMsg != tt.wantMsg {
				t.Errorf("CheckValues() wrote message = %v, want %v", hasMsg, tt.wantMsg)
			}
		})
	}
}

func TestCheckTest(t *testing.T) {
	tests := []struct {
		name     string
		resource *v1alpha1.Test
		want     bool
		wantMsg  bool
	}{
		{
			name:     "nil resource",
			resource: nil,
			want:     false,
			wantMsg:  false,
		},
		{
			name:    "missing apiVersion and kind",
			resource: &v1alpha1.Test{},
			want:    true,
			wantMsg: true,
		},
		{
			name: "deprecated Name field set",
			resource: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
				Name: "legacy-name",
			},
			want:    true,
			wantMsg: true,
		},
		{
			name: "valid resource",
			resource: &v1alpha1.Test{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cli.kyverno.io/v1alpha1",
					Kind:       "Test",
				},
			},
			want:    false,
			wantMsg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := CheckTest(&buf, "kyverno-test.yaml", tt.resource)
			if got != tt.want {
				t.Errorf("CheckTest() = %v, want %v", got, tt.want)
			}
			hasMsg := buf.Len() > 0
			if hasMsg != tt.wantMsg {
				t.Errorf("CheckTest() wrote message = %v, want %v", hasMsg, tt.wantMsg)
			}
		})
	}
}
