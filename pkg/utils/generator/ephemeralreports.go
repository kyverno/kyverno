package generator

import (
	"context"

	"github.com/go-logr/logr"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EphemeralReportGenerator = Generator[*reportsv1.EphemeralReport]

type ephemeralreportsgenerator struct {
	// threshold config.Configuration
	threshold int
	count     int
}

func NewEphemeralReportGenerator() EphemeralReportGenerator {
	return &ephemeralreportsgenerator{
		threshold: 10,
		count:     0,
	}
}

func (g *ephemeralreportsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *reportsv1.EphemeralReport, _ logr.Logger) (*reportsv1.EphemeralReport, error) {
	if g.count >= g.threshold {
		return nil, nil
	}

	report, err := client.ReportsV1().EphemeralReports(resource.GetNamespace()).Create(ctx, resource, metav1.CreateOptions{})
	return report, err
}
