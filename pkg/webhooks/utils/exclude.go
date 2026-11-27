package utils

// ExcludeKyvernoResources checks whether a resource kind belongs to Kyverno's internal report or update kinds.
// Deprecated: use ExcludeKyvernoResource for group-scoped exclusion.
func ExcludeKyvernoResources(kind string) bool {
	switch kind {
	case "AdmissionReport",
		"ClusterAdmissionReport",
		"BackgroundScanReport",
		"ClusterBackgroundScanReport",
		"EphemeralReport",
		"ClusterEphemeralReport",
		"UpdateRequest":
		return true
	default:
		return false
	}
}

// ExcludeKyvernoResource checks whether a resource with the specified API group and kind is an internal Kyverno resource.
func ExcludeKyvernoResource(group, kind string) bool {
	switch group {
	case "reports.kyverno.io":
		switch kind {
		case "AdmissionReport",
			"ClusterAdmissionReport",
			"BackgroundScanReport",
			"ClusterBackgroundScanReport",
			"EphemeralReport",
			"ClusterEphemeralReport":
			return true
		}
	case "kyverno.io":
		switch kind {
		case "AdmissionReport",
			"ClusterAdmissionReport",
			"BackgroundScanReport",
			"ClusterBackgroundScanReport",
			"UpdateRequest":
			return true
		}
	}
	return false
}
