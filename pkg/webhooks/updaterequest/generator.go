package updaterequest

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

// UpdateRequest provides interface to manage update requests
type Interface interface {
	Apply(gr kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error
}

// info object stores message data to create update request
type info struct {
	spec   kyvernov1beta1.UpdateRequestSpec
	action admissionv1.Operation
}

// Generator defines the implementation to mange update request resource
type Generator struct {
	client kyvernoclient.Interface
	stopCh <-chan struct{}
	log    logr.Logger

	urLister urkyvernolister.UpdateRequestNamespaceLister
}

// NewGenerator returns a new instance of UpdateRequest resource generator
func NewGenerator(client kyvernoclient.Interface, urInformer urkyvernoinformer.UpdateRequestInformer, stopCh <-chan struct{}, log logr.Logger) *Generator {
	gen := &Generator{
		client:   client,
		stopCh:   stopCh,
		log:      log,
		urLister: urInformer.Lister().UpdateRequests(config.KyvernoNamespace),
	}
	return gen
}

// Apply creates update request resource
func (g *Generator) Apply(ur kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error {
	logger := g.log
	logger.V(4).Info("reconcile Update Request", "request", ur)

	message := info{
		action: action,
		spec:   ur,
	}
	go g.processApply(message)
	return nil
}

// Run starts the update request spec
func (g *Generator) Run(workers int, stopCh <-chan struct{}) {
	logger := g.log
	defer utilruntime.HandleCrash()

	logger.V(4).Info("starting")
	defer func() {
		logger.V(4).Info("shutting down")
	}()

	<-g.stopCh
}

func (g *Generator) processApply(i info) {
	if err := g.generate(i); err != nil {
		logger.Error(err, "failed to update request CR")
	}
}

func (g *Generator) generate(i info) error {
	if err := retryApplyResource(g.client, i.spec, g.log, i.action, g.urLister); err != nil {
		return err
	}
	return nil
}

func retryApplyResource(client kyvernoclient.Interface, urSpec kyvernov1beta1.UpdateRequestSpec,
	log logr.Logger, action admissionv1.Operation, urLister urkyvernolister.UpdateRequestNamespaceLister) error {

	if action == admissionv1.Delete && urSpec.Type == kyvernov1beta1.Generate {
		return nil
	}

	var i int
	var err error

	_, policyName, err := cache.SplitMetaNamespaceKey(urSpec.Policy)
	if err != nil {
		return err
	}

	applyResource := func() error {
		ur := kyvernov1beta1.UpdateRequest{
			Spec: urSpec,
			Status: kyvernov1beta1.UpdateRequestStatus{
				State: kyvernov1beta1.Pending,
			},
		}

		queryLabels := make(map[string]string)
		if ur.Spec.Type == kyvernov1beta1.Mutate {
			queryLabels := map[string]string{
				kyvernov1beta1.URMutatePolicyLabel:                  ur.Spec.Policy,
				"mutate.updaterequest.kyverno.io/trigger-name":      ur.Spec.Resource.Name,
				"mutate.updaterequest.kyverno.io/trigger-namespace": ur.Spec.Resource.Namespace,
				"mutate.updaterequest.kyverno.io/trigger-kind":      ur.Spec.Resource.Kind,
			}

			if ur.Spec.Resource.APIVersion != "" {
				queryLabels["mutate.updaterequest.kyverno.io/trigger-apiversion"] = ur.Spec.Resource.APIVersion
			}
		} else if ur.Spec.Type == kyvernov1beta1.Generate {
			queryLabels = labels.Set(map[string]string{
				kyvernov1beta1.URGeneratePolicyLabel:     policyName,
				"generate.kyverno.io/resource-name":      urSpec.Resource.Name,
				"generate.kyverno.io/resource-kind":      urSpec.Resource.Kind,
				"generate.kyverno.io/resource-namespace": urSpec.Resource.Namespace,
			})
		}

		ur.SetNamespace(config.KyvernoNamespace)
		isExist := false
		log.V(4).Info("apply UpdateRequest", "ruleType", ur.Spec.Type)

		urList, err := urLister.List(labels.SelectorFromSet(queryLabels))
		if err != nil {
			log.Error(err, "failed to get update request for the resource", "kind", urSpec.Resource.Kind, "name", urSpec.Resource.Name, "namespace", urSpec.Resource.Namespace)
			return err
		}

		for _, v := range urList {
			log.V(4).Info("updating existing update request", "name", v.GetName())

			v.Spec.Context = ur.Spec.Context
			v.Spec.Policy = ur.Spec.Policy
			v.Spec.Resource = ur.Spec.Resource
			v.Status.Message = ""

			new, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Update(context.TODO(), v, metav1.UpdateOptions{})
			if err != nil {
				log.V(4).Info("failed to update UpdateRequest, retrying", "retryCount", i, "name", ur.GetName(), "namespace", ur.GetNamespace(), "err", err.Error())
				i++
				return err
			} else {
				log.V(4).Info("successfully updated UpdateRequest", "retryCount", i, "name", ur.GetName(), "namespace", ur.GetNamespace())
			}
			err = retry.RetryOnConflict(common.DefaultRetry, func() error {
				ur, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Get(context.TODO(), new.GetName(), metav1.GetOptions{})
				if err != nil {
					return err
				}
				ur.Status.State = kyvernov1beta1.Pending
				_, err = client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
				return err
			})
			if err != nil {
				log.Error(err, "failed to set UpdateRequest state to Pending")
				return err
			}
			isExist = true
		}

		if !isExist {
			log.V(4).Info("creating new UpdateRequest", "type", ur.Spec.Type)

			ur.SetGenerateName("ur-")
			ur.SetLabels(queryLabels)

			new, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), &ur, metav1.CreateOptions{})
			if err != nil {
				log.V(4).Info("failed to create UpdateRequest, retrying", "retryCount", i, "name", ur.GetGenerateName(), "namespace", ur.GetNamespace(), "err", err.Error())
				i++
				return err
			} else {
				log.V(4).Info("successfully created UpdateRequest", "retryCount", i, "name", new.GetName(), "namespace", ur.GetNamespace())
			}
		}

		return nil
	}

	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}

	exbackoff.Reset()
	err = backoff.Retry(applyResource, exbackoff)

	if err != nil {
		return err
	}

	return nil
}
