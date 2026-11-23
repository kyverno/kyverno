package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestImagePolicyAdmissionConfinesSecretReferences(t *testing.T) {
	// Initialize the shared CEL context before parallel admission requests,
	// matching controller startup. Lazy initialization is not synchronized.
	libs.GetLibsCtx()
	t.Parallel()
	for _, source := range []string{"credentials", "attestor"} {
		for _, tc := range []struct {
			name, reference, namespace string
			cluster, allowed           bool
		}{
			{name: "bare name", reference: "registry", namespace: "team-a", allowed: true},
			{name: "same namespace", reference: "team-a/registry", namespace: "team-a", allowed: true},
			{name: "foreign namespace", reference: "team-b/registry", namespace: "team-a"},
			{name: "installation namespace", reference: "kyverno/registry", namespace: "team-a"},
			{name: "malformed reference", reference: "team-a/registry/extra", namespace: "team-a"},
			{name: "empty namespace", reference: "registry"},
			{name: "cluster foreign namespace", reference: "team-b/registry", cluster: true, allowed: true},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				spec := policiesv1beta1.ImageValidatingPolicySpec{MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create}, Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
					}}},
				}}
				if source == "credentials" {
					spec.Credentials = &policiesv1beta1.Credentials{Secrets: []string{tc.reference}}
				} else {
					spec.Attestors = []policiesv1beta1.Attestor{{Name: "signer", Cosign: &policiesv1beta1.Cosign{
						Source: &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: tc.reference}}},
					}}}
				}
				var policy policiesv1beta1.ImageValidatingPolicyLike = &policiesv1beta1.NamespacedImageValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "verify-images", Namespace: tc.namespace}, Spec: spec,
				}
				if tc.cluster {
					policy = &policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "verify-images"}, Spec: spec}
				}
				raw, err := json.Marshal(policy)
				require.NoError(t, err)
				request := handlers.AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
					UID: "test", Operation: admissionv1.Create, Namespace: tc.namespace,
					Kind:   metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1beta1", Kind: policy.GetKind()},
					Object: runtime.RawExtension{Raw: raw},
				}}
				response := NewHandlers(nil, nil, "", "").Validate(context.Background(), logr.Discard(), request, "", time.Now())
				assert.Equal(t, tc.allowed, response.Allowed, "%v", response.Result)
				if !tc.allowed {
					require.NotNil(t, response.Result)
					if tc.namespace == "" {
						assert.Contains(t, response.Result.Message, "metadata.namespace")
					} else if source == "credentials" {
						assert.Contains(t, response.Result.Message, "spec.credentials.secrets")
					} else {
						assert.Contains(t, response.Result.Message, "spec.attestors[0].cosign.source.PullSecrets")
					}
				}
			})
		}
	}
}
