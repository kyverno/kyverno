package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/cmd/cleanup/logger"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Stop(context.Context)
	// Cleanup returns the chanel used to wait for the server to clean up resources
	Cleanup() <-chan struct{}
}

type CleanupPolicyHandlers interface {
	// Validate performs the validation check on policy resources
	ValidateCleanupPolicy(logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
}

type ResourceHandlers interface {
	// Validate performs the validation check on kube resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse
}

type server struct {
	server      *http.Server
	runtime     runtimeutils.Runtime
	vwcClient   controllerutils.DeleteClient[*admissionregistrationv1.ValidatingWebhookConfiguration]
	leaseClient controllerutils.DeleteClient[*coordinationv1.Lease]
	cleanUp     chan struct{}
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	policyHandlers CleanupPolicyHandlers,
	resourceHandlers ResourceHandlers,
	configuration config.Configuration,
	metricsConfig *metrics.MetricsConfig,
	debugModeOpts DebugModeOptions,
	tlsProvider TlsProvider,
	vwcClient controllerutils.DeleteClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	leaseClient controllerutils.DeleteClient[*coordinationv1.Lease],
	runtime runtimeutils.Runtime,
) Server {
	mux := httprouter.New()
	// policyLogger := logger.Logger.WithName("cleanuppolicy")
	// mux.HandlerFunc(
	// 	"POST",
	// 	config.CleanupPolicyValidatingWebhookServicePath,
	// 	handlers.AdmissionHandler(policyHandlers.ValidateCleanupPolicy).
	// 		WithFilter(configuration).
	// 		WithDump(debugModeOpts.DumpPayload).
	// 		WithMetrics(metricsConfig).
	// 		WithAdmission(policyLogger.WithName("validate")),
	// )
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
			},
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			IdleTimeout:       5 * time.Minute,
			ErrorLog:          logging.StdLogger(logger.Logger.WithName("server"), ""),
		},
		vwcClient:   vwcClient,
		leaseClient: leaseClient,
		runtime:     runtime,
		cleanUp:     make(chan struct{}),
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		logger.Logger.V(3).Info("started serving requests", "addr", s.server.Addr)
		if err := s.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Logger.Error(err, "failed to listen to requests")
		}
	}()
	logger.Logger.Info("starting service")
}

func (s *server) Stop(ctx context.Context) {
	s.cleanup(ctx)
	err := s.server.Shutdown(ctx)
	if err != nil {
		logger.Logger.Error(err, "shutting down server")
		err = s.server.Close()
		if err != nil {
			logger.Logger.Error(err, "server shut down failed")
		}
	}
}

func (s *server) Cleanup() <-chan struct{} {
	return s.cleanUp
}

func (s *server) cleanup(ctx context.Context) {
	if s.runtime.IsGoingDown() {
		deleteLease := func(name string) {
			if err := s.leaseClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Logger.Error(err, "failed to clean up lease", "name", name)
			}
		}
		deleteVwc := func(name string) {
			if err := s.vwcClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Logger.Error(err, "failed to clean up validating webhook configuration", "name", name)
			}
		}
		deleteLease("kyvernopre-lock")
		deleteLease("kyverno-health")
		deleteVwc(config.ValidatingWebhookConfigurationName)
		deleteVwc(config.PolicyValidatingWebhookConfigurationName)
	}
	close(s.cleanUp)
}

func registerWebhookHandlers(
	logger logr.Logger,
	mux *httprouter.Router,
	basePath string,
	configuration config.Configuration,
	metricsConfig *metrics.MetricsConfig,
	handlerFunc func(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse,
	debugModeOpts DebugModeOptions,
) {
	mux.HandlerFunc(
		"POST",
		basePath,
		handlers.AdmissionHandler(func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "all", startTime)
		}).
			WithFilter(configuration).
			WithProtection(toggle.ProtectManagedResources.Enabled()).
			WithDump(debugModeOpts.DumpPayload).
			WithMetrics(metricsConfig).
			WithAdmission(logger),
	)
	mux.HandlerFunc(
		"POST",
		basePath+"/fail",
		handlers.AdmissionHandler(func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "fail", startTime)
		}).
			WithFilter(configuration).
			WithProtection(toggle.ProtectManagedResources.Enabled()).
			WithDump(debugModeOpts.DumpPayload).
			WithMetrics(metricsConfig).
			WithAdmission(logger),
	)
	mux.HandlerFunc(
		"POST",
		basePath+"/ignore",
		handlers.AdmissionHandler(func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "ignore", startTime)
		}).
			WithFilter(configuration).
			WithProtection(toggle.ProtectManagedResources.Enabled()).
			WithDump(debugModeOpts.DumpPayload).
			WithMetrics(metricsConfig).
			WithAdmission(logger),
	)
}
