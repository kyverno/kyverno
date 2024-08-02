package updaterequest

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	generatorutils "github.com/kyverno/kyverno/pkg/utils/generator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Generator provides interface to manage update requests
type Generator interface {
	Apply(context.Context, kyvernov1beta1.UpdateRequestSpec) error
}

// generator defines the implementation to manage update request resource
type generator struct {
	// clients
	client versioned.Interface

	// listers
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister

	urGenerator generatorutils.UpdateRequestGenerator
}

// NewGenerator returns a new instance of UpdateRequest resource generator
func NewGenerator(client versioned.Interface, urInformer kyvernov1beta1informers.UpdateRequestInformer, urGenerator generatorutils.UpdateRequestGenerator) Generator {
	return &generator{
		client:      client,
		urLister:    urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		urGenerator: urGenerator,
	}
}

// Apply creates update request resource
func (g *generator) Apply(ctx context.Context, ur kyvernov1beta1.UpdateRequestSpec) error {
	logger.V(4).Info("apply Update Request", "request", ur)
	go g.applyResource(context.TODO(), ur)
	return nil
}

func (g *generator) applyResource(ctx context.Context, urSpec kyvernov1beta1.UpdateRequestSpec) {
	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}
	exbackoff.Reset()
	if err := backoff.Retry(func() error { return g.tryApplyResource(ctx, urSpec) }, exbackoff); err != nil {
		logger.Error(err, "failed to update request CR")
	}
}

func (g *generator) tryApplyResource(ctx context.Context, urSpec kyvernov1beta1.UpdateRequestSpec) error {
	l := logger.WithValues("ruleType", urSpec.GetRequestType(), "resource", urSpec.GetResource().String())
	var queryLabels labels.Set

	if urSpec.GetRequestType() == kyvernov1beta1.Mutate {
		queryLabels = common.MutateLabelsSet(urSpec.Policy, urSpec.GetResource())
	} else if urSpec.GetRequestType() == kyvernov1beta1.Generate {
		queryLabels = common.GenerateLabelsSet(urSpec.Policy, urSpec.GetResource())
	}

	l.V(4).Info("creating new UpdateRequest")
	ur := kyvernov1beta1.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    config.KyvernoNamespace(),
			GenerateName: "ur-",
			Labels:       queryLabels,
		},
		Spec: urSpec,
	}
	created, err := g.urGenerator.Generate(ctx, g.client, &ur, l)
	if err != nil {
		l.V(4).Error(err, "failed to create UpdateRequest, retrying", "name", ur.GetGenerateName(), "namespace", ur.GetNamespace())
		return err
	} else if created == nil {
		return nil
	}
	updated := created.DeepCopy()
	updated.Status.State = kyvernov1beta1.Pending
	_, err = g.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	l.V(4).Info("successfully created UpdateRequest", "name", updated.GetName(), "namespace", ur.GetNamespace())
	return nil
}
