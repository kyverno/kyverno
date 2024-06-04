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
	// threshold config.Configuration
	threshold int
	count     int
}

func NewUpdateRequestGenerator() UpdateRequestGenerator {
	return &updaterequestsgenerator{
		threshold: 10,
		count:     0,
	}
}

func (g *updaterequestsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *v1beta1.UpdateRequest) (*v1beta1.UpdateRequest, error) {
	if g.count >= g.threshold {
		return nil, nil
	}

	created, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(ctx, resource, metav1.CreateOptions{})
	return created, err
}
