package generator

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateRequestGenerator = Generator[*v1beta1.UpdateRequest]

type updaterequestsgenerator struct {
	threshold config.Configuration
	count     int64
}

func NewUpdateRequestGenerator(thresholdConfig config.Configuration) UpdateRequestGenerator {
	return &updaterequestsgenerator{
		threshold: thresholdConfig,
		count:     0,
	}
}

func (g *updaterequestsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *v1beta1.UpdateRequest) (*v1beta1.UpdateRequest, error) {
	if g.count >= g.threshold.GetUpdateRequestThreshold() {
		return nil, nil
	}

	created, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(ctx, resource, metav1.CreateOptions{})
	return created, err
}
