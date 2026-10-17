package policy

import (
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const generateOrderBackgroundSA = "system:serviceaccount:kyverno:background-controller"
const generateOrderReportsSA = "system:serviceaccount:kyverno:reports-controller"

func generateOrderClient(allowed func(authorizationv1.SubjectAccessReviewSpec) bool) (dclient.Interface, *[]authorizationv1.SubjectAccessReviewSpec) {
	client, _ := generateAuthorClient(false)
	reviews := []authorizationv1.SubjectAccessReviewSpec{}
	client.GetKubeClient().(*kubefake.Clientset).PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		reviews = append(reviews, *review.Spec.DeepCopy())
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
			Allowed: allowed(review.Spec), Reason: "test authorization decision",
		}}, nil
	})
	return client, &reviews
}

func TestGenerateControllerChecksBeforeWarnings(t *testing.T) {
	t.Parallel()

	missing := kyvernov1.ResourceDescription{Kinds: []string{"MissingKind"}}
	tests := []struct {
		name   string
		update bool
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
		{name: "target namespace update", update: true, change: func(r *kyvernov1.Rule) { r.Generation.Namespace = "other" }},
	}
	for _, test := range tests {
		for _, denyPatch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/denyPatch=%t", test.name, denyPatch), func(t *testing.T) {
				t.Parallel()
				client, reviews := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
					return !denyPatch || review.User != generateOrderBackgroundSA || review.ResourceAttributes.Verb != "patch"
				})
				policy := generateAuthorPolicy()
				var oldPolicy kyvernov1.PolicyInterface
				if test.update {
					oldPolicy = policy.DeepCopy()
				}
				test.change(&policy.Spec.Rules[0])
				warnings, err := ValidateWithUserInfo(policy, oldPolicy, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice"})
				if denyPatch {
					require.ErrorContains(t, err, generateOrderBackgroundSA+" requires permissions patch")
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, warnings, "authorized policies must retain the existing warning-only behavior")
				}
				var controllerVerbs []string
				for _, review := range *reviews {
					if review.User == generateOrderBackgroundSA {
						controllerVerbs = append(controllerVerbs, review.ResourceAttributes.Verb)
						assert.Equal(t, policy.Spec.Rules[0].Generation.Namespace, review.ResourceAttributes.Namespace)
					}
					assert.NotEqual(t, generateOrderReportsSA, review.User, "reports checks must stay after warning-only exits")
				}
				assert.ElementsMatch(t, []string{"get", "create", "patch"}, controllerVerbs)
			})
		}
	}
}

func TestGenerateControllerChecksWithoutAuthorBeforeWarnings(t *testing.T) {
	t.Parallel()

	for _, unchangedSpec := range []bool{false, true} {
		t.Run(fmt.Sprintf("unchangedSpec=%t", unchangedSpec), func(t *testing.T) {
			t.Parallel()
			client, reviews := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
				return review.User == generateOrderBackgroundSA && review.ResourceAttributes.Verb != "patch"
			})
			policy := generateAuthorPolicy()
			policy.Spec.Rules[0].MatchResources.Any[0].Kinds = []string{"MissingKind"}
			var err error
			if unchangedSpec {
				oldPolicy := policy.DeepCopy()
				policy.Labels = map[string]string{"changed": "metadata-only"}
				_, err = ValidateWithUserInfo(policy, oldPolicy, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice"})
			} else {
				_, err = Validate(policy, nil, client, false, generateOrderBackgroundSA, generateOrderReportsSA)
			}
			require.ErrorContains(t, err, generateOrderBackgroundSA+" requires permissions patch")
			require.Len(t, *reviews, 3)
			for _, review := range *reviews {
				assert.Equal(t, generateOrderBackgroundSA, review.User, "unchanged specs must not acquire an author permission requirement")
			}
		})
	}
}

func TestGenerateControllerChecksAllRulesBeforeWarning(t *testing.T) {
	t.Parallel()

	client, _ := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
		return review.User != generateOrderBackgroundSA || review.ResourceAttributes.Resource != "secrets" || review.ResourceAttributes.Verb != "patch"
	})
	policy := generateAuthorPolicy()
	second := policy.Spec.Rules[0].DeepCopy()
	second.Name = "generate-secret"
	second.Generation.Kind = "Secret"
	policy.Spec.Rules = append(policy.Spec.Rules, *second)
	policy.Spec.Rules[0].MatchResources.Any[0].Kinds = []string{"MissingKind"}
	_, err := ValidateWithUserInfo(policy, nil, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice"})
	require.ErrorContains(t, err, "spec.rules[1].generate.")
	require.ErrorContains(t, err, generateOrderBackgroundSA+" requires permissions patch for resource v1/Secret")
}

func TestGenerateControllerChecksSyncAdmissionBeforeWarnings(t *testing.T) {
	t.Parallel()

	admissionSA := fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName())
	for _, update := range []bool{false, true} {
		for _, deniedVerb := range []string{"get", "list"} {
			t.Run(fmt.Sprintf("update=%t/deny=%s", update, deniedVerb), func(t *testing.T) {
				t.Parallel()
				client, _ := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
					return review.User != admissionSA || review.ResourceAttributes.Verb != deniedVerb
				})
				policy := generateAuthorPolicy()
				policy.Spec.Rules[0].Generation.Synchronize = true
				var oldPolicy kyvernov1.PolicyInterface
				if update {
					oldPolicy = policy.DeepCopy()
					policy.Spec.Rules[0].Generation.Namespace = "other"
				} else {
					policy.Spec.Rules[0].MatchResources.Any[0].Kinds = []string{"MissingKind"}
				}
				_, err := ValidateWithUserInfo(policy, oldPolicy, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice"})
				require.ErrorContains(t, err, admissionSA+" requires permissions "+deniedVerb)
			})
		}
	}
}

func TestGenerateControllerAndAuthorChecksRunOnce(t *testing.T) {
	t.Parallel()

	admissionSA := fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName())
	for _, synchronize := range []bool{false, true} {
		t.Run(fmt.Sprintf("synchronize=%t", synchronize), func(t *testing.T) {
			t.Parallel()
			groups := []string{"system:authenticated", "policy-authors"}
			client, reviews := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
				return review.User != "alice" || (review.ResourceAttributes.Verb != "patch" && review.ResourceAttributes.Verb != "list")
			})
			policy := generateAuthorPolicy()
			policy.Spec.Rules[0].Generation.Synchronize = synchronize
			warnings, err := ValidateWithUserInfo(policy, nil, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice", Groups: groups})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			verbs := map[string][]string{}
			for _, review := range *reviews {
				verbs[review.User] = append(verbs[review.User], review.ResourceAttributes.Verb)
				if review.User == "alice" {
					assert.Equal(t, groups, review.Groups)
				}
				if review.User != generateOrderReportsSA {
					assert.Equal(t, "configmaps", review.ResourceAttributes.Resource)
					assert.Equal(t, "target", review.ResourceAttributes.Namespace)
				}
			}
			wantAuthor := []string{"get", "create"}
			wantBackground := []string{"get", "create", "patch"}
			if synchronize {
				wantAuthor = append(wantAuthor, "update", "delete")
				wantBackground = append(wantBackground, "update", "delete")
				assert.ElementsMatch(t, []string{"get", "list"}, verbs[admissionSA])
			} else {
				assert.Empty(t, verbs[admissionSA])
			}
			assert.ElementsMatch(t, wantAuthor, verbs["alice"])
			assert.ElementsMatch(t, wantBackground, verbs[generateOrderBackgroundSA])
			assert.Contains(t, verbs[generateOrderReportsSA], "get")
			assert.Contains(t, verbs[generateOrderReportsSA], "list")
			assert.Contains(t, verbs[generateOrderReportsSA], "watch")
		})
	}
}

func TestGenerateControllerReportsDenialRemainsWarning(t *testing.T) {
	t.Parallel()

	client, _ := generateOrderClient(func(review authorizationv1.SubjectAccessReviewSpec) bool {
		return review.User != generateOrderReportsSA || review.ResourceAttributes.Verb != "watch"
	})
	policy := generateAuthorPolicy()
	policy.Spec.Rules[0].Generation.Synchronize = true
	warnings, err := ValidateWithUserInfo(policy, nil, client, false, generateOrderBackgroundSA, generateOrderReportsSA, authenticationv1.UserInfo{Username: "alice"})
	require.NoError(t, err)
	require.Len(t, warnings, 1, "reports permissions are checked once even when synchronization is enabled")
	assert.Contains(t, warnings[0], generateOrderReportsSA+" requires permissions watch for resource Namespace")
}
