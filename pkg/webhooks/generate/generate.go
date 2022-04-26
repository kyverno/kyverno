package generate

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
	"k8s.io/client-go/tools/cache"
)

// GenerateRequests provides interface to manage generate requests
type GenerateRequests interface {
	Apply(gr kyverno.GenerateRequestSpec, action admissionv1.Operation) error
}

// GeneratorChannel ...
type GeneratorChannel struct {
	spec   urkyverno.UpdateRequestSpec
	action admissionv1.Operation
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

	// urLister can list/get update request from the shared informer's store
	urLister urkyvernolister.UpdateRequestNamespaceLister
	urSynced cache.InformerSynced
}

// NewGenerator returns a new instance of Generate-Request resource generator
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

// Apply creates generate request resource (blocking call if channel is full)
func (g *Generator) Apply(gr urkyverno.UpdateRequestSpec, action admissionv1.Operation) error {
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

func (g *Generator) processApply(m GeneratorChannel) {
	if err := g.generate(m.spec, m.action); err != nil {
		logger.Error(err, "failed to generate request CR")
	}
}

func (g *Generator) generate(grSpec urkyverno.UpdateRequestSpec, action admissionv1.Operation) error {
	// create/update a generate request

	if err := retryApplyResource(g.client, grSpec, g.log, action, g.grLister, g.urLister); err != nil {
		return err
	}
	return nil
}

// -> receiving channel to take requests to create request
// use worker pattern to read and create the CR resource

func retryApplyResource(client *kyvernoclient.Clientset, grSpec urkyverno.UpdateRequestSpec,
	log logr.Logger, action admissionv1.Operation, grLister kyvernolister.GenerateRequestNamespaceLister,
	urLister urkyvernolister.UpdateRequestNamespaceLister) error {

	var i int
	var err error

	_, policyName, err := cache.SplitMetaNamespaceKey(grSpec.Policy)
	if err != nil {
		return err
	}
	applyResource := func() error {
		gr := urkyverno.UpdateRequest{
			Spec: grSpec,
		}

		gr.SetNamespace(config.KyvernoNamespace)
		// Initial state "Pending"
		// generate requests created in kyverno namespace
		isExist := false
		if action == admissionv1.Create || action == admissionv1.Update {
			log.V(4).Info("querying all generate requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				"generate.kyverno.io/policy-name":        policyName,
				"generate.kyverno.io/resource-name":      grSpec.Resource.Name,
				"generate.kyverno.io/resource-kind":      grSpec.Resource.Kind,
				"generate.kyverno.io/resource-namespace": grSpec.Resource.Namespace,
			}))
			grList, err := urLister.List(selector)
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

				_, err = client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Update(context.TODO(), v, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				isExist = true
			}

			if !isExist {
				gr.SetGenerateName("gr-")
				gr.SetLabels(map[string]string{
					"generate.kyverno.io/policy-name":        policyName,
					"generate.kyverno.io/resource-name":      grSpec.Resource.Name,
					"generate.kyverno.io/resource-kind":      grSpec.Resource.Kind,
					"generate.kyverno.io/resource-namespace": grSpec.Resource.Namespace,
				})
				_, err = client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), &gr, metav1.CreateOptions{})
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
