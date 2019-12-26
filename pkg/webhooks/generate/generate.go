package generate

import (
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

type GenerateRequests interface {
	Create(gr kyverno.GenerateRequestSpec) error
}

type Generator struct {
	// channel to recieve request
	ch     chan kyverno.GenerateRequestSpec
	client *kyvernoclient.Clientset
	stopCh <-chan struct{}
}

func NewGenerator(client *kyvernoclient.Clientset, stopCh <-chan struct{}) *Generator {
	gen := &Generator{
		ch:     make(chan kyverno.GenerateRequestSpec, 1000),
		client: client,
		stopCh: stopCh,
	}
	return gen
}

// blocking if channel is full
func (g *Generator) Create(gr kyverno.GenerateRequestSpec) error {
	glog.V(4).Infof("create GR %v", gr)
	// Send to channel
	select {
	case g.ch <- gr:
		return nil
	case <-g.stopCh:
		glog.Info("shutting down channel")
		return fmt.Errorf("shutting down gr create channel")
	}
}

// Run starts the generate request spec
func (g *Generator) Run(workers int) {
	defer utilruntime.HandleCrash()
	glog.V(4).Info("Started generate request")
	defer func() {
		glog.V(4).Info("Shutting down generate request")
	}()
	for i := 0; i < workers; i++ {
		go wait.Until(g.process, time.Second, g.stopCh)
	}
	<-g.stopCh
}

func (g *Generator) process() {
	for r := range g.ch {
		glog.V(4).Infof("recived generate request %v", r)
		if err := g.generate(r); err != nil {
			glog.Errorf("Failed to create Generate Request CR: %v", err)
		}
	}
}

func (g *Generator) generate(grSpec kyverno.GenerateRequestSpec) error {
	// create a generate request
	if err := retryCreateResource(g.client, grSpec); err != nil {
		return err
	}
	return nil
}

// -> recieving channel to take requests to create request
// use worker pattern to read and create the CR resource

func retryCreateResource(client *kyvernoclient.Clientset, grSpec kyverno.GenerateRequestSpec) error {
	var i int
	var err error
	createResource := func() error {
		gr := kyverno.GenerateRequest{
			Spec: grSpec,
		}
		gr.SetGenerateName("gr-")
		gr.SetNamespace("kyverno")
		// generate requests created in kyverno namespace
		_, err = client.KyvernoV1().GenerateRequests("kyverno").Create(&gr)
		glog.V(4).Infof("retry %v create generate request", i)
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
