package generate

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"gotest.tools/v3/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	var err error
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "(podSelector)": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var genRule kyvernov1.Generation
	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}

	rawGenerate = []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker = NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}
}

// countingSAR wraps a fake kube clientset with a SubjectAccessReview reactor
// that records every request and allows all of them, so a test can assert how
// many SubjectAccessReviews were actually issued.
func countingSAR() (*kubefake.Clientset, func() int) {
	kube := kubefake.NewSimpleClientset()
	var mu sync.Mutex
	count := 0
	kube.PrependReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		count++
		mu.Unlock()
		review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	return kube, func() int {
		mu.Lock()
		defer mu.Unlock()
		return count
	}
}

// Test_NewGenerateFactory_CachesBothCheckers exercises the real factory wiring
// (as opposed to NewFakeGenerate, which bypasses it) to prove both the user and
// the reports authorization checkers are wrapped in the shared ResultCache.
//
// It calls Validate twice on the same factory instance: the first pass populates
// the cache with every (subject, verb, resource) tuple it needs, and the second
// pass must be served entirely from that cache. If a regression stops threading
// the shared cache into either checker, the second Validate would re-issue that
// checker's SubjectAccessReviews and the request count would grow.
func Test_NewGenerateFactory_CachesBothCheckers(t *testing.T) {
	kube, requests := countingSAR()
	client := dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient(nil))

	rule := &kyvernov1.Rule{
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"ConfigMap"},
			},
		},
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
			},
		},
	}

	cache := auth.NewResultCache()
	checker := NewGenerateFactory(client, rule, "user-sa", "reports-sa", logging.GlobalLogger(), cache)

	ctx := context.Background()

	if _, _, err := checker.Validate(ctx, nil); err != nil {
		t.Fatalf("first Validate: unexpected error: %v", err)
	}
	// user checker: {get, create} on ConfigMap = 2 SARs.
	// reports checker: {get, list, watch} on ConfigMap = 3 SARs.
	firstPass := requests()
	assert.Equal(t, 5, firstPass)

	if _, _, err := checker.Validate(ctx, nil); err != nil {
		t.Fatalf("second Validate: unexpected error: %v", err)
	}
	// The second pass is a cache hit for every tuple across both checkers, so no
	// additional SubjectAccessReviews are issued.
	assert.Equal(t, firstPass, requests())
}
