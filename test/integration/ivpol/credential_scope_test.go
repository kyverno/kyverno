//go:build integration

package ivpol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	ivpol "github.com/kyverno/kyverno/pkg/webhooks/resource/ivpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// envtest has no policy admission webhook, so these policies simulate objects
// already stored before an upgrade. The real reconciler must retain them and the
// validating resource handler must reject invalid credential scope at runtime.
func TestNamespacedCredentialScopeForStoredPolicies(t *testing.T) {
	const namespace = "credential-scope"
	framework.CreateNamespace(t, testEnv.KubeClient, namespace)
	for _, source := range []string{"credentials", "attestor"} {
		for i, reference := range []string{"regcred", namespace + "/regcred", "tenant-b/regcred", "kyverno/regcred"} {
			t.Run(fmt.Sprintf("%s/%d", source, i), func(t *testing.T) {
				policy := newNivpol(fmt.Sprintf("scope-%s-%d", source, i), namespace)
				// Select no image so a successful scope check requires no external
				// registry. Invalid scope must fail during policy compilation first.
				policy.Spec.MatchImageReferences = []policiesv1beta1.MatchImageReference{{Glob: "no-match.invalid/*"}}
				policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
				policy.Spec.ValidationConfigurations.Required = ptr.To(false)
				policy.Spec.ValidationConfigurations.VerifyDigest = ptr.To(false)
				policy.Spec.ValidationConfigurations.MutateDigest = ptr.To(false)
				if source == "credentials" {
					policy.Spec.Credentials = &policiesv1beta1.Credentials{Secrets: []string{reference}}
				} else {
					policy.Spec.Attestors[0].Cosign.Source = &policiesv1beta1.Source{
						SignaturePullSecrets: []corev1.LocalObjectReference{{Name: reference}},
					}
				}
				createNivpolWithCleanup(t, policy)
				waitForPolicyReady(t, policy.Name, namespace)

				handler := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
				ctx := framework.ContextWithPolicies(context.Background(), policy.Name)
				raw := podRawWithImage(t, "app", namespace, unverifiableImage)
				response := handler.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", namespace, raw), "", time.Now())
				if i < 2 {
					assert.True(t, response.Allowed, "%v", response.Result)
				} else {
					require.False(t, response.Allowed, "stored foreign credential reference must fail closed")
					require.NotNil(t, response.Result)
					assert.Contains(t, response.Result.Message, "instead of policy namespace")
				}
			})
		}
	}
}
