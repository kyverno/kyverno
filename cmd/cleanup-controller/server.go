package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

// ValidatingWebhookServicePath is the path for validation webhook
const ValidatingWebhookServicePath = "/validate"

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop(context.Context)
}

type CleanupPolicyHandlers interface {
	// Validate performs the validation check on policy resources
	Validate(context.Context, logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
}

type server struct {
	server *http.Server
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	policyHandlers CleanupPolicyHandlers,
	tlsProvider TlsProvider,
) Server {
	policyLogger := logging.WithName("cleanup-policy")
	mux := httprouter.New()
	mux.HandlerFunc(
		"POST",
		ValidatingWebhookServicePath,
		handlers.FromAdmissionFunc("VALIDATE", policyHandlers.Validate).
			WithSubResourceFilter().
			WithAdmission(policyLogger.WithName("validate")).
			ToHandlerFunc(),
	)
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
