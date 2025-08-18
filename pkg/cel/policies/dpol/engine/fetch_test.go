package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type fakeNilPolicyLister struct{}

func (f fakeNilPolicyLister) Get(name string) (*policiesv1alpha1.DeletingPolicy, error) {
	return nil, nil // <-- no error, nil policy
}

func (f fakeNilPolicyLister) List(selector labels.Selector) ([]*policiesv1alpha1.DeletingPolicy, error) {
	return nil, nil
}

type fakePolexLister struct{}

func (f fakePolexLister) List(selector labels.Selector) ([]*policiesv1alpha1.PolicyException, error) {
	return nil, fmt.Errorf("forced List error for testing")
}

func (f fakePolexLister) PolicyExceptions(namespace string) policiesv1alpha1listers.PolicyExceptionNamespaceLister {
	return fakePolexNsLister{}
}

type fakePolexNsLister struct{}

func (f fakePolexNsLister) List(selector labels.Selector) ([]*policiesv1alpha1.PolicyException, error) {
	return nil, fmt.Errorf("forced namespace List error for testing")
}

func (f fakePolexNsLister) Get(name string) (*policiesv1alpha1.PolicyException, error) {
	return nil, fmt.Errorf("forced Get error for testing")
}

func TestGet(t *testing.T) {
	c := compiler.NewCompiler()
	ctx := context.TODO()

	policy := &policiesv1alpha1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: policiesv1alpha1.DeletingPolicySpec{},
	}

	exception := &policiesv1alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: policiesv1alpha1.PolicyExceptionSpec{},
	}

	dpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	dpolIndexer.Add(policy)

	polexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	polexIndexer.Add(exception)

	polexListers := policiesv1alpha1listers.NewPolicyExceptionLister(polexIndexer)
	dpolListers := policiesv1alpha1listers.NewDeletingPolicyLister(dpolIndexer)

	tests := []struct {
		name         string
		polName      string
		dpol         policiesv1alpha1listers.DeletingPolicyLister
		polex        policiesv1alpha1listers.PolicyExceptionLister
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
			provider := NewFetchProvider(*tt.compiler, tt.dpol, tt.polex, tt.polexEnabled)
			_, err := provider.Get(ctx, tt.polName)
			if tt.wantErr {
				assert.Error(t, err, err.Error())
			} else {
				assert.NilError(t, err, "expected no error but got one")
			}
		})
	}
}
