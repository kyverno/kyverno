package updaterequest

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/go-logr/logr"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// GenerateRequests provides interface to manage update requests
type Interface interface {
	Apply(gr urkyverno.UpdateRequestSpec, action admissionv1.Operation) error
}

// info object stores message data to create update request
type info struct {
	spec   urkyverno.UpdateRequestSpec
	action admissionv1.Operation
}

// Generator defines the implementation to mange update request resource
type Generator struct {
	client *kyvernoclient.Clientset
	stopCh <-chan struct{}
	log    logr.Logger
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister
	grSynced cache.InformerSynced

	urLister urkyvernolister.UpdateRequestNamespaceLister
	urSynced cache.InformerSynced
}

// NewGenerator returns a new instance of UpdateRequest resource generator
func NewGenerator(client *kyvernoclient.Clientset, grInformer kyvernoinformer.GenerateRequestInformer, urInformer urkyvernoinformer.UpdateRequestInformer, stopCh <-chan struct{}, log logr.Logger) *Generator {
	gen := &Generator{
		client:   client,
		stopCh:   stopCh,
		log:      log,
		grLister: grInformer.Lister().GenerateRequests(config.KyvernoNamespace),
		grSynced: grInformer.Informer().HasSynced,
		urLister: urInformer.Lister().UpdateRequests(config.KyvernoNamespace),
		urSynced: urInformer.Informer().HasSynced,
	}
	return gen
}

// Apply creates update request resource
func (g *Generator) Apply(gr urkyverno.UpdateRequestSpec, action admissionv1.Operation) error {
	logger := g.log
	logger.V(4).Info("reconcile Update Request", "request", gr)

	message := info{
		action: action,
		spec:   gr,
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

	if !cache.WaitForCacheSync(stopCh, g.grSynced, g.urSynced) {
		logger.Info("failed to sync informer cache")
		return
	}

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

func retryApplyResource(client *kyvernoclient.Clientset, urSpec urkyverno.UpdateRequestSpec,
	log logr.Logger, action admissionv1.Operation, urLister urkyvernolister.UpdateRequestNamespaceLister) error {

	if action == admissionv1.Delete && urSpec.Type == urkyverno.Generate {
		return nil
	}

	var i int
	var err error

	_, policyName, err := cache.SplitMetaNamespaceKey(urSpec.Policy)
	if err != nil {
		return err
	}

	applyResource := func() error {
		ur := urkyverno.UpdateRequest{
			Spec: urSpec,
			Status: urkyverno.UpdateRequestStatus{
				State: urkyverno.Pending,
			},
		}

		queryLabels := make(map[string]string)
		if ur.Spec.Type == urkyverno.Mutate {
			queryLabels := map[string]string{
				urkyverno.URMutatePolicyLabel:                       ur.Spec.Policy,
				"mutate.updaterequest.kyverno.io/trigger-name":      ur.Spec.Resource.Name,
				"mutate.updaterequest.kyverno.io/trigger-namespace": ur.Spec.Resource.Namespace,
				"mutate.updaterequest.kyverno.io/trigger-kind":      ur.Spec.Resource.Kind,
			}

			if ur.Spec.Resource.APIVersion != "" {
				queryLabels["mutate.updaterequest.kyverno.io/trigger-apiversion"] = ur.Spec.Resource.APIVersion
			}
		} else if ur.Spec.Type == urkyverno.Generate {
			queryLabels = labels.Set(map[string]string{
				urkyverno.URGeneratePolicyLabel:          policyName,
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
			logger.Error(err, "failed to get update request for the resource", "kind", urSpec.Resource.Kind, "name", urSpec.Resource.Name, "namespace", urSpec.Resource.Namespace)
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

			new.Status.State = urkyverno.Pending
			if _, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
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

			new.Status.State = urkyverno.Pending
			if _, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
				log.Error(err, "failed to set UpdateRequest state to Pending")
				return err
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
