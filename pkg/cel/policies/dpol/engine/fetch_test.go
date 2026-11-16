package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type fakeNilPolicyLister struct{}

func (f fakeNilPolicyLister) Get(name string) (*policiesv1beta1.DeletingPolicy, error) {
	return nil, nil // <-- no error, nil policy
}

func (f fakeNilPolicyLister) List(selector labels.Selector) ([]*policiesv1beta1.DeletingPolicy, error) {
	return nil, nil
}

type fakePolexLister struct{}

func (f fakePolexLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, fmt.Errorf("forced List error for testing")
}

func (f fakePolexLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	return nil, fmt.Errorf("forced List error for testing")
}

type fakePolexNsLister struct{}

func (f fakePolexNsLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, fmt.Errorf("forced namespace List error for testing")
}

func (f fakePolexNsLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	return nil, fmt.Errorf("forced namespace List error for testing")
}

func TestGet(t *testing.T) {
	c := compiler.NewCompiler()
	ctx := context.TODO()

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: policiesv1beta1.DeletingPolicySpec{},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{},
	}

	dpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	dpolIndexer.Add(policy)

	polexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	polexIndexer.Add(exception)

	polexListers := engine.NewPolicyExceptionLister(policiesv1beta1listers.NewPolicyExceptionLister(polexIndexer), "*")
	dpolListers := policiesv1beta1listers.NewDeletingPolicyLister(dpolIndexer)

	tests := []struct {
		name         string
		polName      string
		dpol         policiesv1beta1listers.DeletingPolicyLister
		polex        engine.PolicyExceptionLister
		compiler     *compiler.Compiler
		polexEnabled bool
		wantErr      bool
	}{
		{
			name:         "list exceptions returns error",
			polName:      "test-policy",
			dpol:         dpolListers,
			polex:        fakePolexLister{},
			compiler:     &c,
			polexEnabled: true,
			wantErr:      true,
		},
		{
			name:         "unknown policy",
			polName:      "non-existent-policy",
			dpol:         dpolListers,
			polex:        polexListers,
			compiler:     &c,
			polexEnabled: false,
			wantErr:      true,
		},
		{
			name:         "successful fetch without exceptions",
			polName:      "test-policy",
			dpol:         dpolListers,
			polex:        polexListers,
			compiler:     &c,
			polexEnabled: false,
			wantErr:      false,
		},
		{
			name:         "successful fetch with exceptions",
			polName:      "test-policy",
			dpol:         dpolListers,
			polex:        polexListers,
			compiler:     &c,
			polexEnabled: true,
			wantErr:      false,
		},
		{
			name:         "compile error due to nil policy",
			polName:      "test-policy",
			dpol:         fakeNilPolicyLister{},
			polex:        nil,
			compiler:     &c,
			polexEnabled: false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewFetchProvider(*tt.compiler, tt.dpol, nil, tt.polex, tt.polexEnabled)
			_, err := provider.Get(ctx, "", tt.polName)
			if tt.wantErr {
				assert.Error(t, err, err.Error())
			} else {
				assert.NilError(t, err, "expected no error but got one")
			}
		})
	}
}

// fakeNdpolNamespaceLister serves a fixed set of NamespacedDeletingPolicies for one namespace.
type fakeNdpolNamespaceLister struct {
	policies map[string]*policiesv1beta1.NamespacedDeletingPolicy
}

func (f fakeNdpolNamespaceLister) List(labels.Selector) ([]*policiesv1beta1.NamespacedDeletingPolicy, error) {
	return nil, nil
}

func (f fakeNdpolNamespaceLister) Get(name string) (*policiesv1beta1.NamespacedDeletingPolicy, error) {
	if p, ok := f.policies[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}

type fakeNdpolLister struct {
	policies map[string]*policiesv1beta1.NamespacedDeletingPolicy // keyed by "namespace/name"
}

func (f fakeNdpolLister) List(labels.Selector) ([]*policiesv1beta1.NamespacedDeletingPolicy, error) {
	return nil, nil
}

func (f fakeNdpolLister) NamespacedDeletingPolicies(namespace string) policiesv1beta1listers.NamespacedDeletingPolicyNamespaceLister {
	byName := map[string]*policiesv1beta1.NamespacedDeletingPolicy{}
	for key, p := range f.policies {
		if ns, name, ok := splitNamespaceKey(key); ok && ns == namespace {
			byName[name] = p
		}
	}
	return fakeNdpolNamespaceLister{policies: byName}
}

func splitNamespaceKey(key string) (namespace, name string, ok bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// TestGet_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument passed to
// ListForPolicy: under scope "*" a namespaced policy must only accept exceptions filed in its own
// namespace, never a foreign one. It fails if Get ever stops passing policy.GetNamespace() through
// to the exception lookup.
func TestGet_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
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
	polexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	polexIndexer.Add(ownException)
	polexIndexer.Add(foreignException)
	polexLister := engine.NewPolicyExceptionLister(policiesv1beta1listers.NewPolicyExceptionLister(polexIndexer), "*")

	c := compiler.NewCompiler()
	provider := NewFetchProvider(
		c,
		nil,
		fakeNdpolLister{policies: map[string]*policiesv1beta1.NamespacedDeletingPolicy{"tenant-a/guard": ndpol}},
		polexLister,
		true,
	)

	fetched, err := provider.Get(context.Background(), "tenant-a", "guard")
	require.NoError(t, err)

	result, err := fetched.CompiledPolicy.Evaluate(context.Background(), unstructured.Unstructured{}, nil, &libs.FakeContextProvider{})
	require.NoError(t, err)
	require.Len(t, result.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", result.Exceptions[0].GetNamespace())
	assert.Equal(t, "own", result.Exceptions[0].GetName())
}
