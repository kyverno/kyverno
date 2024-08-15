package generator

import (
	"context"

	"github.com/go-logr/logr"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterEphemeralReportGenerator = Generator[*reportsv1.ClusterEphemeralReport]

type clusterephemeralreportsgenerator struct {
	// threshold config.Configuration
	threshold int
	count     int
}

func NewClusterEphemeralReportGenerator() ClusterEphemeralReportGenerator {
	return &clusterephemeralreportsgenerator{
		threshold: 10,
		count:     0,
	}
}

func (g *clusterephemeralreportsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *reportsv1.ClusterEphemeralReport, _ logr.Logger) (*reportsv1.ClusterEphemeralReport, error) {
	if g.count >= g.threshold {
		return nil, nil
	}

	report, err := client.ReportsV1().ClusterEphemeralReports().Create(ctx, resource, metav1.CreateOptions{})
	return report, err
}
