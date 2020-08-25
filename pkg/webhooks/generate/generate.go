package generate

import (
	"fmt"
	"reflect"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/constant"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

//GenerateRequests provides interface to manage generate requests
type GenerateRequests interface {
	Apply(gr kyverno.GenerateRequestSpec, action v1beta1.Operation) error
}

type GeneratorChannel struct {
	spec   kyverno.GenerateRequestSpec
	action v1beta1.Operation
}

// Generator defines the implmentation to mange generate request resource
type Generator struct {
	// channel to receive request
	ch     chan GeneratorChannel
	client *kyvernoclient.Clientset
	stopCh <-chan struct{}
	log    logr.Logger
}

//NewGenerator returns a new instance of Generate-Request resource generator
func NewGenerator(client *kyvernoclient.Clientset, stopCh <-chan struct{}, log logr.Logger) *Generator {
	gen := &Generator{
		ch:     make(chan GeneratorChannel, 1000),
		client: client,
		stopCh: stopCh,
		log:    log,
	}
	return gen
}

//Create to create generate request resoruce (blocking call if channel is full)
func (g *Generator) Apply(gr kyverno.GenerateRequestSpec, action v1beta1.Operation) error {
	logger := g.log
	logger.V(4).Info("creating Generate Request", "request", gr)
	// Send to channel
	message := GeneratorChannel{
		action: action,
		spec:   gr,
	}
	select {
	case g.ch <- message:
		return nil
	case <-g.stopCh:
		logger.Info("shutting down channel")
		return fmt.Errorf("shutting down gr create channel")
	}
}

// Run starts the generate request spec
func (g *Generator) Run(workers int) {
	logger := g.log
	defer utilruntime.HandleCrash()
	logger.V(4).Info("starting")
	defer func() {
		logger.V(4).Info("shutting down")
	}()
	for i := 0; i < workers; i++ {
		go wait.Until(g.processApply, constant.GenerateControllerResync, g.stopCh)
	}
	<-g.stopCh
}

func (g *Generator) processApply() {
	logger := g.log
	for r := range g.ch {
		logger.V(4).Info("recieved generate request", "request", r)
		if err := g.generate(r.spec, r.action); err != nil {
			logger.Error(err, "failed to generate request CR")
		}
	}
}

func (g *Generator) generate(grSpec kyverno.GenerateRequestSpec, action v1beta1.Operation) error {
	// create/update a generate request
	if err := retryApplyResource(g.client, grSpec, g.log, action); err != nil {
		return err
	}
	return nil
}

// -> receiving channel to take requests to create request
// use worker pattern to read and create the CR resource

func retryApplyResource(client *kyvernoclient.Clientset,
	grSpec kyverno.GenerateRequestSpec,
	log logr.Logger,
	action v1beta1.Operation,
) error {
	var i int
	var err error

	applyResource := func() error {
		gr := kyverno.GenerateRequest{
			Spec: grSpec,
		}
		gr.SetGenerateName("gr-")
		gr.SetNamespace(config.KubePolicyNamespace)
		// Initial state "Pending"
		// TODO: status is not updated
		// gr.Status.State = kyverno.Pending
		// generate requests created in kyverno namespace
		isExist := true
		if action == v1beta1.Create || action == v1beta1.Update {
			grList, err := client.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).List(metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, v := range grList.Items {
				if reflect.DeepEqual(grSpec.Resource, v.Spec.Resource) && grSpec.Policy == v.Spec.Policy {
					gr.SetLabels(map[string]string{
						"resources-update": "true",
					})
					_, err = client.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).Update(&gr)
					isExist = false
				}
			}
			if isExist {
				_, err = client.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).Create(&gr)
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
