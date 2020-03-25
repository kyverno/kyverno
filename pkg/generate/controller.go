package generate

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/glog"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policystatus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	maxRetries = 5
)

// Controller manages the life-cycle for Generate-Requests and applies generate rule
type Controller struct {
	// dyanmic client implementation
	client *dclient.Client
	// event generator interface
	eventGen event.Interface

	// grStatusControl is used to update GR status
	statusControl StatusControlInterface
	// pLister can list/get cluster policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister

	policyStatusListener policystatus.Listener
}

//NewController returns an instance of the Generate-Request Controller
func NewController(
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	eventGen event.Interface,
	policyStatus policystatus.Listener,
) *Controller {
	return &Controller{
		client:               client,
		eventGen:             eventGen,
		policyStatusListener: policyStatus,
		statusControl:        StatusControl{client: kyvernoclient},
		pLister:              pInformer.Lister(),
		grLister:             grInformer.Lister().GenerateRequests("kyverno"),
	}
}

//Run ...
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	glog.Info("Starting generate-policy controller")
	defer glog.Info("Shutting down generate-policy controller")

	c.sync()
	for i := 0; i < workers; i++ {
		go wait.Until(c.sync, time.Second, stopCh)
	}
}

func (c *Controller) sync() {
	grs, err := c.grLister.List(labels.Everything())
	if err != nil {
		glog.V(4).Info(err)
		return
	}

	for _, gr := range grs {
		startTime := time.Now()
		glog.V(4).Infof("Started syncing GR %q (%v)", gr.Name, startTime)
		err = c.processGR(gr)
		if err != nil {
			glog.Errorf("could not process gr %v due to: %v", gr.Name, err)
			continue
		}
		glog.V(4).Infof("Finished syncing GR %q (%v)", gr.Name, time.Since(startTime))
	}
}
