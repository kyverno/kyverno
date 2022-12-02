package event

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Generator generate events
type Generator struct {
	client dclient.Interface
	// list/get cluster policy
	cpLister kyvernov1listers.ClusterPolicyLister
	// list/get policy
	pLister kyvernov1listers.PolicyLister
	// queue to store event generation requests
	queue workqueue.RateLimitingInterface
	// events generated at policy controller
	policyCtrRecorder record.EventRecorder
	// events generated at admission control
	admissionCtrRecorder record.EventRecorder
	// events generated at namespaced policy controller to process 'generate' rule
	genPolicyRecorder record.EventRecorder
	// events generated at mutateExisting controller
	mutateExistingRecorder record.EventRecorder

	maxQueuedEvents int

	log logr.Logger
}

// Interface to generate event
type Interface interface {
	Add(infoList ...Info)
}

// NewEventGenerator to generate a new event controller
func NewEventGenerator(client dclient.Interface, cpInformer kyvernov1informers.ClusterPolicyInformer, pInformer kyvernov1informers.PolicyInformer, maxQueuedEvents int, log logr.Logger) *Generator {
	gen := Generator{
		client:                 client,
		cpLister:               cpInformer.Lister(),
		pLister:                pInformer.Lister(),
		queue:                  workqueue.NewNamedRateLimitingQueue(rateLimiter(), eventWorkQueueName),
		policyCtrRecorder:      initRecorder(client, PolicyController, log),
		admissionCtrRecorder:   initRecorder(client, AdmissionController, log),
		genPolicyRecorder:      initRecorder(client, GeneratePolicyController, log),
		mutateExistingRecorder: initRecorder(client, MutateExistingController, log),
		maxQueuedEvents:        maxQueuedEvents,
		log:                    log,
	}
	return &gen
}

func rateLimiter() workqueue.RateLimiter {
	return workqueue.DefaultItemBasedRateLimiter()
}

func initRecorder(client dclient.Interface, eventSource Source, log logr.Logger) record.EventRecorder {
	// Initialize Event Broadcaster
	err := scheme.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "failed to add to scheme")
		return nil
	}
	eventBroadcaster := record.NewBroadcaster()
	eventInterface := client.GetEventsInterface()
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: eventInterface,
		},
	)
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{
			Component: eventSource.String(),
		},
	)
	return recorder
}

// Add queues an event for generation
func (gen *Generator) Add(infos ...Info) {
	logger := gen.log

	if gen.queue.Len() > gen.maxQueuedEvents {
		logger.V(5).Info("exceeds the event queue limit, dropping the event", "maxQueuedEvents", gen.maxQueuedEvents, "current size", gen.queue.Len())
		return
	}

	for _, info := range infos {
		if info.Name == "" {
			// dont create event for resources with generateName
			// as the name is not generated yet
			logger.V(4).Info("not creating an event as the resource has not been assigned a name yet", "kind", info.Kind, "name", info.Name, "namespace", info.Namespace)
			continue
		}
		gen.queue.Add(info)
	}
}

// Run begins generator
func (gen *Generator) Run(ctx context.Context, workers int) {
	logger := gen.log
	defer utilruntime.HandleCrash()

	logger.Info("start")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, gen.runWorker, time.Second)
	}
	<-ctx.Done()
}

func (gen *Generator) runWorker(ctx context.Context) {
	for gen.processNextWorkItem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	logger := gen.log
	if err == nil {
		gen.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.V(4).Info("retrying event generation", "key", key, "reason", err.Error())
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}

	gen.queue.Forget(key)
	if !errors.IsNotFound(err) {
		logger.Error(err, "failed to generate event", "key", key)
	}
}

func (gen *Generator) processNextWorkItem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	defer gen.queue.Done(obj)
	var key Info
	var ok bool
	if key, ok = obj.(Info); !ok {
		gen.queue.Forget(obj)
		gen.log.V(2).Info("Incorrect type; expected type 'info'", "obj", obj)
		return true
	}
	err := gen.syncHandler(key)
	gen.handleErr(err, obj)

	return true
}

func (gen *Generator) syncHandler(key Info) error {
	logger := gen.log
	var robj runtime.Object
	var err error
	switch key.Kind {
	case "ClusterPolicy":
		robj, err = gen.cpLister.Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster policy", "name", key.Name)
			return err
		}
	case "Policy":
		robj, err = gen.pLister.Policies(key.Namespace).Get(key.Name)
		if err != nil {
			logger.Error(err, "failed to get policy", "name", key.Name)
			return err
		}
	default:
		robj, err = gen.client.GetResource(context.TODO(), "", key.Kind, key.Namespace, key.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "failed to get resource", "kind", key.Kind, "name", key.Name, "namespace", key.Namespace)
				return nil
			}
			return err
		}
	}

	// set the event type based on reason
	// if skip/pass, reason will be: NORMAL
	// else reason will be: WARNING
	eventType := corev1.EventTypeWarning
	if key.Reason == PolicyApplied.String() || key.Reason == PolicySkipped.String() {
		eventType = corev1.EventTypeNormal
	}

	// based on the source of event generation, use different event recorders
	switch key.Source {
	case AdmissionController:
		gen.admissionCtrRecorder.Event(robj, eventType, key.Reason, key.Message)
	case PolicyController:
		gen.policyCtrRecorder.Event(robj, eventType, key.Reason, key.Message)
	case GeneratePolicyController:
		gen.genPolicyRecorder.Event(robj, eventType, key.Reason, key.Message)
	case MutateExistingController:
		gen.mutateExistingRecorder.Event(robj, eventType, key.Reason, key.Message)
	default:
		logger.Info("info.source not defined for the request")
	}
	return nil
}
