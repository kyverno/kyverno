package generator

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	configutils "github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type UpdateRequestGenerator = Generator[*kyvernov2.UpdateRequest]

type updaterequestsgenerator struct {
	config   configutils.Configuration
	urLister kyvernov2listers.UpdateRequestLister
}

// NewUpdateRequestGenerator returns a generator that checks updateRequestThreshold against the
// UpdateRequests in urLister, across all namespaces. urLister reads an informer cache, so the
// threshold is best-effort: UpdateRequests created just before a call might not be counted yet.
func NewUpdateRequestGenerator(config configutils.Configuration, urLister kyvernov2listers.UpdateRequestLister) UpdateRequestGenerator {
	return &updaterequestsgenerator{
		config:   config,
		urLister: urLister,
	}
}

func (g *updaterequestsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *kyvernov2.UpdateRequest, log logr.Logger) (*kyvernov2.UpdateRequest, error) {
	objects, err := g.urLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	count := len(objects)
	threshold := g.config.GetUpdateRequestThreshold()
	if int64(count) >= threshold {
		log.Error(errors.New("UpdateRequest creation skipped"),
			"the number of updaterequests exceeds the threshold, please adjust updateRequestThreshold in the Kyverno configmap",
			"current count", count, "threshold", threshold)
		return nil, nil
	}
	if g.config.GetEnableUpdateRequestCleanup() {
		ttl := g.config.GetUpdateRequestCleanupTTL()
		if ttl != "" {
			if resource.Labels == nil {
				resource.Labels = make(map[string]string)
			}
			resource.Labels[kyverno.LabelCleanupTtl] = ttl
		}
	}
	created, err := client.KyvernoV2().UpdateRequests(configutils.KyvernoNamespace()).Create(ctx, resource, metav1.CreateOptions{})
	return created, err
}
