package webhook

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// fakeConfiguration implements config.Configuration with a non-empty GetMatchConditions, so the
// tests below can tell whether build() applied it.
type fakeConfiguration struct{}

func (fakeConfiguration) GetDefaultRegistry() string             { return "" }
func (fakeConfiguration) GetEnableDefaultRegistryMutation() bool { return false }
func (fakeConfiguration) IsExcluded(string, []string, []string, []string) bool {
	return false
}
func (fakeConfiguration) ToFilter(schema.GroupVersionKind, string, string, string) bool { return false }
func (fakeConfiguration) GetGenerateSuccessEvents() bool                                { return false }
func (fakeConfiguration) GetSuccessEventActions() sets.Set[string]                      { return sets.New[string]() }
func (fakeConfiguration) GetWebhook() config.WebhookConfig                              { return config.WebhookConfig{} }
func (fakeConfiguration) GetWebhookAnnotations() map[string]string                      { return nil }
func (fakeConfiguration) GetWebhookLabels() map[string]string                           { return nil }
func (fakeConfiguration) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return []admissionregistrationv1.MatchCondition{{Name: "exempt-principal", Expression: "true"}}
}
func (fakeConfiguration) Load(*corev1.ConfigMap)           {}
func (fakeConfiguration) OnChanged(func())                 {}
func (fakeConfiguration) GetUpdateRequestThreshold() int64 { return 0 }
func (fakeConfiguration) GetMaxContextSize() int64         { return config.DefaultMaxContextSize }

func newTestController(labelSelector *metav1.LabelSelector, opts ...Option) *controller {
	c := &controller{
		webhookName:   "test-webhook",
		path:          "/test",
		rules:         []admissionregistrationv1.RuleWithOperations{{Rule: admissionregistrationv1.Rule{APIGroups: []string{"kyverno.io"}}}},
		failurePolicy: Fail,
		sideEffects:   None,
		labelSelector: labelSelector,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func TestBuildDefaultKeepsConfigMatchConditions(t *testing.T) {
	c := newTestController(nil)
	vwc, err := c.build(fakeConfiguration{}, nil)
	require.NoError(t, err)
	require.Len(t, vwc.Webhooks, 1)
	assert.NotEmpty(t, vwc.Webhooks[0].MatchConditions, "default behaviour must keep applying ConfigMap match conditions")
}

func TestBuildWithoutConfigMatchConditionsDropsThem(t *testing.T) {
	c := newTestController(nil, WithoutConfigMatchConditions())
	vwc, err := c.build(fakeConfiguration{}, nil)
	require.NoError(t, err)
	require.Len(t, vwc.Webhooks, 1)
	webhook := vwc.Webhooks[0]
	assert.Empty(t, webhook.MatchConditions, "WithoutConfigMatchConditions must drop the ConfigMap match conditions")
	assert.Equal(t, Fail, webhook.FailurePolicy)
	assert.Equal(t, None, webhook.SideEffects)
	assert.Nil(t, webhook.ObjectSelector)
	assert.Equal(t, []string{"v1"}, webhook.AdmissionReviewVersions)
}
