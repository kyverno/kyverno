package generate

import (
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/constant"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

//GenerateRequests provides interface to manage generate requests
type GenerateRequests interface {
	Create(gr kyverno.GenerateRequestSpec) error
}

// Generator defines the implmentation to mange generate request resource
type Generator struct {
	// channel to receive request
	ch     chan kyverno.GenerateRequestSpec
	client *kyvernoclient.Clientset
	stopCh <-chan struct{}
	log    logr.Logger
}

//NewGenerator returns a new instance of Generate-Request resource generator
func NewGenerator(client *kyvernoclient.Clientset, stopCh <-chan struct{}, log logr.Logger) *Generator {
	gen := &Generator{
		ch:     make(chan kyverno.GenerateRequestSpec, 1000),
		client: client,
		stopCh: stopCh,
		log:    log,
	}
	return gen
}

//Create to create generate request resoruce (blocking call if channel is full)
func (g *Generator) Create(gr kyverno.GenerateRequestSpec) error {
	logger := g.log
	logger.V(4).Info("creating Generate Request", "request", gr)
	// Send to channel
	select {
	case g.ch <- gr:
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
		go wait.Until(g.process, constant.GenerateControllerResync, g.stopCh)
	}
	<-g.stopCh
}

func (g *Generator) process() {
	logger := g.log
	for r := range g.ch {
		logger.V(4).Info("recieved generate request", "request", r)
		if err := g.generate(r); err != nil {
			logger.Error(err, "failed to generate request CR")
		}
	}
}

func (g *Generator) generate(grSpec kyverno.GenerateRequestSpec) error {
	// create a generate request
	if err := retryCreateResource(g.client, grSpec, g.log); err != nil {
		return err
	}
	return nil
}

// -> receiving channel to take requests to create request
// use worker pattern to read and create the CR resource

func retryCreateResource(client *kyvernoclient.Clientset,
	grSpec kyverno.GenerateRequestSpec,
	log logr.Logger,
) error {
	var i int
	var err error
	createResource := func() error {
		gr := kyverno.GenerateRequest{
			Spec: grSpec,
		}
		gr.SetGenerateName("gr-")
		gr.SetNamespace("kyverno")
		// Initial state "Pending"
		// TODO: status is not updated
		// gr.Status.State = kyverno.Pending
		// generate requests created in kyverno namespace
		_, err = client.KyvernoV1().GenerateRequests("kyverno").Create(&gr)
		log.V(4).Info("retrying create generate request CR", "retryCount", i, "name", gr.GetGenerateName(), "namespace", gr.GetNamespace())
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
	err = backoff.Retry(createResource, exbackoff)
	if err != nil {
		return err
	}

	return nil
}
