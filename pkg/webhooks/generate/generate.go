package generate

import (
	"fmt"
	"time"

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
	}
}

func (g *Generator) generate(gr kyverno.GenerateRequestSpec) {
	// create a generate request
}

// -> recieving channel to take requests to create request
// use worker pattern to read and create the CR resource
