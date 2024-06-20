package generator

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	configutils "github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
)

type UpdateRequestGenerator = Generator[*kyvernov2.UpdateRequest]

type updaterequestsgenerator struct {
	config     configutils.Configuration
	metaClient metadata.Interface
}

func NewUpdateRequestGenerator(config configutils.Configuration, metaClient metadata.Interface) UpdateRequestGenerator {
	return &updaterequestsgenerator{
		config:     config,
		metaClient: metaClient,
	}
}

func (g *updaterequestsgenerator) Generate(ctx context.Context, client versioned.Interface, resource *kyvernov2.UpdateRequest, log logr.Logger) (*kyvernov2.UpdateRequest, error) {
	objects, err := g.metaClient.Resource(
		schema.GroupVersionResource{
			Group:    "kyverno.io",
			Version:  "v2",
			Resource: "updaterequests",
		},
	).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	count := len(objects.Items)
	threshold := g.config.GetUpdateRequestThreshold()
	if int64(count) >= threshold {
		log.Error(errors.New("UpdateRequest creation skipped"),
			"the number of updaterequests exceeds the threshold, please adjust updateRequestThreshold in the Kyverno configmap",
			"current count", count, "threshold", threshold)
		return nil, nil
	}

	created, err := client.KyvernoV2().UpdateRequests(configutils.KyvernoNamespace()).Create(ctx, resource, metav1.CreateOptions{})
	return created, err
}
