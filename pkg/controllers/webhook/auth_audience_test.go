package webhook

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/webhooks/auth"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationAudienceMatchesWebhookClientConfig(t *testing.T) {
	for _, tc := range []struct {
		server string
		port   int32
		path   string
	}{
		{"", 443, "/validate/fail"},
		{"", 8443, "/vpol/policy-a/policy-b"},
		{"", 8443, "/validate/fail/"},
		{"webhook.example:9443", 443, "/validate/fail"},
	} {
		client := newClientConfig(tc.server, tc.port, nil, tc.path)
		audience := auth.NewReceiver(nil, tc.server, tc.port).Audience(tc.path)
		if client.URL != nil {
			require.Equal(t, *client.URL, audience)
		} else {
			require.Equal(t, fmt.Sprintf("https://%s.%s.svc:%d%s", client.Service.Name, client.Service.Namespace, *client.Service.Port, *client.Service.Path), audience)
		}
	}
}
