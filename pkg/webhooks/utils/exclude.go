package utils

func ExcludeKyvernoResources(kind string) bool {
	switch kind {
	case "AdmissionReport":
		return true
	case "ClusterAdmissionReport":
		return true
	case "BackgroundScanReport":
		return true
	case "ClusterBackgroundScanReport":
		return true
	case "UpdateRequest":
		return true
	default:
		return false
	}
}
