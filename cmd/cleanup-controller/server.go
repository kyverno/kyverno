package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/cleanup"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop(context.Context)
}

type server struct {
	server *http.Server
}

type (
	TlsProvider       = func() ([]byte, []byte, error)
	ValidationHandler = func(context.Context, logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
	CleanupHandler    = func(context.Context, logr.Logger, string, time.Time) error
)

type Probes interface {
	IsReady() bool
	IsLive() bool
}

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	tlsProvider TlsProvider,
	validationHandler ValidationHandler,
	cleanupHandler CleanupHandler,
	metricsConfig metrics.MetricsConfigManager,
	debugModeOpts webhooks.DebugModeOptions,
	probes Probes,
) Server {
	policyLogger := logging.WithName("cleanup-policy")
	cleanupLogger := logging.WithName("cleanup")
	cleanupHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		policy := r.URL.Query().Get("policy")
		logger := cleanupLogger.WithValues("policy", policy)
		err := cleanupHandler(r.Context(), logger, policy, time.Now())
		if err == nil {
			w.WriteHeader(http.StatusOK)
		} else {
			if apierrors.IsNotFound(err) {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}
	mux := httprouter.New()
	mux.HandlerFunc(
		"POST",
		config.CleanupValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", validationHandler).
			WithDump(debugModeOpts.DumpPayload, nil, nil, nil).
			WithSubResourceFilter().
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(policyLogger.WithName("validate")).
			ToHandlerFunc(),
	)
	mux.HandlerFunc(
		"GET",
		cleanup.CleanupServicePath,
		handlers.HttpHandler(cleanupHandlerFunc).
			WithMetrics(policyLogger).
			WithTrace("CLEANUP").
			ToHandlerFunc(),
	)
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(probes.IsLive))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(probes.IsReady))
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
			ErrorLog:          logging.StdLogger(logging.WithName("server"), ""),
		},
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		if err := s.server.ListenAndServeTLS("", ""); err != nil {
			logging.Error(err, "failed to start server")
		}
	}()
}

func (s *server) Stop(ctx context.Context) {
	err := s.server.Shutdown(ctx)
	if err != nil {
		err = s.server.Close()
		if err != nil {
			logging.Error(err, "failed to start server")
		}
	}
}
