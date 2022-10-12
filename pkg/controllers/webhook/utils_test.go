package webhook

import (
	"testing"

	"gotest.tools/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore)
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore)
	notEmpty.setWildcard()
	assert.Equal(t, notEmpty.isEmpty(), false)
}
