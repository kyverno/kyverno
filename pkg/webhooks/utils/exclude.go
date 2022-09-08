package utils

func ExcludeKyvernoResources(kind string) bool {
	switch kind {
	case "ClusterPolicyReport":
		return true
	case "PolicyReport":
		return true
	case "ReportChangeRequest":
		return true
	case "GenerateRequest":
		return true
	case "ClusterReportChangeRequest":
		return true
	default:
		return false
	}
}
