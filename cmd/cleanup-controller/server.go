package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
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
	TlsProvider            = func() ([]byte, []byte, error)
	ValidationHandler      = func(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) handlers.AdmissionResponse
	LabelValidationHandler = func(context.Context, logr.Logger, handlers.AdmissionRequest, time.Time) handlers.AdmissionResponse
	CleanupHandler         = func(context.Context, logr.Logger, string, time.Time, config.Configuration) error
)

type Probes interface {
	IsReady(context.Context) bool
	IsLive(context.Context) bool
}

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	tlsProvider TlsProvider,
	validationHandler ValidationHandler,
	labelValidationHandler LabelValidationHandler,
	metricsConfig metrics.MetricsConfigManager,
	debugModeOpts webhooks.DebugModeOptions,
	probes Probes,
	cfg config.Configuration,
) Server {
	policyLogger := logging.WithName("cleanup-policy")
	labelLogger := logging.WithName("ttl-label")
	mux := httprouter.New()
	mux.HandlerFunc(
		"POST",
		config.CleanupValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", validationHandler).
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(policyLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(policyLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
	)
	mux.HandlerFunc(
		"POST",
		config.TtlValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", labelValidationHandler).
			WithDump(debugModeOpts.DumpPayload).
			WithSubResourceFilter().
			WithMetrics(labelLogger, metricsConfig.Config(), metrics.WebhookValidating).
			WithAdmission(labelLogger.WithName("validate")).
			ToHandlerFunc("VALIDATE"),
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
