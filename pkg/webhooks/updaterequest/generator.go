package updaterequest

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Generator provides interface to manage update requests
type Generator interface {
	Apply(gr kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error
}

// generator defines the implementation to manage update request resource
type generator struct {
	// clients
	client kyvernoclient.Interface

	// listers
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister
}

// NewGenerator returns a new instance of UpdateRequest resource generator
func NewGenerator(client kyvernoclient.Interface, urInformer kyvernov1beta1informers.UpdateRequestInformer) Generator {
	return &generator{
		client:   client,
		urLister: urInformer.Lister().UpdateRequests(config.KyvernoNamespace),
	}
}

// Apply creates update request resource
func (g *generator) Apply(ur kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error {
	logger.V(4).Info("reconcile Update Request", "request", ur)
	if action == admissionv1.Delete && ur.Type == kyvernov1beta1.Generate {
		return nil
	}
	go g.applyResource(ur)
	return nil
}

func (g *generator) applyResource(urSpec kyvernov1beta1.UpdateRequestSpec) {
	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}
	exbackoff.Reset()
	if err := backoff.Retry(func() error { return g.tryApplyResource(urSpec) }, exbackoff); err != nil {
		logger.Error(err, "failed to update request CR")
	}
}

func (g *generator) tryApplyResource(urSpec kyvernov1beta1.UpdateRequestSpec) error {
	l := logger.WithValues("ruleType", urSpec.Type, "kind", urSpec.Resource.Kind, "name", urSpec.Resource.Name, "namespace", urSpec.Resource.Namespace)
	var queryLabels labels.Set

	if urSpec.Type == kyvernov1beta1.Mutate {
		queryLabels = common.MutateLabelsSet(urSpec.Policy, urSpec.Resource)
	} else if urSpec.Type == kyvernov1beta1.Generate {
		queryLabels = common.GenerateLabelsSet(urSpec.Policy, urSpec.Resource)
	}
	urList, err := g.urLister.List(labels.SelectorFromSet(queryLabels))
	if err != nil {
		l.Error(err, "failed to get update request for the resource", "kind", urSpec.Resource.Kind, "name", urSpec.Resource.Name, "namespace", urSpec.Resource.Namespace)
		return err
	}
	for _, v := range urList {
		l := l.WithValues("name", v.GetName())
		l.V(4).Info("updating existing update request")
		if _, err := common.Update(g.client, g.urLister, v.GetName(), func(ur *kyvernov1beta1.UpdateRequest) {
			v.Spec = urSpec
		}); err != nil {
			l.V(4).Error(err, "failed to update UpdateRequest")
			return err
		} else {
			l.V(4).Info("successfully updated UpdateRequest")
		}
		if _, err := common.UpdateStatus(g.client, g.urLister, v.GetName(), kyvernov1beta1.Pending, "", nil); err != nil {
			l.V(4).Error(err, "failed to update UpdateRequest status")
			return err
		}
	}
	if len(urList) == 0 {
		l.V(4).Info("creating new UpdateRequest")
		ur := kyvernov1beta1.UpdateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    config.KyvernoNamespace,
				GenerateName: "ur-",
				Labels:       queryLabels,
			},
			Spec: urSpec,
		}
		if new, err := g.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), &ur, metav1.CreateOptions{}); err != nil {
			l.V(4).Error(err, "failed to create UpdateRequest, retrying", "name", ur.GetGenerateName(), "namespace", ur.GetNamespace())
			return err
		} else {
			l.V(4).Info("successfully created UpdateRequest", "name", new.GetName(), "namespace", ur.GetNamespace())
		}
	}
	return nil
}
