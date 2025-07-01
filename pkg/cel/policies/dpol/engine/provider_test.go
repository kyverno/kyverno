package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewProvider(t *testing.T) {
	c := compiler.NewCompiler()

	tests := []struct {
		name       string
		compiler   compiler.Compiler
		policies   []policiesv1alpha1.DeletingPolicy
		exceptions []*policiesv1alpha1.PolicyException
		wantErr    bool
		wantCount  int
	}{
		{
			name:     "valid policy without exceptions",
			compiler: c,
			policies: []policiesv1alpha1.DeletingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "p1"},
					TypeMeta:   metav1.TypeMeta{Kind: "DeletingPolicy"},
				},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:     "valid policy with matching exception",
			compiler: c,
			policies: []policiesv1alpha1.DeletingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "p2"},
					TypeMeta:   metav1.TypeMeta{Kind: "DeletingPolicy"},
				},
			},
			exceptions: []*policiesv1alpha1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "e1"},
					Spec: policiesv1alpha1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1alpha1.PolicyRef{
							{Name: "p2", Kind: "DeletingPolicy"},
						},
					},
				},
			},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerFunc, err := NewProvider(tt.compiler, tt.policies, tt.exceptions)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got error=%v", tt.wantErr, err)
			}
			if err != nil {
				return
			}

			gotPolicies, err := providerFunc.Fetch(context.TODO())
			if err != nil {
				t.Fatalf("unexpected fetch error: %v", err)
			}
			if len(gotPolicies) != tt.wantCount {
				t.Errorf("expected %d policies, got %d", tt.wantCount, len(gotPolicies))
			}
		})
	}
}
