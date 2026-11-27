package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewProvider(t *testing.T) {
	c := compiler.NewCompiler()

	tests := []struct {
		name       string
		compiler   compiler.Compiler
		policies   []policiesv1beta1.DeletingPolicyLike
		exceptions []*policiesv1beta1.PolicyException
		wantErr    bool
		wantCount  int
	}{
		{
			name:     "valid policy without exceptions",
			compiler: c,
			policies: []policiesv1beta1.DeletingPolicyLike{
				&policiesv1beta1.DeletingPolicy{
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
			policies: []policiesv1beta1.DeletingPolicyLike{
				&policiesv1beta1.DeletingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "p2"},
					TypeMeta:   metav1.TypeMeta{Kind: "DeletingPolicy"},
				},
			},
			exceptions: []*policiesv1beta1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "e1"},
					Spec: policiesv1beta1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1beta1.PolicyRef{
							{Name: "p2", Kind: "DeletingPolicy"},
						},
					},
				},
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:     "namespaced policy",
			compiler: c,
			policies: []policiesv1beta1.DeletingPolicyLike{
				&policiesv1beta1.NamespacedDeletingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "np1"},
				},
			},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerFunc, err := NewProvider(tt.compiler, tt.policies, tt.exceptions, engine.ExceptionScopeAll)
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

// TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument
// NewProvider passes to scope.Select: under scope "*" a namespaced policy must only accept
// exceptions filed in its own namespace, never a foreign one. It fails if NewProvider ever stops
// passing policy.GetNamespace() through to the exception match.
func TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	ndpol := &policiesv1beta1.NamespacedDeletingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"}}
	ndpol.TypeMeta.Kind = "NamespacedDeletingPolicy"
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedDeletingPolicy"}},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedDeletingPolicy"}},
		},
	}

	providerFunc, err := NewProvider(
		compiler.NewCompiler(),
		[]policiesv1beta1.DeletingPolicyLike{ndpol},
		[]*policiesv1beta1.PolicyException{ownException, foreignException},
		engine.ExceptionScopeAll,
	)
	require.NoError(t, err)

	policies, err := providerFunc.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, policies, 1)

	result, err := policies[0].CompiledPolicy.Evaluate(context.Background(), unstructured.Unstructured{}, nil, &libs.FakeContextProvider{})
	require.NoError(t, err)
	require.Len(t, result.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", result.Exceptions[0].GetNamespace())
	assert.Equal(t, "own", result.Exceptions[0].GetName())
}

// TestNewProvider_ScopeGatesExceptionSelection pins the scope argument NewProvider passes to
// scope.Select: it must not be ignored or hardcoded to ExceptionScopeAll. ExceptionScopeNone must
// select nothing, and a concrete scope must select only the exception filed in that (central)
// namespace, even though the policy's own namespace would otherwise qualify.
func TestNewProvider_ScopeGatesExceptionSelection(t *testing.T) {
	ndpol := &policiesv1beta1.NamespacedDeletingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"}}
	ndpol.TypeMeta.Kind = "NamespacedDeletingPolicy"
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedDeletingPolicy"}},
		},
	}
	centralException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "central", Namespace: "central-ns"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedDeletingPolicy"}},
		},
	}

	tests := []struct {
		name          string
		scope         engine.ExceptionScope
		expectedNames []string
	}{
		{name: "scope none selects nothing", scope: engine.ExceptionScopeNone},
		{name: "concrete scope selects only the central namespace's exception", scope: engine.ExceptionScope("central-ns"), expectedNames: []string{"central"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerFunc, err := NewProvider(
				compiler.NewCompiler(),
				[]policiesv1beta1.DeletingPolicyLike{ndpol},
				[]*policiesv1beta1.PolicyException{ownException, centralException},
				tt.scope,
			)
			require.NoError(t, err)

			policies, err := providerFunc.Fetch(context.Background())
			require.NoError(t, err)
			require.Len(t, policies, 1)

			result, err := policies[0].CompiledPolicy.Evaluate(context.Background(), unstructured.Unstructured{}, nil, &libs.FakeContextProvider{})
			require.NoError(t, err)

			if len(tt.expectedNames) == 0 {
				assert.Empty(t, result.Exceptions)
				return
			}
			require.Len(t, result.Exceptions, len(tt.expectedNames))
			for i, name := range tt.expectedNames {
				assert.Equal(t, name, result.Exceptions[i].GetName())
			}
		})
	}
}
