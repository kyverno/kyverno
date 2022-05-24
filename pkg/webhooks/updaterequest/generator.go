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
	"k8s.io/client-go/tools/cache"
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
		urLister: urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
	}
}

// Apply creates update request resource
func (g *generator) Apply(ur kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error {
	logger.V(4).Info("reconcile Update Request", "request", ur)
	if action == admissionv1.Delete && ur.Type == kyvernov1beta1.Generate {
		return nil
	}
	_, policyName, err := cache.SplitMetaNamespaceKey(ur.Policy)
	if err != nil {
		return err
	}
	go g.applyResource(policyName, ur)
	return nil
}

func (g *generator) applyResource(policyName string, urSpec kyvernov1beta1.UpdateRequestSpec) {
	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}
	exbackoff.Reset()
	if err := backoff.Retry(func() error { return g.tryApplyResource(policyName, urSpec) }, exbackoff); err != nil {
		logger.Error(err, "failed to update request CR")
	}
}

func (g *generator) tryApplyResource(policyName string, urSpec kyvernov1beta1.UpdateRequestSpec) error {
	ur := kyvernov1beta1.UpdateRequest{
		Spec: urSpec,
		Status: kyvernov1beta1.UpdateRequestStatus{
			State: kyvernov1beta1.Pending,
		},
	}

	var queryLabels labels.Set
	if ur.Spec.Type == kyvernov1beta1.Mutate {
		queryLabels = common.MutateLabelsSet(ur.Spec.Policy, ur.Spec.Resource)
	} else if ur.Spec.Type == kyvernov1beta1.Generate {
		queryLabels = common.GenerateLabelsSet(ur.Spec.Policy, ur.Spec.Resource)
	}

	ur.SetNamespace(config.KyvernoNamespace())
	isExist := false
	logger.V(4).Info("apply UpdateRequest", "ruleType", ur.Spec.Type)

	urList, err := g.urLister.List(labels.SelectorFromSet(queryLabels))
	if err != nil {
		logger.Error(err, "failed to get update request for the resource", "kind", urSpec.Resource.Kind, "name", urSpec.Resource.Name, "namespace", urSpec.Resource.Namespace)
		return err
	}

	for _, v := range urList {
		logger.V(4).Info("updating existing update request", "name", v.GetName())

		v.Spec.Context = ur.Spec.Context
		v.Spec.Policy = ur.Spec.Policy
		v.Spec.Resource = ur.Spec.Resource
		v.Status.Message = ""

		new, err := g.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Update(context.TODO(), v, metav1.UpdateOptions{})
		if err != nil {
			logger.V(4).Info("failed to update UpdateRequest, retrying", "name", ur.GetName(), "namespace", ur.GetNamespace(), "err", err.Error())
			return err
		} else {
			logger.V(4).Info("successfully updated UpdateRequest", "name", ur.GetName(), "namespace", ur.GetNamespace())
		}

		new.Status.State = kyvernov1beta1.Pending
		if _, err := g.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to set UpdateRequest state to Pending")
			return err
		}

		isExist = true
	}

	if !isExist {
		logger.V(4).Info("creating new UpdateRequest", "type", ur.Spec.Type)
		ur.SetGenerateName("ur-")
		ur.SetLabels(queryLabels)
		if new, err := g.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), &ur, metav1.CreateOptions{}); err != nil {
			logger.V(4).Info("failed to create UpdateRequest, retrying", "name", ur.GetGenerateName(), "namespace", ur.GetNamespace(), "err", err.Error())
			return err
		} else {
			logger.V(4).Info("successfully created UpdateRequest", "name", new.GetName(), "namespace", ur.GetNamespace())
		}
	}
	return nil
}
