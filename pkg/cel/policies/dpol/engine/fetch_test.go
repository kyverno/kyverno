package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	policieskyvernoiov1alpha1 "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestGet(t *testing.T) {
	c := compiler.NewCompiler()
	ctx := context.TODO()

	policy := &policiesv1alpha1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: policiesv1alpha1.DeletingPolicySpec{
			// add req. fields if required.
		},
	}

	exception := &policiesv1alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: policiesv1alpha1.PolicyExceptionSpec{
			// add req. fields if required
		},
	}

	dpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	dpolIndexer.Add(policy)

	polexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	polexIndexer.Add(exception)

	polexListers := policieskyvernoiov1alpha1.NewPolicyExceptionLister(polexIndexer)
	dpolListers := policieskyvernoiov1alpha1.NewDeletingPolicyLister(dpolIndexer)

	tests := []struct {
		name         string
		polName      string
		dpol         policieskyvernoiov1alpha1.DeletingPolicyLister
		polex        policieskyvernoiov1alpha1.PolicyExceptionLister
		compiler     *compiler.Compiler
		polexEnabled bool
		wantErr      bool
	}{
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
