package generate

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// GenerateRequests provides interface to manage generate requests
type GenerateRequests interface {
	Apply(gr kyverno.GenerateRequestSpec, action v1beta1.Operation) error
}

// GeneratorChannel ...
type GeneratorChannel struct {
	spec   kyverno.GenerateRequestSpec
	action v1beta1.Operation
}

// Generator defines the implementation to mange generate request resource
type Generator struct {
	// channel to receive request
	client *kyvernoclient.Clientset
	stopCh <-chan struct{}
	log    logr.Logger
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister
	grSynced cache.InformerSynced
}

// NewGenerator returns a new instance of Generate-Request resource generator
func NewGenerator(client *kyvernoclient.Clientset, grInformer kyvernoinformer.GenerateRequestInformer, stopCh <-chan struct{}, log logr.Logger) *Generator {
	gen := &Generator{
		client:   client,
		stopCh:   stopCh,
		log:      log,
		grLister: grInformer.Lister().GenerateRequests(config.KyvernoNamespace),
		grSynced: grInformer.Informer().HasSynced,
	}
	return gen
}

// Apply creates generate request resource (blocking call if channel is full)
func (g *Generator) Apply(gr kyverno.GenerateRequestSpec, action v1beta1.Operation) error {
	logger := g.log
	logger.V(4).Info("creating Generate Request", "request", gr)

	// Update to channel
	message := GeneratorChannel{
		action: action,
		spec:   gr,
	}
	go g.processApply(message)
	return nil
}

// Run starts the generate request spec
func (g *Generator) Run(workers int, stopCh <-chan struct{}) {
	logger := g.log
	defer utilruntime.HandleCrash()

	logger.V(4).Info("starting")
	defer func() {
		logger.V(4).Info("shutting down")
	}()

	if !cache.WaitForCacheSync(stopCh, g.grSynced) {
		logger.Info("failed to sync informer cache")
		return
	}

	<-g.stopCh
}

func (g *Generator) processApply(m GeneratorChannel) {
	if err := g.generate(m.spec, m.action); err != nil {
		logger.Error(err, "failed to generate request CR")
	}
}

func (g *Generator) generate(grSpec kyverno.GenerateRequestSpec, action v1beta1.Operation) error {
	// create/update a generate request

	if err := retryApplyResource(g.client, grSpec, g.log, action, g.grLister); err != nil {
		return err
	}
	return nil
}

// -> receiving channel to take requests to create request
// use worker pattern to read and create the CR resource

func retryApplyResource(client *kyvernoclient.Clientset, grSpec kyverno.GenerateRequestSpec,
	log logr.Logger, action v1beta1.Operation, grLister kyvernolister.GenerateRequestNamespaceLister) error {

	var i int
	var err error

	applyResource := func() error {
		gr := kyverno.GenerateRequest{
			Spec: grSpec,
		}

		gr.SetNamespace(config.KyvernoNamespace)
		// Initial state "Pending"
		// generate requests created in kyverno namespace
		isExist := false
		if action == v1beta1.Create || action == v1beta1.Update {
			log.V(4).Info("querying all generate requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				"generate.kyverno.io/policy-name":        grSpec.Policy,
				"generate.kyverno.io/resource-name":      grSpec.Resource.Name,
				"generate.kyverno.io/resource-kind":      grSpec.Resource.Kind,
				"generate.kyverno.io/resource-namespace": grSpec.Resource.Namespace,
			}))
			grList, err := grLister.List(selector)
			if err != nil {
				logger.Error(err, "failed to get generate request for the resource", "kind", grSpec.Resource.Kind, "name", grSpec.Resource.Name, "namespace", grSpec.Resource.Namespace)
				return err
			}

			for _, v := range grList {

				grLabels := gr.Labels
				if len(grLabels) == 0 {
					grLabels = make(map[string]string)
				}
				grLabels["resources-update"] = "true"
				gr.SetLabels(grLabels)
				v.Spec.Context = gr.Spec.Context
				v.Spec.Policy = gr.Spec.Policy
				v.Spec.Resource = gr.Spec.Resource

				_, err = client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(context.TODO(), v, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				isExist = true
			}

			if !isExist {
				gr.SetGenerateName("gr-")
				gr.SetLabels(map[string]string{
					"generate.kyverno.io/policy-name":        grSpec.Policy,
					"generate.kyverno.io/resource-name":      grSpec.Resource.Name,
					"generate.kyverno.io/resource-kind":      grSpec.Resource.Kind,
					"generate.kyverno.io/resource-namespace": grSpec.Resource.Namespace,
				})
				_, err = client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Create(context.TODO(), &gr, metav1.CreateOptions{})
				if err != nil {
					return err
				}
			}
		}

		log.V(4).Info("retrying update generate request CR", "retryCount", i, "name", gr.GetGenerateName(), "namespace", gr.GetNamespace())
		i++
		return err
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
