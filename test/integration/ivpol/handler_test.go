//go:build integration

package ivpol_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
// the in-tree cosign verifier tests. Policies here verify against it; the hermetic tests never run
// the verification (they read a pre-stamped outcome), but the key keeps the policy authentic and
// lets the reconciler compile it.
const cosignPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`

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

// ivpolSpec returns an authentic ImageValidatingPolicy spec: match pods on CREATE, only images under
// ghcr.io, verify a cosign signature against the test key. actions selects the validation actions,
// defaulting to Deny when none are given.
func ivpolSpec(actions ...admissionregistrationv1.ValidationAction) policiesv1beta1.ImageValidatingPolicySpec {
	if len(actions) == 0 {
		actions = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	}
	return policiesv1beta1.ImageValidatingPolicySpec{
		MatchConstraints:     framework.PodMatchRules(),
		ValidationAction:     actions,
		MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
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

// podRawWithOutcomes builds a Pod carrying the image-verification-outcomes annotation the validate
// phase reads. outcomes maps a policy name to a status ("pass"/"fail"/"warning"). This is what the
// mutate phase would normally stamp; seeding it lets us exercise the validate decision hermetically.
func podRawWithOutcomes(t *testing.T, name, namespace string, outcomes map[string]string) []byte {
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
			"containers": []any{map[string]any{"name": "app", "image": "ghcr.io/kyverno/test-verify-image:signed"}},
		},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return raw
}

// outcomeStatusFor reports the verification status the mutating phase recorded for a policy in the
// image-verification-outcomes annotation it patches onto the object. An empty string means no verdict
// was recorded, i.e. the policy never got as far as verifying an image. Note that a policy which does
// not apply can still appear with an empty outcome (autogen variants share their parent's name), so
// the status, not the presence of the name, is what says whether verification actually ran.
func outcomeStatusFor(t *testing.T, patch []byte, policyName string) string {
	t.Helper()
	if len(patch) == 0 {
		return ""
	}
	var ops []struct {
		Path  string `json:"path"`
		Value any    `json:"value"`
	}
	require.NoError(t, json.Unmarshal(patch, &ops))

	annotationPath := "/metadata/annotations/" + strings.ReplaceAll(kyverno.AnnotationImageVerifyOutcomes, "/", "~1")
	for _, op := range ops {
		if op.Path != annotationPath {
			continue
		}
		raw, ok := op.Value.(string)
		require.True(t, ok, "outcomes annotation value should be a JSON string")
		outcomes := map[string]struct {
			Status string `json:"status"`
		}{}
		require.NoError(t, json.Unmarshal([]byte(raw), &outcomes))
		return outcomes[policyName].Status
	}
	return ""
}

// podRawWithImage builds a Pod running the given image and no image-verification-outcomes annotation,
// i.e. a pod that has not been through the mutating phase yet.
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

// podRawNoOutcomes builds a Pod with no image-verification-outcomes annotation.
func podRawNoOutcomes(t *testing.T, name, namespace string) []byte {
	t.Helper()
	return podRawWithImage(t, name, namespace, "ghcr.io/kyverno/test-verify-image:signed")
}

// TestValidate_MissingOutcomesAnnotation_FailsClosed proves the two-phase contract is enforced: if a
// pod reaches the validating webhook without the outcomes annotation the mutating phase should have
// stamped, the request is refused rather than admitted unverified.
func TestValidate_MissingOutcomesAnnotation_FailsClosed(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("require-outcomes"))
	waitForPolicyReady(t, "require-outcomes", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawNoOutcomes(t, "unstamped", "default")
	ctx := framework.ContextWithPolicies(context.Background(), "require-outcomes")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("unstamped", "default", raw), "", time.Now())

	assert.False(t, resp.Allowed, "a pod without verification outcomes must not be admitted")
}

// TestValidate_PassOutcome_AdmitsPod is the baseline of the annotation contract: a verified image
// carries a pass outcome and the pod is admitted.
func TestValidate_PassOutcome_AdmitsPod(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("verified-passes"))
	waitForPolicyReady(t, "verified-passes", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithOutcomes(t, "app", "default", map[string]string{"verified-passes": "pass"})
	ctx := framework.ContextWithPolicies(context.Background(), "verified-passes")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "a pod with a passing verification outcome must be admitted")
	assert.Empty(t, resp.Warnings, "a passing outcome must not raise warnings")
}

// TestValidate_WarnAction_AdmitsPodWithWarning covers the rollout path teams use before enforcing:
// the same failing verification only warns when the policy is in Warn mode.
func TestValidate_WarnAction_AdmitsPodWithWarning(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("warn-only", admissionregistrationv1.Warn))
	waitForPolicyReady(t, "warn-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawWithOutcomes(t, "app", "default", map[string]string{"warn-only": "fail"})
	ctx := framework.ContextWithPolicies(context.Background(), "warn-only")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "a Warn policy must not block the pod")
	assert.NotEmpty(t, resp.Warnings, "a Warn policy must surface the failed verification as a warning")
}

// TestValidate_OutcomeMissingForPolicy_DeniesPod covers the case where the annotation exists but
// carries no verdict for this policy (for example the policy was created between the two phases):
// the policy is treated as not evaluated and the pod is denied rather than let through.
func TestValidate_OutcomeMissingForPolicy_DeniesPod(t *testing.T) {
	createIvpolWithCleanup(t, newIvpol("late-policy"))
	waitForPolicyReady(t, "late-policy", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	// The annotation only carries a verdict for some other policy.
	raw := podRawWithOutcomes(t, "app", "default", map[string]string{"a-different-policy": "pass"})
	ctx := framework.ContextWithPolicies(context.Background(), "late-policy")
	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "default", raw), "", time.Now())

	assert.False(t, resp.Allowed, "a policy with no recorded verdict must not admit the pod")
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
	raw := podRawWithOutcomes(t, "app", "collide-ns", map[string]string{"shared-name": "fail"})

	// The namespaced route must pick the namespaced (Deny) policy.
	nsResp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "collide-ns", raw), "", time.Now())
	assert.False(t, nsResp.Allowed, "the namespaced route must evaluate the namespaced Deny policy")

	// The clustered route must pick the cluster (Warn) policy.
	clusterResp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "collide-ns", raw), "", time.Now())
	assert.True(t, clusterResp.Allowed, "the clustered route must evaluate the cluster Warn policy")
	assert.NotEmpty(t, clusterResp.Warnings, "the cluster Warn policy must surface a warning")
}

// TestMutateNamespaced_PolicyDoesNotApplyAcrossNamespaces mirrors the validate-phase scoping check on
// the mutating route: a namespaced policy must not even be evaluated for a pod in another namespace,
// so no verification runs and no outcomes annotation is stamped.
func TestMutateNamespaced_PolicyDoesNotApplyAcrossNamespaces(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "mutate-a")
	framework.CreateNamespace(t, testEnv.KubeClient, "mutate-b")
	createNivpolWithCleanup(t, newNivpol("mutate-crossns", "mutate-a"))
	waitForPolicyReady(t, "mutate-crossns", "mutate-a")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	raw := podRawNoOutcomes(t, "app", "mutate-b")
	ctx := framework.ContextWithPolicies(context.Background(), "mutate-crossns")
	resp := h.MutateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "mutate-b", raw), "", time.Now())

	assert.True(t, resp.Allowed, "the mutating route must admit a pod no namespaced policy applies to")
	assert.Empty(t, outcomeStatusFor(t, resp.Patch, "mutate-crossns"),
		"a policy from another namespace must not verify the image")
}

// TestMutate_MatchConditionNotMet_SkipsBeforeVerification covers the common "only enforce on
// production workloads" shape: a pod that does not satisfy the policy matchConditions is admitted
// without the policy being evaluated at all, so no image is ever pulled and no verdict is recorded.
func TestMutate_MatchConditionNotMet_SkipsBeforeVerification(t *testing.T) {
	policy := newIvpol("prod-only")
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
		Name:       "check-prod-label",
		Expression: "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'",
	}}
	createIvpolWithCleanup(t, policy)
	waitForPolicyReady(t, "prod-only", "")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	// The pod carries no prod label, so the match condition is not met.
	raw := podRawNoOutcomes(t, "dev-app", "default")
	ctx := framework.ContextWithPolicies(context.Background(), "prod-only")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("dev-app", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "a pod outside the policy match conditions must be admitted")
	assert.Empty(t, outcomeStatusFor(t, resp.Patch, "prod-only"),
		"a policy whose match conditions are not met must not verify the image")
}

// TestMutate_PolicyExceptionSkipsVerification covers the break-glass path: a PolicyException naming
// the policy exempts the matching pod, and the recorded outcome is a skip rather than a verification
// verdict, so the image is never pulled.
func TestMutate_PolicyExceptionSkipsVerification(t *testing.T) {
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

	raw := podRawNoOutcomes(t, "skipped-pod", "default")
	ctx := framework.ContextWithPolicies(context.Background(), "exception-verify")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("skipped-pod", "default", raw), "", time.Now())

	assert.True(t, resp.Allowed, "an exempted pod must be admitted")
	assert.Equal(t, "skip", outcomeStatusFor(t, resp.Patch, "exception-verify"),
		"the exempted pod must record a skip rather than a verification verdict")
}

// TestMutate_ResourceOutsideMatchConstraints_NotEvaluated confirms matchConstraints filtering reaches
// the handler: a policy scoped to pods records nothing for a namespace admission request.
func TestMutate_ResourceOutsideMatchConstraints_NotEvaluated(t *testing.T) {
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
	resp := h.MutateClustered(ctx, logr.Discard(), framework.NamespaceAdmissionRequest("some-namespace", nsRaw), "", time.Now())

	assert.True(t, resp.Allowed, "a resource outside the policy match constraints must be admitted")
	assert.Empty(t, outcomeStatusFor(t, resp.Patch, "pods-only"),
		"a policy scoped to pods must not verify images for a namespace")
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
// NamespacedImageValidatingPolicy in ns-a must not enforce against a pod in ns-b. The ns-b pod even
// carries a stamped "fail" outcome for that policy as a trap; correct namespace scoping filters the
// ns-a policy out so the trap is never consulted, and the pod is admitted.
func TestValidateNamespaced_PolicyDoesNotApplyAcrossNamespaces(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "team-a")
	framework.CreateNamespace(t, testEnv.KubeClient, "team-b")
	createNivpolWithCleanup(t, newNivpol("req-crossns", "team-a"))
	waitForPolicyReady(t, "req-crossns", "team-a")

	h := ivpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	// Pod in team-b carries a fail verdict for the team-a policy; scoping must ignore it.
	raw := podRawWithOutcomes(t, "app", "team-b", map[string]string{"req-crossns": "fail"})
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

	raw := podRawWithOutcomes(t, "app", "team-own", map[string]string{"req-ownns": "fail"})
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
		raw := podRawWithOutcomes(t, "app", ns, map[string]string{"cluster-require-signed": "fail"})
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

	raw := podRawWithOutcomes(t, "app", "route-ns", map[string]string{"cluster-only": "fail"})
	ctx := framework.ContextWithPolicies(context.Background(), "cluster-only")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app", "route-ns", raw), "", time.Now())

	assert.True(t, resp.Allowed, "namespaced route must not apply a cluster policy")
}
