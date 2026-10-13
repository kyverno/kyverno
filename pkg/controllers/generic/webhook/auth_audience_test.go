package webhook

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/webhooks/auth"
	"github.com/stretchr/testify/require"
)

func TestCleanupAuthenticationAudienceMatchesClientConfig(t *testing.T) {
	for _, tc := range []struct {
		server string
		port   int32
		path   string
	}{
		{"", 443, "/validate"},
		{"", 8443, "/verifyttl"},
		{"webhook.example:9443", 443, "/validate"},
	} {
		controller := &controller{server: tc.server, servicePort: tc.port, path: tc.path}
		client := controller.clientConfig(nil)
		audience := auth.NewReceiver(nil, tc.server, tc.port).Audience(tc.path)
		if client.URL != nil {
			require.Equal(t, *client.URL, audience)
		} else {
			require.Equal(t, fmt.Sprintf("https://%s.%s.svc:%d%s", client.Service.Name, client.Service.Namespace, *client.Service.Port, *client.Service.Path), audience)
		}
	}
}
