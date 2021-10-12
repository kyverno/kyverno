package admissionrequests

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/metrics"
)

func ParsePromMetrics(pm metrics.PromMetrics) PromMetrics {
	return PromMetrics(pm)
}

func ParsePromConfig(pc metrics.PromConfig) PromConfig {
	return PromConfig(pc)
}

func ParseResourceRequestOperation(requestOperationStr string) (metrics.ResourceRequestOperation, error) {
	switch requestOperationStr {
	case "CREATE":
		return metrics.ResourceCreated, nil
	case "UPDATE":
		return metrics.ResourceUpdated, nil
	case "DELETE":
		return metrics.ResourceDeleted, nil
	case "CONNECT":
		return metrics.ResourceConnected, nil
	default:
		return "", fmt.Errorf("unknown request operation made by resource: %s. Allowed requests: 'CREATE', 'UPDATE', 'DELETE', 'CONNECT'", requestOperationStr)
	}
}
