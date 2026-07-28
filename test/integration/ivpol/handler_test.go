//go:build integration

package ivpol_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	ivpol "github.com/kyverno/kyverno/pkg/webhooks/resource/ivpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnv  *framework.TestEnv
	engine   ivpolengine.Engine
	provider ivpolengine.Provider
)

// cosignPubKey is the public key for ghcr.io/kyverno/test-verify-image:signed, the same key used by
// the in-tree cosign verifier tests. Policies here verify against it. The hermetic tests never reach
// a registry, so verification of the unsigned or unreachable images they use fails and the policies
// behave as configured (deny or warn); the registry-tagged tests use this key against the real signed
// image.
const cosignPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`

// unverifiableImage points at a registry host that can never resolve ("registry.invalid" uses the
// RFC 6761 reserved .invalid TLD). Verification of it fails fast and closed at name resolution, with
// no network round trip, so the deny and warn assertions below are deterministic and independent of
// whether the test host has (or flakily has) registry egress. The real signed/unsigned image checks
// that need a registry live in the registry-tagged tests.
const unverifiableImage = "registry.invalid/kyverno/unverifiable:latest"

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	engine, provider, err = framework.NewIvpolEngineWithExceptions(testEnv.Mgr, testEnv.KubeClient)
	if err != nil {
		testEnv.Stop()
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

// waitForPolicyReady waits until the reconciler has loaded the policy with the given name and
// namespace (empty namespace for a cluster scoped policy). It matches by identity rather than by
// count so it tolerates the shared provider carrying policies from other tests, and autogen variants
// (which share their parent's name but target pod controllers, so the pod-scoped handler filters
// them out at match time).
func waitForPolicyReady(t *testing.T, name, namespace string) {
	t.Helper()
	require.Eventually(t, func() bool {
		policies, err := provider.Fetch(context.Background())
		if err != nil {
			return false
		}
		for _, p := range policies {
			if p.Policy.GetName() == name && p.Policy.GetNamespace() == namespace {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond, "policy %q (namespace %q) not reconciled in time", name, namespace)
}

// ivpolSpec returns an authentic ImageValidatingPolicy spec: match pods on CREATE, verify a cosign
// signature on every image against the test key. actions selects the validation actions, defaulting
// to Deny when none are given. The image glob is "*" (verify all images) so the hermetic tests can
// point a pod at an unresolvable reference and still have it selected for verification.
func ivpolSpec(actions ...admissionregistrationv1.ValidationAction) policiesv1beta1.ImageValidatingPolicySpec {
	if len(actions) == 0 {
		actions = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	}
	return policiesv1beta1.ImageValidatingPolicySpec{
		MatchConstraints:     framework.PodMatchRules(),
		ValidationAction:     actions,
		MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "*"}},
		Attestors: []policiesv1beta1.Attestor{{
			Name:   "cosign",
			Cosign: &policiesv1beta1.Cosign{Key: &policiesv1beta1.Key{Data: cosignPubKey}},
		}},
		Validations: []admissionregistrationv1.Validation{{
			Expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.cosign])).all(e, e > 0)",
			Message:    "images must be signed by the platform cosign key",
		}},
	}
}

func newIvpol(name string, actions ...admissionregistrationv1.ValidationAction) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       ivpolSpec(actions...),
	}
}

func newNivpol(name, namespace string, actions ...admissionregistrationv1.ValidationAction) *policiesv1beta1.NamespacedImageValidatingPolicy {
	return &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       ivpolSpec(actions...),
	}
}

func createIvpolWithCleanup(t *testing.T, policy *policiesv1beta1.ImageValidatingPolicy) {
	t.Helper()
	require.NoError(t, testEnv.Client.Create(context.Background(), policy))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), policy) })
}

func createNivpolWithCleanup(t *testing.T, policy *policiesv1beta1.NamespacedImageValidatingPolicy) {
	t.Helper()
	require.NoError(t, testEnv.Client.Create(context.Background(), policy))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), policy) })
}

// podRawWithImageAndOutcomes builds a Pod running the given image and carrying a forged
// image-verification-outcomes annotation. outcomes maps a policy name to a status ("pass"/"fail").
// The annotation is what an attacker (or a skipped mutate webhook) could pre-stamp; the validating
// phase must ignore it and verify the image itself, so tests use this to prove the stamp is not
// trusted (see issue #16336).
func podRawWithImageAndOutcomes(t *testing.T, name, namespace, image string, outcomes map[string]string) []byte {
	t.Helper()
	stamped := map[string]map[string]string{}
	for polName, status := range outcomes {
		stamped[polName] = map[string]string{
			"name":    polName,
			"message": "verified image signatures",
			"status":  status,
		}
	}
	outcomesJSON, err := json.Marshal(stamped)
	require.NoError(t, err)

	pod := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":        name,
			"namespace":   namespace,
			"annotations": map[string]string{kyverno.AnnotationImageVerifyOutcomes: string(outcomesJSON)},
		},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "app", "image": image}},
		},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return raw
}

// podRawWithImage builds a Pod running the given image, with no image-verification-outcomes
// annotation. This is the normal shape the validating phase verifies.
func podRawWithImage(t *testing.T, name, namespace, image string) []byte {
	t.Helper()
	pod := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "app", "image": image}},
		},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return raw
}

// TestValidate_UnverifiableImage_FailsClosed is the fail-closed baseline: when an image cannot be
// verified (it is unsigned, and the hermetic run has no registry egress to complete the check
// either), a Deny policy refuses the pod rather than admitting it unverified.
func TestValidate_UnverifiableImage_FailsClosed(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("require-signed"))
	waitForPolicyReady(t, "require-signed", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "unsigned-pod", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "require-signed")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("unsigned-pod", "default", raw), "", time.Now())

	assert.False(t, resp.Allowed, "a pod whose image cannot be verified must not be admitted")
}

// TestValidate_StampedOutcomeIsNotTrusted proves the annotation-trust hardening from #16336: the
// validating phase does its own image verification and never trusts a pre-stamped
// image-verification-outcomes annotation. A pod forges a passing outcome for an image that does not
// actually verify, and it is still denied. (A genuinely signed image being admitted is covered by
// the registry-tagged tests, which can reach a registry to complete the signature check.)
func TestValidate_StampedOutcomeIsNotTrusted(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("reverify-stamped"))
	waitForPolicyReady(t, "reverify-stamped", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImageAndOutcomes(t, "app", "default", unverifiableImage, map[string]string{"reverify-stamped": "pass"})
	ctx := framework.ContextWithPolicies(context.Background(), "reverify-stamped")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.False(t, resp.Allowed, "a forged pass outcome must not be trusted; the image is re-verified and denied")
}

// TestValidate_WarnAction_AdmitsPodWithWarning covers the rollout path teams use before enforcing: a
// failing verification only warns, and does not block, when the policy is in Warn mode.
func TestValidate_WarnAction_AdmitsPodWithWarning(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("warn-only", admissionregistrationv1.Warn))
	waitForPolicyReady(t, "warn-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "warn-only")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "a Warn policy must not block the pod")
	assert.NotEmpty(t, resp.Warnings, "a Warn policy must surface the failed verification as a warning")
}

// TestValidate_MultiplePolicies_DenyAndWarnEnforcedIndependently covers a pod matched by two cluster
// policies at once: a Deny policy and a Warn policy. Both fail verification, so the request is blocked
// by the Deny policy and still carries the Warn policy's warning, proving the per-policy actions are
// aggregated rather than one overriding the other.
func TestValidate_MultiplePolicies_DenyAndWarnEnforcedIndependently(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("enforce-signed"))
	createIvpolWithCleanup(t, newIvpol("observe-signed", admissionregistrationv1.Warn))
	waitForPolicyReady(t, "enforce-signed", "")
	waitForPolicyReady(t, "observe-signed", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "enforce-signed", "observe-signed")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.False(t, resp.Allowed, "the Deny policy must block the pod")
	assert.NotEmpty(t, resp.Warnings, "the Warn policy must still surface its warning alongside the deny")
}

// TestValidate_SameNameClusterAndNamespacedPolicies_DoNotCollide covers the hazard called out by the
// namespaced routing: policy names are not unique across namespaces. A cluster policy and a
// namespaced policy share a name here but differ in action, so the admission outcome reveals which
// one each route actually evaluated.
func TestValidate_SameNameClusterAndNamespacedPolicies_DoNotCollide(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "collide-ns")
	createIvpolWithCleanup(t, newIvpol("shared-name", admissionregistrationv1.Warn))
	createNivpolWithCleanup(t, newNivpol("shared-name", "collide-ns", admissionregistrationv1.Deny))
	waitForPolicyReady(t, "shared-name", "")
	waitForPolicyReady(t, "shared-name", "collide-ns")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), "shared-name")
	raw := podRawWithImage(t, "app", "collide-ns", unverifiableImage)

	// The namespaced route must pick the namespaced (Deny) policy.
	nsResp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "collide-ns", raw), "", time.Now())
	assert.False(t, nsResp.Allowed, "the namespaced route must evaluate the namespaced Deny policy")

	// The clustered route must pick the cluster (Warn) policy.
	clusterResp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "collide-ns", raw), "", time.Now())
	assert.True(t, clusterResp.Allowed, "the clustered route must evaluate the cluster Warn policy")
	assert.NotEmpty(t, clusterResp.Warnings, "the cluster Warn policy must surface a warning")
}

// TestMutate_IsNoOp confirms the mutating webhook no longer verifies images or stamps an outcome
// annotation (issue #16336): verification moved entirely to the validating phase. Even a pod whose
// image would fail verification is admitted unchanged, with no patch, by the mutating route.
func TestMutate_IsNoOp(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("mutate-noop"))
	waitForPolicyReady(t, "mutate-noop", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "mutate-noop")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "the mutating route must admit the pod")
	assert.Empty(t, resp.Patch, "the mutating route must not patch the pod (no verification, no stamped outcome)")
}

// TestValidate_MatchConditionNotMet_SkipsVerification covers the common "only enforce on production
// workloads" shape: a pod that does not satisfy the policy matchConditions is not evaluated at all, so
// its image (which would otherwise fail verification) is never pulled and the pod is admitted.
func TestValidate_MatchConditionNotMet_SkipsVerification(t *testing.T) {
	policy := newIvpol("prod-only")
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
		Name:       "check-prod-label",
		Expression: "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'",
	}}
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "prod-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	// The pod carries no prod label, so the match condition is not met.
	raw := podRawWithImage(t, "dev-app", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "prod-only")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("dev-app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "a pod outside the policy match conditions must be admitted")
	assert.Empty(t, resp.Warnings, "a policy whose match conditions are not met must not verify the image or warn")
}

// TestValidate_PolicyExceptionSkipsVerification covers the break-glass path: a PolicyException naming
// the policy exempts the matching pod, so the validating phase skips verification for it. The image
// is one that would otherwise fail to verify, so admission proves the exception short-circuited
// before the image was pulled rather than the image happening to pass.
func TestValidate_PolicyExceptionSkipsVerification(t *testing.T) {
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "allow-migration-pod", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "exception-verify",
				Kind: "ImageValidatingPolicy",
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "check-name",
				Expression: "object.metadata.name == 'skipped-pod'",
			}},
		},
	}
	require.NoError(t, testEnv.Client.Create(context.Background(), exception))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), exception) })

	createIvpolWithCleanup(t, newIvpol("exception-verify"))
	waitForPolicyReady(t, "exception-verify", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "skipped-pod", "default", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "exception-verify")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("skipped-pod", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "an exempted pod must be admitted")
	assert.Empty(t, resp.Warnings, "an exempted pod must not raise warnings")
}

// TestMutate_ResourceOutsideMatchConstraints_NotEvaluated confirms matchConstraints filtering reaches
// the handler: a policy scoped to pods records nothing for a namespace admission request.
func TestValidate_ResourceOutsideMatchConstraints_NotEvaluated(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("pods-only"))
	waitForPolicyReady(t, "pods-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	nsRaw, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": "some-namespace"},
	})
	require.NoError(t, err)

	ctx := framework.ContextWithPolicies(context.Background(), "pods-only")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.NamespaceAdmissionRequest("some-namespace", nsRaw), "", time.Now())

	assert.True(t, resp.Allowed, "a resource outside the policy match constraints must be admitted")
	assert.Empty(t, resp.Warnings, "a policy scoped to pods must not verify images for a namespace")
}

// TestImageValidatingPolicy_ReconcilesClusterAndNamespaced guards the single-reconciler invariant the
// namespace scoping relies on: one provider must surface both an ImageValidatingPolicy and a
// NamespacedImageValidatingPolicy.
func TestImageValidatingPolicy_ReconcilesClusterAndNamespaced(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "recon-ns")
	createIvpolWithCleanup(t, newIvpol("cluster-verify"))
	createNivpolWithCleanup(t, newNivpol("namespaced-verify", "recon-ns"))

	waitForPolicyReady(t, "cluster-verify", "")
	waitForPolicyReady(t, "namespaced-verify", "recon-ns")
}

// TestValidateNamespaced_PolicyDoesNotApplyAcrossNamespaces is the core scoping regression: a
// NamespacedImageValidatingPolicy in ns-a must not enforce against a pod in ns-b. The pod runs an
// image that would fail verification, so admission proves the ns-a policy was scoped out rather than
// evaluated.
func TestValidateNamespaced_PolicyDoesNotApplyAcrossNamespaces(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "team-a")
	framework.CreateNamespace(t, testEnv.KubeClient, "team-b")
	createNivpolWithCleanup(t, newNivpol("req-crossns", "team-a"))
	waitForPolicyReady(t, "req-crossns", "team-a")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "team-b", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "req-crossns")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "team-b", raw), "", time.Now())

	assert.True(t, resp.Allowed, "namespaced policy in team-a must not apply to a pod in team-b")
}

// TestValidateNamespaced_PolicyAppliesInOwnNamespace is the positive control: the same
// NamespacedImageValidatingPolicy denies a pod in its own namespace when the stamped outcome is a fail.
func TestValidateNamespaced_PolicyAppliesInOwnNamespace(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "team-own")
	createNivpolWithCleanup(t, newNivpol("req-ownns", "team-own"))
	waitForPolicyReady(t, "req-ownns", "team-own")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "team-own", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "req-ownns")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "team-own", raw), "", time.Now())

	assert.False(t, resp.Allowed, "namespaced policy must deny an unverified pod in its own namespace")
}

// TestValidateClustered_PolicyAppliesInEveryNamespace confirms a cluster ImageValidatingPolicy
// enforces regardless of the request namespace.
func TestValidateClustered_PolicyAppliesInEveryNamespace(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("cluster-require-signed"))
	waitForPolicyReady(t, "cluster-require-signed", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), "cluster-require-signed")

	for _, ns := range []string{"default", "kube-system"} {
		raw := podRawWithImage(t, "app", ns, unverifiableImage)
		resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", ns, raw), "", time.Now())
		assert.Falsef(t, resp.Allowed, "cluster policy must deny an unverified pod in namespace %q", ns)
	}
}

// TestValidateNamespaced_IgnoresClusterPolicy proves route isolation: with only a cluster policy
// present, the namespaced route must not apply it, so a pod that would fail the cluster policy is
// still admitted through /nivpol.
func TestValidateNamespaced_IgnoresClusterPolicy(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "route-ns")
	createIvpolWithCleanup(t, newIvpol("cluster-only"))
	waitForPolicyReady(t, "cluster-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithImage(t, "app", "route-ns", unverifiableImage)
	ctx := framework.ContextWithPolicies(context.Background(), "cluster-only")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "route-ns", raw), "", time.Now())

	assert.True(t, resp.Allowed, "namespaced route must not apply a cluster policy")
}
