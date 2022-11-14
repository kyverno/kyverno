package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
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

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	tlsProvider TlsProvider,
) Server {
	mux := httprouter.New()
	// mux.HandlerFunc(
	// 	"POST",
	// 	config.PolicyMutatingWebhookServicePath,
	// 	handlers.AdmissionHandler(policyHandlers.Mutate).
	// 		WithFilter(configuration).
	// 		WithDump(debugModeOpts.DumpPayload).
	// 		WithMetrics(metricsConfig).
	// 		WithAdmission(policyLogger.WithName("mutate")),
	// )
	// mux.HandlerFunc(
	// 	"POST",
	// 	config.PolicyValidatingWebhookServicePath,
	// 	handlers.AdmissionHandler(policyHandlers.Validate).
	// 		WithFilter(configuration).
	// 		WithDump(debugModeOpts.DumpPayload).
	// 		WithMetrics(metricsConfig).
	// 		WithAdmission(policyLogger.WithName("validate")),
	// )
	// mux.HandlerFunc(
	// 	"POST",
	// 	config.VerifyMutatingWebhookServicePath,
	// 	handlers.Verify().
	// 		WithAdmission(verifyLogger.WithName("mutate")),
	// )
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
			// ErrorLog:          logging.StdLogger(logger.WithName("server"), ""),
		},
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		s.server.ListenAndServeTLS("", "")
	}()
}

func (s *server) Stop(ctx context.Context) {
	err := s.server.Shutdown(ctx)
	if err != nil {
		err = s.server.Close()
		if err != nil {
		}
	}
}
