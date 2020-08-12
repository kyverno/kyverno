package webhooks

import (
	"github.com/go-logr/logr"
	"github.com/minio/minio/cmd/logger"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/constant"
	enginectx "github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policycache"
	"github.com/nirmata/kyverno/pkg/policystatus"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/userinfo"
	"github.com/pkg/errors"
	"k8s.io/api/admission/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	workQueueName       = "validate-audit-handler"
	workQueueRetryLimit = 3
)

// Handler applies validate audit policies to the admission request
// the handler adds the request to the work queue and returns immediately
// the request is processed in background, with the exact same logic
// when process the admission request in the webhook
type AuditHandler interface {
	Add(request *v1beta1.AdmissionRequest)
	Run(workers int, stopCh <-chan struct{})
}

type auditHandler struct {
	client         *kyvernoclient.Clientset
	queue          workqueue.RateLimitingInterface
	pCache         policycache.Interface
	eventGen       event.Interface
	statusListener policystatus.Listener
	pvGenerator    policyviolation.GeneratorInterface

	rbLister  rbaclister.RoleBindingLister
	rbSynced  cache.InformerSynced
	crbLister rbaclister.ClusterRoleBindingLister
	crbSynced cache.InformerSynced

	log logr.Logger
	configHandler config.Interface
}

// NewValidateAuditHandler returns a new instance of audit policy handler
func NewValidateAuditHandler(pCache policycache.Interface,
	eventGen event.Interface,
	statusListener policystatus.Listener,
	pvGenerator policyviolation.GeneratorInterface,
	rbInformer rbacinformer.RoleBindingInformer,
	crbInformer rbacinformer.ClusterRoleBindingInformer,
	log logr.Logger,
	dynamicConfig config.Interface) AuditHandler {

	return &auditHandler{
		pCache:         pCache,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		eventGen:       eventGen,
		statusListener: statusListener,
		pvGenerator:    pvGenerator,
		rbLister:       rbInformer.Lister(),
		rbSynced:       rbInformer.Informer().HasSynced,
		crbLister:      crbInformer.Lister(),
		crbSynced:      crbInformer.Informer().HasSynced,
		log:            log,
		configHandler : dynamicConfig,
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
		go wait.Until(h.runWorker, constant.GenerateControllerResync, stopCh)
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
		return false
	}

	err := h.process(request)
	h.handleErr(err)

	return true
}

func (h *auditHandler) process(request *v1beta1.AdmissionRequest) error {
	var roles, clusterRoles []string
	var err error

	logger := h.log.WithName("process")
	policies := h.pCache.Get(policycache.ValidateAudit, nil)
	nsPolicies := h.pCache.Get(policycache.ValidateAudit, &request.Namespace)
	policies = append(policies, nsPolicies...)
	// getRoleRef only if policy has roles/clusterroles defined
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(h.rbLister, h.crbLister, request,h.configHandler)
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
	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		return errors.Wrap(err, "failed to load service account in context")
	}

	HandleValidation(request, policies, nil, ctx, userRequestInfo, h.statusListener, h.eventGen, h.pvGenerator, logger,h.configHandler)
	return nil
}

func (h *auditHandler) handleErr(err error) {

}
