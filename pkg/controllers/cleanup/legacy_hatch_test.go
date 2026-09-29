package cleanup

import (
	"context"
	"testing"

	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// legacyCleanupCreateRequest is a bare admission request that denyLegacyWrite/DenyLegacyWrite
// treats as a create of a legacy kyverno.io ClusterCleanupPolicy.
func legacyCleanupCreateRequest() admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "ClusterCleanupPolicy"},
		Operation: admissionv1.Create,
	}
}

// TestNewControllerRegistersLegacyExecutionEscapeHatch is Gap 1 (cleanup side) from the #17708
// test plan: nothing previously observed that constructing this package's controller -- the
// legacy CleanupPolicy/ClusterCleanupPolicy scheduler -- actually registers the escape hatch.
// No other test in this package calls the real NewController (Test_Cleanup_* build a *controller
// struct literal directly), so the hatch is guaranteed unregistered until this test constructs
// one.
//
// Verified by temporarily commenting out the registerLegacyExecutionEscapeHatch() call in
// NewController: the assertion below flipped from allowed to denied (failed), confirming this
// test actually exercises the registration and is not vacuous.
func TestNewControllerRegistersLegacyExecutionEscapeHatch(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
	})
	require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse("false"))

	// The hatch registry is add-only and process-global, so a previous iteration under
	// `go test -count=N` would leave its callback registered and make the assertion below
	// vacuous. Clearing only ever makes the gate stricter.
	deprecations.ClearExecutionEscapeHatch()

	// Before NewController runs, a legacy create must still be denied even with the toggle
	// disabled: nothing in this binary has registered the hatch yet.
	decision, err := deprecations.DecideLegacyWrite(legacyCleanupCreateRequest())
	assert.Equal(t, deprecations.Deny, decision, "the hatch must not be registered before cleanup.NewController runs")
	require.Error(t, err)

	ctx := context.Background()
	kubeClient := fake.NewSimpleClientset()
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kubeInformers.Start(ctx.Done())

	kyvernoClient := versionedfake.NewSimpleClientset()
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoClient, 0)
	kyvernoInformers.Start(ctx.Done())

	configuration := config.NewDefaultConfiguration(false)
	jp := jmespath.New(configuration)

	NewController(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		kyvernoInformers.Kyverno().V2().ClusterCleanupPolicies(),
		kyvernoInformers.Kyverno().V2().CleanupPolicies(),
		kubeInformers.Core().V1().Namespaces().Lister(),
		configuration,
		nil,
		jp,
		event.NewFake(),
		nil,
	)

	// After construction, the same create must be let through: NewController registered the
	// hatch and the toggle is disabled.
	decision, err = deprecations.DecideLegacyWrite(legacyCleanupCreateRequest())
	assert.NotEqual(t, deprecations.Deny, decision, "cleanup.NewController must register the escape hatch")
	assert.NoError(t, err)
}
