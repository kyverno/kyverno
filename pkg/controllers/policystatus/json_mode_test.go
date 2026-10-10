package policystatus

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	auth "github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers/webhook"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// A JSON mode MutatingPolicy is neither registered in the webhook nor scanned by
// the reports controller, so its readiness must not depend on either: both
// conditions are set True and a stale not-ready webhook condition is healed.
// A Kubernetes mode policy with the same shape keeps the normal checks.
func TestReconcileConditions_JSONMode(t *testing.T) {
	t.Parallel()
	secretsRule := []admissionregistrationv1.NamedRuleWithOperations{{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"}},
		},
	}}
	mutation := admissionregistrationv1alpha1.Mutation{
		PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
		JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op:"add",path:"/a",value:1}]`},
	}
	staleWebhook := metav1.Condition{
		Type:               string(policiesv1beta1.PolicyConditionTypeWebhookConfigured),
		Status:             metav1.ConditionFalse,
		Reason:             "Failed",
		Message:            "Policy is not configured in the webhook.",
		LastTransitionTime: metav1.Now(),
	}

	newController := func() controller {
		dc := dclient.NewEmptyFakeClient()
		kube := dc.GetKubeClient().(*kubefake.Clientset)
		kube.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false, Reason: "denied by test"}}, nil
		})
		return controller{
			dclient:          dc,
			client:           versionedfake.NewSimpleClientset(),
			authChecker:      auth.NewSubjectChecker(dc.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), "system:serviceaccount:kyverno:reports-controller", nil),
			polStateRecorder: webhook.NewStateRecorder(nil),
		}
	}

	t.Run("cluster json policy", func(t *testing.T) {
		t.Parallel()
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "json"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
				Mutations:               []admissionregistrationv1alpha1.Mutation{mutation},
			},
			Status: policiesv1beta1.MutatingPolicyStatus{ConditionStatus: policiesv1beta1.ConditionStatus{Conditions: []metav1.Condition{staleWebhook}}},
		}
		status := newController().reconcileConditions(context.Background(), engineapi.NewMutatingPolicy(mpol))
		for _, condType := range []policiesv1beta1.PolicyConditionType{policiesv1beta1.PolicyConditionTypeWebhookConfigured, policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted} {
			cond := findCondition(status.Conditions, condType)
			require.NotNil(t, cond, condType)
			assert.Equal(t, metav1.ConditionTrue, cond.Status, condType)
			assert.Contains(t, cond.Message, "JSON evaluation mode")
		}
	})

	t.Run("namespaced json policy", func(t *testing.T) {
		t.Parallel()
		nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "json", Namespace: "ns"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
				Mutations:               []admissionregistrationv1alpha1.Mutation{mutation},
			},
		}
		status := newController().reconcileConditions(context.Background(), engineapi.NewNamespacedMutatingPolicy(nmpol))
		cond := findCondition(status.Conditions, policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted)
		require.NotNil(t, cond)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
	})

	t.Run("kubernetes policy still checked", func(t *testing.T) {
		t.Parallel()
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "k8s"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{ResourceRules: secretsRule},
				Mutations:        []admissionregistrationv1alpha1.Mutation{mutation},
			},
			Status: policiesv1beta1.MutatingPolicyStatus{ConditionStatus: policiesv1beta1.ConditionStatus{Conditions: []metav1.Condition{staleWebhook}}},
		}
		status := newController().reconcileConditions(context.Background(), engineapi.NewMutatingPolicy(mpol))
		webhookCond := findCondition(status.Conditions, policiesv1beta1.PolicyConditionTypeWebhookConfigured)
		require.NotNil(t, webhookCond)
		assert.Equal(t, metav1.ConditionFalse, webhookCond.Status, "stale webhook condition must not be healed for Kubernetes mode")
		rbacCond := findCondition(status.Conditions, policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted)
		require.NotNil(t, rbacCond)
		assert.Equal(t, metav1.ConditionFalse, rbacCond.Status)
		assert.Contains(t, rbacCond.Message, "missing permissions")
	})
}
