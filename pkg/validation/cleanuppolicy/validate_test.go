package cleanuppolicy

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
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

func cleanupPolicy() *kyvernov2.CleanupPolicy {
	return &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup-configmaps", Namespace: "target"},
		Spec: kyvernov2.CleanupPolicySpec{
			MatchResources: kyvernov2.MatchResources{
				Any: []kyvernov1.ResourceFilter{{
					ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}},
				}},
			},
		},
	}
}

func TestCleanupPolicyChecksAuthorSeparatelyFromController(t *testing.T) {
	client := dclient.NewEmptyFakeClient()
	kubeClient := client.GetKubeClient().(*kubefake.Clientset)
	controller := config.KyvernoUserName(config.KyvernoServiceAccountName())
	users := []string{}
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		users = append(users, review.Spec.User)
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: review.Spec.User == controller,
				Reason:  "test authorization decision",
			},
		}, nil
	})

	err := validateAuth(context.Background(), client, cleanupPolicy(), controller, nil, "cleanup controller")
	assert.NoError(t, err)
	err = validateAuth(context.Background(), client, cleanupPolicy(), "alice", []string{"policy-authors"}, "policy author")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `policy author "alice" has no permission`)
	assert.True(t, strings.Contains(strings.Join(users, ","), controller))
	assert.True(t, strings.Contains(strings.Join(users, ","), "alice"))
}

func TestCleanupPolicyAuthorGroupsAndNamespace(t *testing.T) {
	client := dclient.NewEmptyFakeClient()
	kubeClient := client.GetKubeClient().(*kubefake.Clientset)
	groups := []string{"system:authenticated", "cleanup-authors"}
	reviews := []authorizationv1.SubjectAccessReviewSpec{}
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		reviews = append(reviews, *review.Spec.DeepCopy())
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	})

	err := validateAuth(context.Background(), client, cleanupPolicy(), "alice", groups, "policy author")
	assert.NoError(t, err)
	assert.Len(t, reviews, 2)
	for _, review := range reviews {
		assert.Equal(t, review.User, "alice")
		assert.Equal(t, review.Groups, groups)
		assert.Equal(t, review.ResourceAttributes.Namespace, "target")
	}
	assert.Equal(t, reviews[0].ResourceAttributes.Verb, "delete")
	assert.Equal(t, reviews[1].ResourceAttributes.Verb, "list")
}

// cleanupPolicyDiscovery supplies the cached discovery interface used by the
// public validators while retaining the fake resource lookup for authorization.
type cleanupPolicyDiscovery struct {
	dclient.IDiscovery
	cached discovery.CachedDiscoveryInterface
}

func (d cleanupPolicyDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

func TestCleanupAuthorChecksOnPolicyUpdates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		change         func(*kyvernov2.CleanupPolicy)
		create         bool
		emptyAuthor    bool
		denyController bool
		wantAuthor     bool
		wantError      string
	}{
		{name: "create", create: true, wantAuthor: true, wantError: `policy author "alice" has no permission`},
		{name: "unchanged spec"},
		{name: "metadata only", change: func(p *kyvernov2.CleanupPolicy) {
			p.Labels = map[string]string{"app.kubernetes.io/managed-by": "Helm"}
			p.Annotations = map[string]string{"kubectl.kubernetes.io/last-applied-configuration": "updated"}
			p.ResourceVersion = "2"
		}},
		{name: "target kind changed", change: func(p *kyvernov2.CleanupPolicy) {
			p.Spec.MatchResources.Any[0].Kinds = []string{"Secret"}
		}, wantAuthor: true, wantError: `policy author "alice" has no permission`},
		{name: "match changed", change: func(p *kyvernov2.CleanupPolicy) {
			p.Spec.MatchResources.Any[0].Names = []string{"other"}
		}, wantAuthor: true, wantError: `policy author "alice" has no permission`},
		{name: "schedule changed", change: func(p *kyvernov2.CleanupPolicy) {
			p.Spec.Schedule = "*/2 * * * *"
		}, wantAuthor: true, wantError: `policy author "alice" has no permission`},
		{name: "context changed", change: func(p *kyvernov2.CleanupPolicy) {
			p.Spec.Context = []kyvernov1.ContextEntry{{Name: "value", Variable: &kyvernov1.Variable{JMESPath: "`'changed'`"}}}
		}, wantAuthor: true, wantError: `policy author "alice" has no permission`},
		{name: "unchanged spec with empty author", emptyAuthor: true, wantError: "no service account provided"},
		{name: "unchanged spec retains controller checks", denyController: true, wantError: "cleanup controller"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			kubeClient := kubefake.NewSimpleClientset()
			kubeClient.Resources = []*metav1.APIResourceList{{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{{Name: "configmaps", Kind: "ConfigMap", Namespaced: true}},
			}}
			disco := cleanupPolicyDiscovery{
				IDiscovery: dclient.NewFakeDiscoveryClient(nil),
				cached:     memory.NewMemCacheClient(kubeClient.Discovery()),
			}
			client := dclient.NewFakeClientWithDisco(nil, kubeClient, disco)
			controller := config.KyvernoUserName(config.KyvernoServiceAccountName())
			users := []string{}
			kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				users = append(users, review.Spec.User)
				return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: review.Spec.User == controller && !test.denyController,
					Reason:  "test authorization decision",
				}}, nil
			})
			policy := cleanupPolicy()
			policy.Spec.Schedule = "* * * * *"
			var oldPolicy kyvernov2.CleanupPolicyInterface
			if !test.create {
				oldPolicy = policy.DeepCopy()
			}
			if test.change != nil {
				test.change(policy)
			}
			author := authenticationv1.UserInfo{Username: "alice", Groups: []string{"policy-authors"}}
			if test.emptyAuthor {
				author.Username = ""
			}
			err := ValidateAdmission(context.Background(), logr.Discard(), client, policy, oldPolicy, author)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			assert.Contains(t, users, controller, "controller authorization must still run")
			if test.wantAuthor {
				assert.Contains(t, users, "alice")
			} else {
				assert.NotContains(t, users, "alice")
			}
		})
	}
}
