package webhookconfig

import (
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Ignore)
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Ignore)
	setWildcardConfig(notEmpty)
	assert.Equal(t, notEmpty.isEmpty(), false)
}
