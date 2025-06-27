package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
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

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run()
	// Stop TLS server and returns control after the server is shut down
	Stop()
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
	celExceptionHandlers CELExceptionHandlers,
	globalContextHandlers GlobalContextHandlers,
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
	webhookServerPort int32,
) Server {
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	exceptionLogger := logger.WithName("exception")
	celExceptionLogger := logger.WithName("cel-exception")
	globalContextLogger := logger.WithName("globalcontext")
	verifyLogger := logger.WithName("verify")
	vpolLogger := logger.WithName("vpol")
	ivpolLogger := logger.WithName("ivpol")
	mpolLogger := logger.WithName("mpol")
	mux.HandlerFunc(
		"POST",
		"/mpol/*policies",
		handlerFunc("MUTATE", resourceHandlers.MutatingPolicies, "").
			WithFilter(configuration).
			WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
			WithDump(debugModeOpts.DumpPayload).
			WithTopLevelGVK(discovery).
			WithRoles(rbLister, crbLister).
			WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(mpolLogger.WithName("mutate")).
			ToHandlerFunc("MPOL"),
	)
	// new vpol and ivpol handlers
	mux.HandlerFunc(
		"POST",
		"/vpol/*policies",
		handlerFunc("VALIDATE", resourceHandlers.ValidatingPolicies, "").
			WithFilter(configuration).
			WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
			WithDump(debugModeOpts.DumpPayload).
			WithTopLevelGVK(discovery).
			WithRoles(rbLister, crbLister).
			WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(vpolLogger.WithName("validate")).
			ToHandlerFunc("VPOL"),
	)
	mux.HandlerFunc(
		"POST",
		"/ivpol/validate/*policies",
		handlerFunc("IVPOL-VALIDATE", resourceHandlers.ImageVerificationPolicies, "").
			WithFilter(configuration).
			WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
			WithDump(debugModeOpts.DumpPayload).
			WithTopLevelGVK(discovery).
			WithRoles(rbLister, crbLister).
			WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(ivpolLogger.WithName("validate")).
			ToHandlerFunc("IVPOL"),
	)
	mux.HandlerFunc(
		"POST",
		"/ivpol/mutate/*policies",
		handlerFunc("IVPOL-MUTATE", resourceHandlers.ImageVerificationPoliciesMutation, "").
			WithFilter(configuration).
			WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
			WithDump(debugModeOpts.DumpPayload).
			WithTopLevelGVK(discovery).
			WithRoles(rbLister, crbLister).
			WithOperationFilter(admissionv1.Create, admissionv1.Update, admissionv1.Connect).
			WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookMutating).
			WithAdmission(resourceLogger.WithName("mutate")).
			ToHandlerFunc("IVPOL"),
	)
	mux.HandlerFunc(
		"POST",
		"/gpol/*policies",
		handlerFunc("GENERATE", resourceHandlers.GeneratingPolicies, "").
			WithFilter(configuration).
			WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
			WithDump(debugModeOpts.DumpPayload).
			WithTopLevelGVK(discovery).
			WithRoles(rbLister, crbLister).
			WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(resourceLogger.WithName("generate")).
			ToHandlerFunc("GPOL"),
	)
	registerWebhookHandlersWithAll(
		mux,
		"MUTATE",
		config.MutatingWebhookServicePath,
		resourceHandlers.Mutation,
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
	registerWebhookHandlersWithAll(
		mux,
		"IVPOL-MUTATE",
		config.PolicyServicePath+config.ImageValidatingPolicyServicePath+config.MutatingWebhookServicePath,
		resourceHandlers.ImageVerificationPoliciesMutation,
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
	registerWebhookHandlersWithAll(
		mux,
		"VALIDATE",
		config.ValidatingWebhookServicePath,
		resourceHandlers.Validation,
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
	registerWebhookHandlers(
		mux,
		"VPOL",
		config.PolicyServicePath+config.ValidatingPolicyServicePath+config.ValidatingWebhookServicePath,
		resourceHandlers.ValidatingPolicies,
		func(handler handlers.AdmissionHandler) handlers.HttpHandler {
			return handler.
				WithFilter(configuration).
				WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
				WithDump(debugModeOpts.DumpPayload).
				WithTopLevelGVK(discovery).
				WithRoles(rbLister, crbLister).
				WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
				WithAdmission(vpolLogger.WithName("validate"))
		},
	)
	registerWebhookHandlers(
		mux,
		"IVPOL-VALIDATE",
		config.PolicyServicePath+config.ImageValidatingPolicyServicePath+config.ValidatingWebhookServicePath,
		resourceHandlers.ImageVerificationPolicies,
		func(handler handlers.AdmissionHandler) handlers.HttpHandler {
			return handler.
				WithFilter(configuration).
				WithProtection(toggle.FromContext(ctx).ProtectManagedResources()).
				WithDump(debugModeOpts.DumpPayload).
				WithTopLevelGVK(discovery).
				WithRoles(rbLister, crbLister).
				WithMetrics(resourceLogger, metricsConfig.Config(), metrics.WebhookValidating).
				WithAdmission(ivpolLogger.WithName("validate"))
		},
	)
	mux.HandlerFunc(
		"POST",
		config.PolicyMutatingWebhookServicePath,
		handlerFunc("MUTATE", policyHandlers.Mutation, "").
			WithDump(debugModeOpts.DumpPayload).
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookMutating).
			WithAdmission(policyLogger.WithName("mutate")).
			ToHandlerFunc("MUTATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.PolicyValidatingWebhookServicePath,
		handlerFunc("VALIDATE", policyHandlers.Validation, "").
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(policyLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.ExceptionValidatingWebhookServicePath,
		handlerFunc("VALIDATE", exceptionHandlers.Validation, "").
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(exceptionLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(exceptionLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.CELExceptionValidatingWebhookServicePath,
		handlerFunc("VALIDATE", celExceptionHandlers.Validation, "").
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(celExceptionLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(celExceptionLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.GlobalContextValidatingWebhookServicePath,
		handlerFunc("VALIDATE", globalContextHandlers.Validation, "").
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(globalContextLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(globalContextLogger.WithName("validate")).
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
			Addr: fmt.Sprintf(":%d", webhookServerPort),
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

func (s *server) Run() {
	go func() {
		if err := s.server.ListenAndServeTLS("", ""); err != nil {
			logging.Error(err, "failed to start server")
		}
	}()
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
			} else if err == nil {
				logger.V(2).Info("successfully deleted leases", "label", kyverno.LabelWebhookManagedBy)
			}
		}
		deleteVwc := func() {
			if err := s.vwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up validating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
			} else if err == nil {
				logger.V(2).Info("successfully deleted validating webhook configurations", "label", kyverno.LabelWebhookManagedBy)
			}
		}
		deleteMwc := func() {
			if err := s.mwcClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: kyverno.LabelWebhookManagedBy,
			}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up mutating webhook configuration", "label", kyverno.LabelWebhookManagedBy)
			} else if err == nil {
				logger.V(2).Info("successfully deleted mutating webhook configurations", "label", kyverno.LabelWebhookManagedBy)
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
	handler Handler,
	builder func(handler handlers.AdmissionHandler) handlers.HttpHandler,
) {
	ignore := handlerFunc(name, handler, "ignore")
	fail := handlerFunc(name, handler, "fail")
	mux.HandlerFunc("POST", basePath+"/ignore", builder(ignore).ToHandlerFunc(name))
	mux.HandlerFunc("POST", basePath+"/fail", builder(fail).ToHandlerFunc(name))
	mux.HandlerFunc("POST", basePath+"/ignore"+config.FineGrainedWebhookPath+"/*policy", builder(ignore).ToHandlerFunc(name))
	mux.HandlerFunc("POST", basePath+"/fail"+config.FineGrainedWebhookPath+"/*policy", builder(fail).ToHandlerFunc(name))
}

func registerWebhookHandlersWithAll(
	mux *httprouter.Router,
	name string,
	basePath string,
	handler Handler,
	builder func(handler handlers.AdmissionHandler) handlers.HttpHandler,
) {
	all := handlerFunc(name, handler, "all")
	mux.HandlerFunc("POST", basePath, builder(all).ToHandlerFunc(name))
	registerWebhookHandlers(mux, name, basePath, handler, builder)
}

func handlerFunc(name string, handler Handler, failurePolicy string) handlers.AdmissionHandler {
	return handlers.FromAdmissionFunc(
		name,
		func(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, startTime time.Time) admissionv1.AdmissionResponse {
			return handler.Execute(ctx, logger, request, failurePolicy, startTime)
		},
	)
}
