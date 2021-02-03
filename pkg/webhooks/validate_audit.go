package webhooks

import (
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/policystatus"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/minio/minio/cmd/logger"
	"github.com/pkg/errors"
	"k8s.io/api/admission/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
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
	Add(request *v1beta1.AdmissionRequest)
	Run(workers int, stopCh <-chan struct{})
}

type auditHandler struct {
	client         *client.Client
	queue          workqueue.RateLimitingInterface
	pCache         policycache.Interface
	eventGen       event.Interface
	statusListener policystatus.Listener
	prGenerator    policyreport.GeneratorInterface

	rbLister       rbaclister.RoleBindingLister
	rbSynced       cache.InformerSynced
	crbLister      rbaclister.ClusterRoleBindingLister
	crbSynced      cache.InformerSynced
	nsLister       listerv1.NamespaceLister
	nsListerSynced cache.InformerSynced

	log           logr.Logger
	configHandler config.Interface
	resCache      resourcecache.ResourceCache
}

// NewValidateAuditHandler returns a new instance of audit policy handler
func NewValidateAuditHandler(pCache policycache.Interface,
	eventGen event.Interface,
	statusListener policystatus.Listener,
	prGenerator policyreport.GeneratorInterface,
	rbInformer rbacinformer.RoleBindingInformer,
	crbInformer rbacinformer.ClusterRoleBindingInformer,
	namespaces informers.NamespaceInformer,
	log logr.Logger,
	dynamicConfig config.Interface,
	resCache resourcecache.ResourceCache,
	client *client.Client) AuditHandler {

	return &auditHandler{
		pCache:         pCache,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		eventGen:       eventGen,
		statusListener: statusListener,
		rbLister:       rbInformer.Lister(),
		rbSynced:       rbInformer.Informer().HasSynced,
		crbLister:      crbInformer.Lister(),
		crbSynced:      crbInformer.Informer().HasSynced,
		nsLister:       namespaces.Lister(),
		nsListerSynced: namespaces.Informer().HasSynced,
		log:            log,
		prGenerator:    prGenerator,
		configHandler:  dynamicConfig,
		resCache:       resCache,
		client:         client,
	}
}

func (h *auditHandler) Add(request *v1beta1.AdmissionRequest) {
	h.log.V(4).Info("admission request added", "uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	h.queue.Add(request)
}

func (h *auditHandler) Run(workers int, stopCh <-chan struct{}) {
	h.log.V(4).Info("starting")

	defer func() {
		utilruntime.HandleCrash()
		h.log.V(4).Info("shutting down")
	}()

	if !cache.WaitForCacheSync(stopCh, h.rbSynced, h.crbSynced) {
		logger.Info("failed to sync informer cache")
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

	request, ok := obj.(*v1beta1.AdmissionRequest)
	if !ok {
		h.queue.Forget(obj)
		logger.Info("incorrect type: expecting type 'AdmissionRequest'", "object", obj)
		return true
	}

	err := h.process(request)
	h.handleErr(err, obj, request)

	return true
}

func (h *auditHandler) process(request *v1beta1.AdmissionRequest) error {
	var roles, clusterRoles []string
	var err error

	logger := h.log.WithName("process")
	policies := h.pCache.Get(policycache.ValidateAudit, nil)
	// Get namespace policies from the cache for the requested resource namespace
	nsPolicies := h.pCache.Get(policycache.ValidateAudit, &request.Namespace)
	policies = append(policies, nsPolicies...)
	// getRoleRef only if policy has roles/clusterroles defined
	if containRBACInfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(h.rbLister, h.crbLister, request, h.configHandler)
		if err != nil {
			logger.Error(err, "failed to get RBAC information for request")
		}
	}

	userRequestInfo := v1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}

	// build context
	ctx := enginectx.NewContext()
	err = ctx.AddRequest(request)
	if err != nil {
		return errors.Wrap(err, "failed to load incoming request in context")
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		return errors.Wrap(err, "failed to load userInfo in context")
	}
	err = ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		return errors.Wrap(err, "failed to load service account in context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	HandleValidation(request, policies, nil, ctx, userRequestInfo, h.statusListener, h.eventGen, h.prGenerator, logger, h.configHandler, h.resCache, h.client, namespaceLabels)
	return nil
}

func (h *auditHandler) handleErr(err error, key interface{}, request *v1beta1.AdmissionRequest) {
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
