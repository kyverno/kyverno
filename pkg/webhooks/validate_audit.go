package webhooks

import (
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
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
	client      client.Interface
	queue       workqueue.RateLimitingInterface
	pCache      policycache.Interface
	eventGen    event.Interface
	prGenerator policyreport.GeneratorInterface

	rbLister  rbaclister.RoleBindingLister
	crbLister rbaclister.ClusterRoleBindingLister
	nsLister  listerv1.NamespaceLister

	log           logr.Logger
	configHandler config.Configuration
	promConfig    *metrics.PromConfig
}

// NewValidateAuditHandler returns a new instance of audit policy handler
func NewValidateAuditHandler(pCache policycache.Interface,
	eventGen event.Interface,
	prGenerator policyreport.GeneratorInterface,
	rbInformer rbacinformer.RoleBindingInformer,
	crbInformer rbacinformer.ClusterRoleBindingInformer,
	namespaces informers.NamespaceInformer,
	log logr.Logger,
	dynamicConfig config.Configuration,
	client client.Interface,
	promConfig *metrics.PromConfig) AuditHandler {

	return &auditHandler{
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
		promConfig:    promConfig,
	}
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
		h.log.Info("incorrect type: expecting type 'AdmissionRequest'", "object", obj)
		return true
	}

	err := h.process(request)
	h.handleErr(err, obj, request)

	return true
}

func (h *auditHandler) process(request *admissionv1.AdmissionRequest) error {
	var roles, clusterRoles []string
	var err error
	// time at which the corresponding the admission request's processing got initiated
	admissionRequestTimestamp := time.Now().Unix()
	logger := h.log.WithName("process")

	policies := h.pCache.GetPolicies(policycache.ValidateAudit, request.Kind.Kind, request.Namespace)

	// getRoleRef only if policy has roles/clusterroles defined
	if containsRBACInfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(h.rbLister, h.crbLister, request, h.configHandler)
		if err != nil {
			logger.Error(err, "failed to get RBAC information for request")
		}
	}

	userRequestInfo := v1beta1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}

	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return errors.Wrap(err, "unable to build variable context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	newResource, oldResource, err := utils.ExtractResources(nil, request)
	if err != nil {
		return errors.Wrap(err, "failed create parse resource")
	}

	if err := ctx.AddImageInfos(&newResource); err != nil {
		return errors.Wrap(err, "failed add image information to policy rule context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         newResource,
		OldResource:         oldResource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    h.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: h.configHandler.ToFilter,
		JSONContext:         ctx,
		Client:              h.client,
		AdmissionOperation:  true,
	}

	vh := &validationHandler{
		log:         h.log,
		eventGen:    h.eventGen,
		prGenerator: h.prGenerator,
	}

	vh.handleValidation(h.promConfig, request, policies, policyContext, namespaceLabels, admissionRequestTimestamp)
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
