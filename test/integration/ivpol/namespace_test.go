//go:build integration

package ivpol_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/ivpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// unverifiablePinnedImage is unverifiableImage pinned by digest, so it passes the digest check and the
// policy goes on to verify it. Verification then fails closed when it cannot fetch the image data.
const unverifiablePinnedImage = "registry.invalid/kyverno/unverifiable@sha256:0000000000000000000000000000000000000000000000000000000000000000"

// TestValidate_NamespaceSelector_VerifiesImagesOnlyInLabelledNamespaces covers the usual rollout of
// image verification: it is enforced first in namespaces that opted in with a label, while the rest
// of the cluster is untouched. The same pod must be verified (and fail closed) in an opted-in
// namespace and be admitted in any other, which only holds if the policy's namespaceSelector is
// matched against the namespace's real labels.
func TestValidate_NamespaceSelector_VerifiesImagesOnlyInLabelledNamespaces(t *testing.T) {
	enforced := "payments-" + rand.String(5)
	other := "sandbox-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, enforced, map[string]string{"security.example.com/image-signing": "enforced"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, other, nil)

	policy := newIvpol("require-signed-in-opted-in-namespaces")
	policy.Spec.MatchConstraints.NamespaceSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"security.example.com/image-signing": "enforced"},
	}
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, policy.Name, "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policy.Name)

	resp := h.ValidateClustered(ctx, logr.Discard(),
		framework.PodAdmissionRequest("api", enforced, podRawWithImage(t, "api", enforced, unverifiablePinnedImage)), "", time.Now())
	assert.False(t, resp.Allowed, "an image that cannot be verified must be denied in a namespace that opted in")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "failed to get imagedata",
		"the denial must come from image verification, not from a check that runs before it")

	resp = h.ValidateClustered(ctx, logr.Discard(),
		framework.PodAdmissionRequest("api", other, podRawWithImage(t, "api", other, unverifiablePinnedImage)), "", time.Now())
	assert.True(t, resp.Allowed, "the policy must not apply in a namespace that did not opt in: %v", resp.Result)
}
