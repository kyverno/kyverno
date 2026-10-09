package policy

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func Test_validateActions_NilRule(t *testing.T) {
	warnings, err := validateActions(0, nil, nil, true, "", "", nil)
	assert.Nil(t, err)
	assert.Nil(t, warnings)
}

func Test_validateActions_ChecksAuthorSeparatelyFromController(t *testing.T) {
	client := dclient.NewEmptyFakeClient()
	kubeClient := client.GetKubeClient().(*kubefake.Clientset)
	users := []string{}
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		users = append(users, review.Spec.User)
		allowed := review.Spec.User == "system:serviceaccount:kyverno:background-controller" || review.Spec.ResourceAttributes.Verb == "get"
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: allowed, Reason: "test authorization decision"},
		}, nil
	})
	rule := &kyvernov1.Rule{
		Name: "generate-configmap",
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "generated",
					Namespace:  "target",
				},
			},
		},
	}
	author := &authenticationv1.UserInfo{Username: "alice", Groups: []string{"policy-authors"}}
	warnings, err := validateActions(0, rule, client, false, "system:serviceaccount:kyverno:background-controller", "", author)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `policy author "alice" is not authorized`)
	assert.Empty(t, warnings)
	assert.True(t, strings.Contains(strings.Join(users, ","), "system:serviceaccount:kyverno:background-controller"))
	assert.True(t, strings.Contains(strings.Join(users, ","), "alice"))
}

func Test_validateActions_GenerateSameKind(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				},
			},
		},
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
				},
			},
		},
	}
	rule.MatchResources.Kinds = []string{"ConfigMap"}

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generation kind and match resource kind should not be the same")
	assert.Nil(t, warnings)
}

func Test_validateActions_GenerateMockSuccess(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
					Name: "test-cm",
				},
			},
		},
	}

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func Test_validateActions_GenerateAuthorIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		author *authenticationv1.UserInfo
	}{
		{name: "unspecified author"},
		{name: "empty user", author: &authenticationv1.UserInfo{}},
		{name: "groups without user", author: &authenticationv1.UserInfo{Groups: []string{"system:authenticated", "policy-authors"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const backgroundSA = "system:serviceaccount:kyverno:background-controller"
			client := dclient.NewEmptyFakeClient()
			kubeClient := client.GetKubeClient().(*kubefake.Clientset)
			users := []string{}
			kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				users = append(users, review.Spec.User)
				return true, &authorizationv1.SubjectAccessReview{
					Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
				}, nil
			})
			rule := &kyvernov1.Rule{
				Name: "generate-configmap",
				Generation: &kyvernov1.Generation{
					GeneratePattern: kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "generated",
							Namespace:  "target",
						},
					},
				},
			}

			warnings, err := validateActions(0, rule, client, false, backgroundSA, "", test.author)
			if test.author == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, `policy author "" is not authorized`)
				assert.ErrorContains(t, err, "no service account provided")
			}
			assert.Empty(t, warnings)
			assert.Equal(t, []string{backgroundSA, backgroundSA}, users)
		})
	}
}
