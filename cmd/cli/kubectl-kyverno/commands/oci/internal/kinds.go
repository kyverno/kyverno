package internal

const SupportedAPIVersion = "policies.kyverno.io/v1beta1"

var LegacyKinds = map[string]bool{
	"Policy":               true,
	"ClusterPolicy":        true,
	"CleanupPolicy":        true,
	"ClusterCleanupPolicy": true,
}

var SupportedCELKinds = map[string]bool{
	"ValidatingPolicy":                true,
	"NamespacedValidatingPolicy":      true,
	"MutatingPolicy":                  true,
	"NamespacedMutatingPolicy":        true,
	"GeneratingPolicy":                true,
	"NamespacedGeneratingPolicy":      true,
	"DeletingPolicy":                  true,
	"NamespacedDeletingPolicy":        true,
	"ImageValidatingPolicy":           true,
	"NamespacedImageValidatingPolicy": true,
	"PolicyException":                 true,
}
