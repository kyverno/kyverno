package resource

import (
	"context"
	"testing"

	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/exceptions"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	"github.com/kyverno/sdk/extensions/registryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// legacyCreateRequest is a bare admission request that denyLegacyWrite/DenyLegacyWrite treats as
// a create of a legacy kyverno.io ClusterPolicy: enough to observe whether the escape hatch is
// registered, without needing a real object body.
func legacyCreateRequest() admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
		Operation: admissionv1.Create,
	}
}

// newRealHandlersForHatchTest constructs *resourceHandlers via the real NewHandlers constructor,
// deliberately not NewFakeHandlers: NewFakeHandlers builds the struct literal directly and never
// calls registerLegacyExecutionEscapeHatch, so a test built on it could not fail if that
// registration were deleted from NewHandlers. This helper is the one call site in this package
// that exercises the actual registration wiring.
func newRealHandlersForHatchTest(t *testing.T, pCache policycache.Cache) *resourceHandlers {
	t.Helper()
	ctx := context.Background()

	client := fake.NewSimpleClientset()
	metricsConfig := metrics.NewFakeMetricsConfig()

	informers := kubeinformers.NewSharedInformerFactory(client, 0)
	informers.Start(ctx.Done())

	kyvernoclient := fakekyvernov1.NewSimpleClientset()
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoclient, 0)
	configMapResolver, err := resolvers.NewClientBasedResolver(client)
	require.NoError(t, err)
	kyvernoInformers.Start(ctx.Done())

	dc := dclient.NewEmptyFakeClient()
	configuration := config.NewDefaultConfiguration(false)
	urLister := kyvernoInformers.Kyverno().V2().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
	peLister := kyvernoInformers.Kyverno().V2().PolicyExceptions().Lister()
	jp := jmespath.New(configuration)
	rclient := registryclient.New()

	eng := engine.NewEngine(
		configuration,
		jp,
		adapters.Client(dc),
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(configMapResolver),
		exceptions.New(peLister),
		nil,
	)

	return NewHandlers(
		eng,
		dc,
		kyvernoclient,
		configuration,
		metricsConfig,
		pCache,
		informers.Core().V1().Namespaces().Lister(),
		urLister,
		kyvernoInformers.Kyverno().V1().ClusterPolicies(),
		kyvernoInformers.Kyverno().V1().Policies(),
		updaterequest.NewFake(),
		event.NewFake(),
		false,
		"",
		"",
		jp,
		1,
		1,
	)
}

// TestNewHandlersRegistersLegacyExecutionEscapeHatch is Gap 1 from the #17708 test plan: nothing
// previously observed that constructing legacy execution in the kyverno binary actually
// registers the escape hatch. The nil-cache assertion runs first, within the same test function,
// so it does not depend on cross-test/file ordering: no other test in this package calls the
// real NewHandlers (they all use NewFakeHandlers, see its doc comment), so the hatch is
// guaranteed unregistered until this function's own second half runs.
//
// Verified by temporarily commenting out the registerLegacyExecutionEscapeHatch(pCache) call in
// NewHandlers: both assertions below failed (the nil case already passes trivially without the
// call, but the real-cache case flipped from allowed to denied, i.e. failed), confirming this
// test actually exercises the registration and is not vacuous.
func TestNewHandlersRegistersLegacyExecutionEscapeHatch(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
	})
	require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse("false"))

	// The hatch registry is add-only and process-global, so a previous iteration under
	// `go test -count=N` would leave its callback registered and make the assertion below
	// vacuous. Clearing only ever makes the gate stricter.
	deprecations.ClearExecutionEscapeHatch()

	// A nil cache means this binary constructed no legacy execution; the hatch must not be
	// registered, so a legacy create stays denied even with the toggle disabled.
	newRealHandlersForHatchTest(t, nil)
	decision, err := deprecations.DecideLegacyWrite(legacyCreateRequest())
	assert.Equal(t, deprecations.Deny, decision, "a nil policy cache must not register the escape hatch")
	require.Error(t, err)

	// A real cache means this binary can execute legacy policies; NewHandlers must register the
	// hatch, so the same create is now let through by the disabled toggle.
	newRealHandlersForHatchTest(t, policycache.NewCache())
	decision, err = deprecations.DecideLegacyWrite(legacyCreateRequest())
	assert.NotEqual(t, deprecations.Deny, decision, "constructing NewHandlers with a real policy cache must register the escape hatch")
	assert.NoError(t, err)
}
