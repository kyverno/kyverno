package internal

import (
	"context"

	extcertmanager "github.com/kyverno/pkg/certmanager"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// certControllerAdapter wraps a controller-runtime based cert reconciler
// to satisfy the controllers.Controller interface.
type certControllerAdapter struct {
	restConfig    *rest.Config
	certRenewer   tls.CertRenewer
	caSecretName  string
	tlsSecretName string
	namespace     string
}

func NewCertManagerController(
	restConfig *rest.Config,
	certRenewer tls.CertRenewer,
	caSecretName string,
	tlsSecretName string,
	namespace string,
) *certControllerAdapter {
	return &certControllerAdapter{
		restConfig:    restConfig,
		certRenewer:   certRenewer,
		caSecretName:  caSecretName,
		tlsSecretName: tlsSecretName,
		namespace:     namespace,
	}
}

func (a *certControllerAdapter) Run(ctx context.Context, _ int) {
	logger := logging.WithName(extcertmanager.ControllerName)
	// Perform initial certificate renewal so secrets exist before the manager starts.
	if err := a.certRenewer.RenewCA(ctx); err != nil {
		logger.Error(err, "initial CA renewal failed")
	}
	if err := a.certRenewer.RenewTLS(ctx); err != nil {
		logger.Error(err, "initial TLS renewal failed")
	}
	scheme := kruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	mgr, err := ctrl.NewManager(a.restConfig, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				a.namespace: {},
			},
		},
		LeaderElection:         false, // already in leader-elected context
		Metrics:                server.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		logger.Error(err, "failed to create manager for cert controller")
		return
	}
	reconciler := &certReconciler{
		certRenewer:   a.certRenewer,
		caSecretName:  a.caSecretName,
		tlsSecretName: a.tlsSecretName,
		namespace:     a.namespace,
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		Named(extcertmanager.ControllerName).
		For(&corev1.Secret{}).
		Complete(reconciler); err != nil {
		logger.Error(err, "failed to setup cert controller with manager")
		return
	}
	if err := mgr.Start(ctx); err != nil {
		logger.Error(err, "cert controller manager exited with error")
	}
}

// certReconciler reconciles Secret objects to keep CA and TLS certificates
// renewed. It returns RequeueAfter to provide periodic renewal equivalent
// to the old ticker-based approach.
type certReconciler struct {
	certRenewer   tls.CertRenewer
	caSecretName  string
	tlsSecretName string
	namespace     string
}

func (r *certReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if req.Namespace != r.namespace {
		return reconcile.Result{}, nil
	}
	if req.Name != r.caSecretName && req.Name != r.tlsSecretName {
		return reconcile.Result{}, nil
	}
	if err := r.certRenewer.RenewCA(ctx); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.certRenewer.RenewTLS(ctx); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: tls.CertRenewalInterval}, nil
}

// Ensure certReconciler implements reconcile.Reconciler.
var _ reconcile.Reconciler = &certReconciler{}

// Ensure certControllerAdapter satisfies the expected interface.
var _ interface{ Run(context.Context, int) } = &certControllerAdapter{}
