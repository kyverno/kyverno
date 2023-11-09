package webhooks

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

// DebugModeOptions holds the options to configure debug mode
type DebugModeOptions struct {
	// DumpPayload is used to activate/deactivate debug mode.
	DumpPayload bool
}

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop()
}

type ExceptionHandlers interface {
	// Validate performs the validation check on exception resources
	Validate(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) admissionv1.AdmissionResponse
}

type PolicyHandlers interface {
	// Mutate performs the mutation of policy resources
	Mutate(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) admissionv1.AdmissionResponse
	// Validate performs the validation check on policy resources
	Validate(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) admissionv1.AdmissionResponse
}

type ResourceHandlers interface {
	// Mutate performs the mutation of kube resources
	Mutate(context.Context, logr.Logger, handlers.AdmissionRequest, string, time.Time) admissionv1.AdmissionResponse
	// Validate performs the validation check on kube resources
	Validate(context.Context, logr.Logger, handlers.AdmissionRequest, string, time.Time) admissionv1.AdmissionResponse
}

type server struct {
	server      *http.Server
	runtime     runtimeutils.Runtime
	mwcClient   controllerutils.DeleteCollectionClient
	vwcClient   controllerutils.DeleteCollectionClient
	leaseClient controllerutils.DeleteClient
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	ctx context.Context,
	policyHandlers PolicyHandlers,
	resourceHandlers ResourceHandlers,
	exceptionHandlers ExceptionHandlers,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	debugModeOpts DebugModeOptions,
	tlsProvider TlsProvider,
	mwcClient controllerutils.DeleteCollectionClient,
	vwcClient controllerutils.DeleteCollectionClient,
	leaseClient controllerutils.DeleteClient,
	runtime runtimeutils.Runtime,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
	discovery dclient.IDiscovery,
) Server {
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	exceptionLogger := logger.WithName("exception")
	verifyLogger := logger.WithName("verify")
	registerWebhookHandlers(
		mux,
		"MUTATE",
		config.MutatingWebhookServicePath,
		resourceHandlers.Mutate,
		func(handler handlers.AdmissionHandler) handlers.HttpHandler {
			return handler.
				WithFilter(configuration).
				WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
				WithDump(debugModeOpts.DumpPayload).
				WithTopLevelGVK(discovery).
				WithRoles(rbLister, crbLister).
				WithOperationFilter(admissionv1.Create, admissionv1.Update, admissionv1.Connect).
				WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookMutating).
				WithAdmission(resourceLogger.WithName("mutate"))
		},
	)
	registerWebhookHandlers(
		mux,
		"VALIDATE",
		config.ValidatingWebhookServicePath,
		resourceHandlers.Validate,
		func(handler handlers.AdmissionHandler) handlers.HttpHandler {
			return handler.
				WithFilter(configuration).
				WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
				WithDump(debugModeOpts.DumpPayload).
				WithTopLevelGVK(discovery).
				WithRoles(rbLister, crbLister).
				WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
				WithAdmission(resourceLogger.WithName("validate"))
		},
	)
	mux.HandlerFunc(
		"POST",
		config.PolicyMutatingWebhookServicePath,
		handlers.FromAdmissionFunc("MUTATE", policyHandlers.Mutate).
			WithDump(debugModeOpts.DumpPayload).
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookMutating).
			WithAdmission(policyLogger.WithName("mutate")).
			ToHandlerFunc("MUTATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.PolicyValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", policyHandlers.Validate).
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(policyLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.ExceptionValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", exceptionHandlers.Validate).
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(exceptionLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(exceptionLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.VerifyMutatingWebhookServicePath,
		handlers.FromAdmissionFunc("VERIFY", handlers.Verify).
			WithAdmission(verifyLogger.WithName("mutate")).
			ToHandlerFunc("VERIFY"),
	)
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(runtime.IsLive))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(runtime.IsReady))
	return &server{
		server: &http.Server{
			Addr: ":9443",
			TLSConfig: &tls.Config{
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					certPem, keyPem, err := tlsProvider()
					if err != nil {
						return nil, err
					}
					pair, err := tls.X509KeyPair(certPem, keyPem)
					if err != nil {
						return nil, err
					}
					return &pair, nil
				},
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					// AEADs w/ ECDHE
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				},
			},
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			IdleTimeout:       5 * time.Minute,
			ErrorLog:          logging.StdLogger(logger.WithName("server"), ""),
		},
		mwcClient:   mwcClient,
		vwcClient:   vwcClient,
		leaseClient: leaseClient,
		runtime:     runtime,
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		logger.V(3).Info("started serving requests", "addr", s.server.Addr)
		if err := s.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Error(err, "failed to listen to requests")
		}
	}()
	logger.Info("starting service")

	<-stopCh
	s.Stop()
}

func (s *server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.cleanup(ctx)
	err := s.server.Shutdown(ctx)
	if err != nil {
		logger.Error(err, "shutting down server")
		err = s.server.Close()
		if err != nil {
			logger.Error(err, "server shut down failed")
		}
	}
}

func (s *server) cleanup(ctx context.Context) {
	if s.runtime.IsGoingDown() {
		deleteLease := func(name string) {
			if err := s.leaseClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up lease", "name", name)
			}
		}
		deleteVwc := func() {
			if err := s.vwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up validating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
			}
		}
		deleteMwc := func() {
			if err := s.mwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up mutating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
			}
		}
		deleteLease("kyvernopre-lock")
		deleteLease("kyverno-health")
		deleteVwc()
		deleteMwc()
	}
}

func registerWebhookHandlers(
	mux *httprouter.Router,
	name string,
	basePath string,
	handlerFunc func(context.Context, logr.Logger, handlers.AdmissionRequest, string, time.Time) admissionv1.AdmissionResponse,
	builder func(handler handlers.AdmissionHandler) handlers.HttpHandler,
) {
	all := handlers.FromAdmissionFunc(
		name,
		func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, startTime time.Time) admissionv1.AdmissionResponse {
			return handlerFunc(ctx, logger, request, "all", startTime)
		},
	)
	ignore := handlers.FromAdmissionFunc(
		name,
		func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, startTime time.Time) admissionv1.AdmissionResponse {
			return handlerFunc(ctx, logger, request, "ignore", startTime)
		},
	)
	fail := handlers.FromAdmissionFunc(
		name,
		func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, startTime time.Time) admissionv1.AdmissionResponse {
			return handlerFunc(ctx, logger, request, "fail", startTime)
		},
	)
	mux.HandlerFunc("POST", basePath, builder(all).ToHandlerFunc(name))
	mux.HandlerFunc("POST", basePath+"/ignore", builder(ignore).ToHandlerFunc(name))
	mux.HandlerFunc("POST", basePath+"/fail", builder(fail).ToHandlerFunc(name))
}
