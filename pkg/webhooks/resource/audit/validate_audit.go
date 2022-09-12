package audit

import (
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	workQueueName       = "validate-audit-handler"
	workQueueRetryLimit = 3
)

// AuditHandler applies validate audit policies to the admission request
// the handler adds the request to the work queue and returns immediately
// the request is processed in background, with the exact same logic
// when process the admission request in the webhook
type AuditHandler interface {
	Add(request *admissionv1.AdmissionRequest)
	Run(workers int, stopCh <-chan struct{})
}

type auditHandler struct {
	client      dclient.Interface
	queue       workqueue.RateLimitingInterface
	pCache      policycache.Cache
	eventGen    event.Interface
	prGenerator policyreport.GeneratorInterface
	pcBuilder   webhookutils.PolicyContextBuilder

	rbLister  rbacv1listers.RoleBindingLister
	crbLister rbacv1listers.ClusterRoleBindingLister
	nsLister  corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	log           logr.Logger
	configHandler config.Configuration
	metricsConfig *metrics.MetricsConfig
}

// NewValidateAuditHandler returns a new instance of audit policy handler
func NewValidateAuditHandler(pCache policycache.Cache,
	eventGen event.Interface,
	prGenerator policyreport.GeneratorInterface,
	rbInformer rbacv1informers.RoleBindingInformer,
	crbInformer rbacv1informers.ClusterRoleBindingInformer,
	namespaces corev1informers.NamespaceInformer,
	log logr.Logger,
	dynamicConfig config.Configuration,
	client dclient.Interface,
	metricsConfig *metrics.MetricsConfig,
) AuditHandler {
	c := &auditHandler{
		pCache:        pCache,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		eventGen:      eventGen,
		rbLister:      rbInformer.Lister(),
		crbLister:     crbInformer.Lister(),
		nsLister:      namespaces.Lister(),
		log:           log,
		prGenerator:   prGenerator,
		configHandler: dynamicConfig,
		client:        client,
		metricsConfig: metricsConfig,
		pcBuilder:     webhookutils.NewPolicyContextBuilder(dynamicConfig, client, rbInformer.Lister(), crbInformer.Lister()),
	}
	c.informersSynced = []cache.InformerSynced{rbInformer.Informer().HasSynced, crbInformer.Informer().HasSynced, namespaces.Informer().HasSynced}
	return c
}

func (h *auditHandler) Add(request *admissionv1.AdmissionRequest) {
	h.log.V(4).Info("admission request added", "uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	h.queue.Add(request)
}

func (h *auditHandler) Run(workers int, stopCh <-chan struct{}) {
	h.log.V(4).Info("starting")

	defer func() {
		utilruntime.HandleCrash()
		h.log.V(4).Info("shutting down")
	}()

	if !cache.WaitForNamedCacheSync("ValidateAuditHandler", stopCh, h.informersSynced...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(h.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (h *auditHandler) runWorker() {
	for h.processNextWorkItem() {
	}
}

func (h *auditHandler) processNextWorkItem() bool {
	obj, shutdown := h.queue.Get()
	if shutdown {
		return false
	}

	defer h.queue.Done(obj)

	request, ok := obj.(*admissionv1.AdmissionRequest)
	if !ok {
		h.queue.Forget(obj)
		h.log.V(2).Info("incorrect type: expecting type 'AdmissionRequest'", "object", obj)
		return true
	}

	err := h.process(request)
	h.handleErr(err, obj, request)

	return true
}

func (h *auditHandler) process(request *admissionv1.AdmissionRequest) error {
	var err error
	// time at which the corresponding the admission request's processing got initiated
	admissionRequestTimestamp := time.Now()
	logger := h.log.WithName("process")

	policies := h.pCache.GetPolicies(policycache.ValidateAudit, request.Kind.Kind, request.Namespace)

	policyContext, err := h.pcBuilder.Build(request, policies...)
	if err != nil {
		logger.Error(err, "failed create policy context")
		return errors.Wrap(err, "failed create policy context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	vh := validation.NewValidationHandler(h.log, h.eventGen, h.prGenerator)
	vh.HandleValidation(h.metricsConfig, request, policies, policyContext, namespaceLabels, admissionRequestTimestamp)
	return nil
}

func (h *auditHandler) handleErr(err error, key interface{}, request *admissionv1.AdmissionRequest) {
	logger := h.log.WithName("handleErr")
	if err == nil {
		h.queue.Forget(key)
		return
	}

	k := strings.Join([]string{request.Kind.Kind, request.Namespace, request.Name}, "/")
	if h.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.V(3).Info("retrying processing admission request", "key", k, "error", err.Error())
		h.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process admission request", "key", k)
	h.queue.Forget(key)
}
