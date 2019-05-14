package webhooks

import (
	kubeclient "github.com/nirmata/kube-policy/kubeclient"
)

// KindIsSupported checks kind to be prensent in
// SupportedKinds defined in config
func KindIsSupported(kind string) bool {
	for _, k := range kubeclient.GetSupportedKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
