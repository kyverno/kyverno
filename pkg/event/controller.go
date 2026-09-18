package event

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/tools/record/util"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

const (
	maxEventNoteLen = 1024 // Kubernetes Event Note field maximum in bytes
)

var (
	sanitizerRegex      *regexp.Regexp
	sanitizerRegexError error
)

func init() {
	sanitizerRegex, sanitizerRegexError = regexp.Compile(`[^a-zA-Z0-9_.-]`)
}

// truncateMessageToByteLimit truncates a message to fit within the byte limit
// while ensuring the cut point is at a valid UTF-8 character boundary.
// The 3-byte "..." suffix is reserved within the limit.
func truncateMessageToByteLimit(message string, maxBytes int) string {
	byteCount := len(message)
	if byteCount <= maxBytes {
		return message
	}

	// Reserve 3 bytes for "..."
	availableBytes := maxBytes - 3
	if availableBytes < 0 {
		availableBytes = 0
	}

	// Find the byte position just before the truncation point
	truncated := message[:availableBytes]

	// Back up to a valid UTF-8 boundary
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		r, size := utf8.DecodeLastRuneInString(truncated)
		if r == utf8.RuneError {
			break
		}
		truncated = truncated[:len(truncated)-size]
	}

	return truncated + "..."
}

type controller struct {
	logger             logr.Logger
	clientset          v1.EventsV1Interface
	queue              workqueue.Interface
	clock              clock.Clock
	hostname           string
	processors         int
	config             config.Configuration
	podName            string
	eventSource        string
	generateSuccess    bool
	successEventActions sets.String
	metric             *metrics.EventMetrics
	eventCounter       metricshelper.LabelCounterMetric
}

func NewEventGenerator(
	clientset v1.EventsV1Interface,
	logger logr.Logger,
	processors int,
	cfg config.Configuration,
) *controller {
	hostname, _ := os.Hostname()
	if len(hostname) > 63 {
		hostname = hostname[:63]
	}

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = hostname
	}

	eventSource := "kyverno-controller"

	return &controller{
		logger:          logger,
		clientset:       clientset,
		queue:           workqueue.NewNamed("event-controller"),
		clock:           clock.RealClock{},
		hostname:        hostname,
		processors:      processors,
		config:          cfg,
		podName:         podName,
		eventSource:     eventSource,
		generateSuccess: cfg.GenerateSuccessEvents(),
		successEventActions: sets.NewString(cfg.SuccessEventActions()...),
		metric:          metrics.NewEventMetrics(),
	}
}

func (gen *controller) Run(ctx context.Context, numWorkers int) {
	defer gen.queue.ShutDown()

	gen.logger.Info("Starting event controller")
	defer gen.logger.Info("Shutting down event controller")

	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				gen.updateConfig()
			}
		}
	}()

	for i := 0; i < numWorkers; i++ {
		go wait.Until(gen.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

func (gen *controller) updateConfig() {
	gen.generateSuccess = gen.config.GenerateSuccessEvents()
	gen.successEventActions = sets.NewString(gen.config.SuccessEventActions()...)
}

func (gen *controller) runWorker() {
	for gen.processNextItem() {
	}
}

func (gen *controller) processNextItem() bool {
	key, quit := gen.queue.Get()
	if quit {
		return false
	}
	defer gen.queue.Done(key)

	err := gen.syncHandler(key.(Info))
	if err != nil {
		gen.logger.Error(err, "Failed to sync event")
		gen.queue.AddRateLimited(key)
		return true
	}

	gen.queue.Forget(key)
	return true
}

func (gen *controller) syncHandler(info Info) error {
	gen.logger.V(6).Info("processing event", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason, "action", info.Action, "message", info.Message)

	if info.Action == "" || info.Action == string(enginev1.ActionApply) || info.Action == string(enginev1.ActionMutate) {
		if info.Reason != LegacyPolicyPresent {
			if !gen.generateSuccess {
				gen.logger.V(6).Info("skipping event creation, success events disabled", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason, "action", info.Action)
				return nil
			}

			if !gen.successEventActions.Has(string(info.Action)) {
				gen.logger.V(6).Info("skipping event creation, action not in successEventActions", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason, "action", info.Action)
				return nil
			}
		}
	}

	gen.emitEvent(info)
	return nil
}

func (gen *controller) emitEvent(key Info) {
	eventType := generateEventType(key.Type, key.Reason)

	if !util.ValidateEventType(eventType) {
		gen.logger.Error(nil, "Unsupported event type", "eventType", eventType)
		return
	}

	timestamp := metav1.MicroTime{Time: time.Now()}
	refRegarding, err := reference.GetReference(scheme.Scheme, &key.Regarding)
	if err != nil {
		gen.logger.Error(err, "Could not construct reference, will not report event", "object", &key.Regarding, "eventType", eventType, "reason", string(key.Reason), "message", key.Message)
		return
	}

	var refRelated *corev1.ObjectReference
	if key.Related != nil {
		refRelated, err = reference.GetReference(scheme.Scheme, key.Related)
		if err != nil {
			gen.logger.V(9).Info("Could not construct reference", "object", key.Related, "err", err)
		}
	}

	reportingController := string(key.Source)
	reportingInstance := reportingController + "-" + gen.hostname

	t := metav1.Time{Time: gen.clock.Now()}
	namespace := refRegarding.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	message := key.Message
	if len(message) > maxEventNoteLen {
		message = truncateMessageToByteLimit(message, maxEventNoteLen)
	}

	sanitizedName := sanitizeEventName(refRegarding.Name)

	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", sanitizedName, t.UnixNano()),
			Namespace: namespace,
		},
		EventTime:           timestamp,
		Series:              nil,
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
		Action:              string(key.Action),
		Reason:              string(key.Reason),
		Regarding:           *refRegarding,
		Related:             refRelated,
		Note:                message,
		Type:                eventType,
	}

	gen.queue.Add(event)
}

func generateEventType(eventType, reason string) string {
	if eventType != "" {
		return eventType
	}

	switch reason {
	case PolicyApplied, PolicyVerified, ApplyResource, CleanResource:
		return corev1.EventTypeNormal
	case PolicyFailed:
		return corev1.EventTypeWarning
	case LegacyPolicyPresent:
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}

func sanitizeEventName(name string) string {
	if sanitizerRegexError != nil {
		return name
	}
	return sanitizerRegex.ReplaceAllString(name, "-")
}

func (gen *controller) Add(info Info) {
	gen.logger.V(4).Info("adding event to queue", "kind", info.Regarding.Kind, "name", info.Regarding.Name, "namespace", info.Regarding.Namespace, "reason", info.Reason, "action", info.Action, "message", info.Message)
	gen.queue.Add(info)
}