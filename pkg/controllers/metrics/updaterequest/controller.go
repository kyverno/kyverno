package updaterequest

import (
	"context"

	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/labels"
)

type controller struct {
	urMetrics metrics.UpdateRequestMetrics
	urLister  kyvernov2listers.UpdateRequestNamespaceLister
}

func NewController(urInformer kyvernov2informers.UpdateRequestInformer) {
	c := controller{
		urMetrics: metrics.GetUpdateRequestMetrics(),
		urLister:  urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
	}

	if c.urMetrics != nil {
		if _, err := c.urMetrics.RegisterCallback(c.report); err != nil {
			logger.Error(err, "Failed to register callback")
		}
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	urs, err := c.urLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list update requests")
		return err
	}

	c.urMetrics.RecordTotal(ctx, config.KyvernoNamespace(), int64(len(urs)), observer)
	return nil
}
