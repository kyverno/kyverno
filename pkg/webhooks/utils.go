package webhooks

import "github.com/nirmata/kube-policy/client"

// KindIsSupported checks kind to be prensent in
// SupportedKinds defined in config
func KindIsSupported(kind string) bool {
	for _, k := range client.GetSupportedKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
