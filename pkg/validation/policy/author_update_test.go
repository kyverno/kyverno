package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type authorPolicyDiscovery struct {
	dclient.IDiscovery
	cached discovery.CachedDiscoveryInterface
}

func (d authorPolicyDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

func generateAuthorPolicy() *kyvernov1.ClusterPolicy {
	policy := &kyvernov1.ClusterPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: "generate-configmap"},
		Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
			Name: "generate-configmap",
			MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Namespace"}},
			}}},
			Generation: &kyvernov1.Generation{GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: "target", Name: "generated"},
			}},
		}}},
	}
	policy.Spec.Rules[0].Generation.SetData(map[string]interface{}{"data": map[string]interface{}{"key": "value"}})
	return policy
}

func generateAuthorClient(denyController bool) (dclient.Interface, *[]authorizationv1.SubjectAccessReviewSpec) {
	kubeClient := kubefake.NewSimpleClientset()
	kubeClient.Resources = []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "namespaces", Kind: "Namespace"},
			{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
		},
	}}
	disco := authorPolicyDiscovery{
		IDiscovery: dclient.NewFakeDiscoveryClient(nil),
		cached:     memory.NewMemCacheClient(kubeClient.Discovery()),
	}
	client := dclient.NewFakeClientWithDisco(nil, kubeClient, disco)
	reviews := []authorizationv1.SubjectAccessReviewSpec{}
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		reviews = append(reviews, *review.Spec.DeepCopy())
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
			Allowed: review.Spec.User != "alice" && !denyController,
			Reason:  "test authorization decision",
		}}, nil
	})
	return client, &reviews
}

func TestGenerateAuthorChecksOnPolicyUpdates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		change         func(*kyvernov1.ClusterPolicy)
		create         bool
		allowAuthor    bool
		emptyAuthor    bool
		denyController bool
		wantAuthor     bool
		wantError      string
		wantWarning    bool
	}{
		{name: "create", create: true, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "unchanged spec"},
		{name: "metadata only", change: func(p *kyvernov1.ClusterPolicy) {
			p.Labels = map[string]string{"app.kubernetes.io/managed-by": "Helm"}
			p.Annotations = map[string]string{"kubectl.kubernetes.io/last-applied-configuration": "updated"}
			p.ResourceVersion = "2"
		}},
		{name: "target namespace changed", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.Rules[0].Generation.Namespace = "other"
		}, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "authorized target namespace changed", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.Rules[0].Generation.Namespace = "other"
		}, allowAuthor: true, wantAuthor: true, wantWarning: true},
		{name: "match changed", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.Rules[0].MatchResources.Any[0].Names = []string{"other"}
		}, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "synchronization enabled", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.Rules[0].Generation.Synchronize = true
		}, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "generate existing enabled", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.GenerateExisting = true
		}, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "context changed", change: func(p *kyvernov1.ClusterPolicy) {
			p.Spec.Rules[0].Context = []kyvernov1.ContextEntry{{Name: "value", Variable: &kyvernov1.Variable{JMESPath: "`'changed'`"}}}
		}, wantAuthor: true, wantError: `policy author "alice" is not authorized`},
		{name: "unchanged spec with empty author", emptyAuthor: true, wantError: "no service account provided"},
		{name: "unchanged spec retains controller checks", denyController: true, wantError: "requires permissions"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, reviews := generateAuthorClient(test.denyController)
			policy := generateAuthorPolicy()
			var oldPolicy kyvernov1.PolicyInterface
			if !test.create {
				oldPolicy = policy.DeepCopy()
			}
			if test.change != nil {
				test.change(policy)
			}
			author := authenticationv1.UserInfo{Username: "alice", Groups: []string{"policy-authors"}}
			if test.allowAuthor {
				author.Username = "bob"
			}
			if test.emptyAuthor {
				author.Username = ""
			}
			warnings, err := ValidateWithUserInfo(policy, oldPolicy, client, false, "background-controller", "", author)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			if test.wantWarning {
				assert.NotEmpty(t, warnings)
			}
			authorReviewed := false
			for _, review := range *reviews {
				authorReviewed = authorReviewed || review.User == author.Username
			}
			assert.Equal(t, test.wantAuthor, authorReviewed)
			if test.wantError == "" && !test.wantWarning {
				assert.NotEmpty(t, *reviews, "controller authorization must still run")
			}
		})
	}
}

func TestGenerateAuthorChecksBeforeKindWarnings(t *testing.T) {
	t.Parallel()

	missing := kyvernov1.ResourceDescription{Kinds: []string{"MissingKind"}}
	tests := []struct {
		name   string
		change func(*kyvernov1.Rule)
	}{
		{name: "match any", change: func(r *kyvernov1.Rule) { r.MatchResources.Any[0].ResourceDescription = missing }},
		{name: "match all", change: func(r *kyvernov1.Rule) {
			r.MatchResources = kyvernov1.MatchResources{All: kyvernov1.ResourceFilters{{ResourceDescription: missing}}}
		}},
		{name: "match resources", change: func(r *kyvernov1.Rule) { r.MatchResources = kyvernov1.MatchResources{ResourceDescription: missing} }},
		{name: "exclude any", change: func(r *kyvernov1.Rule) {
			r.ExcludeResources = &kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{ResourceDescription: missing}}}
		}},
		{name: "exclude all", change: func(r *kyvernov1.Rule) {
			r.ExcludeResources = &kyvernov1.MatchResources{All: kyvernov1.ResourceFilters{{ResourceDescription: missing}}}
		}},
		{name: "exclude resources", change: func(r *kyvernov1.Rule) { r.ExcludeResources = &kyvernov1.MatchResources{ResourceDescription: missing} }},
	}
	for _, test := range tests {
		for _, user := range []string{"alice", "bob"} {
			t.Run(test.name+"/"+user, func(t *testing.T) {
				t.Parallel()
				client, reviews := generateAuthorClient(false)
				policy := generateAuthorPolicy()
				test.change(&policy.Spec.Rules[0])
				warnings, err := ValidateWithUserInfo(policy, nil, client, false, "background-controller", "reports-controller", authenticationv1.UserInfo{Username: user})
				if user == "alice" {
					require.ErrorContains(t, err, `policy author "alice" is not authorized`)
				} else {
					require.NoError(t, err)
					assert.NotEmpty(t, warnings)
				}
				require.Len(t, *reviews, 2, "only the author's get/create checks run before the kind warning")
				for _, review := range *reviews {
					assert.Equal(t, user, review.User)
				}
			})
		}
	}
}

func TestGenerateAuthorChecksAllRulesBeforeKindWarning(t *testing.T) {
	t.Parallel()

	client, _ := generateAuthorClient(false)
	resources := []string{}
	client.GetKubeClient().(*kubefake.Clientset).PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		resources = append(resources, review.Spec.ResourceAttributes.Resource)
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
			Allowed: review.Spec.ResourceAttributes.Resource == "configmaps",
		}}, nil
	})
	policy := generateAuthorPolicy()
	second := policy.Spec.Rules[0].DeepCopy()
	second.Name = "generate-secret"
	second.Generation.Kind = "Secret"
	policy.Spec.Rules = append(policy.Spec.Rules, *second)
	policy.Spec.Rules[0].MatchResources.Any[0].Kinds = []string{"MissingKind"}
	_, err := ValidateWithUserInfo(policy, nil, client, false, "background-controller", "reports-controller", authenticationv1.UserInfo{Username: "alice"})
	require.ErrorContains(t, err, `spec.rules[1].generate.: policy author "alice" is not authorized`)
	assert.Equal(t, []string{"configmaps", "configmaps", "secrets", "secrets"}, resources)
}

func TestValidationOnlyPolicyRetainsKindWarning(t *testing.T) {
	t.Parallel()

	client, reviews := generateAuthorClient(false)
	policy := generateAuthorPolicy()
	policy.Spec.Rules[0].Generation = nil
	policy.Spec.Rules[0].Validation = &kyvernov1.Validation{Deny: &kyvernov1.Deny{}}
	policy.Spec.Rules[0].MatchResources.Any[0].Kinds = []string{"MissingKind"}
	warnings, err := ValidateWithUserInfo(policy, nil, client, false, "background-controller", "reports-controller", authenticationv1.UserInfo{Username: "alice"})
	require.NoError(t, err)
	assert.NotEmpty(t, warnings)
	assert.Empty(t, *reviews)
}

func TestGenerateAuthorOfflineValidation(t *testing.T) {
	t.Parallel()

	client, reviews := generateAuthorClient(false)
	_, err := ValidateWithUserInfo(generateAuthorPolicy(), nil, client, true, "background-controller", "reports-controller", authenticationv1.UserInfo{Username: "alice"})
	require.NoError(t, err)
	assert.Empty(t, *reviews)
}
