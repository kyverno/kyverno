package webhooks

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop(context.Context)
}

type Handlers interface {
	// Mutate performs the mutation of policy resources
	Mutate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
	// Validate performs the validation check on policy resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
}

type server struct {
	server          *http.Server
	webhookRegister *webhookconfig.Register
	cleanUp         chan<- struct{}
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	policyHandlers Handlers,
	resourceHandlers Handlers,
	tlsProvider TlsProvider,
	configuration config.Configuration,
	register *webhookconfig.Register,
	monitor *webhookconfig.Monitor,
	cleanUp chan<- struct{},
) Server {
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	verifyLogger := logger.WithName("verify")
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, admission(resourceLogger.WithName("mutate"), monitor, filter(configuration, resourceHandlers.Mutate)))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, admission(resourceLogger.WithName("validate"), monitor, filter(configuration, resourceHandlers.Validate)))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, admission(policyLogger.WithName("mutate"), monitor, filter(configuration, policyHandlers.Mutate)))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, admission(policyLogger.WithName("validate"), monitor, filter(configuration, policyHandlers.Validate)))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, admission(verifyLogger.WithName("mutate"), monitor, handlers.Verify(monitor)))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(register.Check))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(nil))
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
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		webhookRegister: register,
		cleanUp:         cleanUp,
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
}

func (s *server) Stop(ctx context.Context) {
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
	cleanupKyvernoResource := s.webhookRegister.ShouldCleanupKyvernoResource()

	var wg sync.WaitGroup
	wg.Add(2)
	go s.webhookRegister.Remove(cleanupKyvernoResource, &wg)
	go s.webhookRegister.ResetPolicyStatus(cleanupKyvernoResource, &wg)
	wg.Wait()
	close(s.cleanUp)
}

func filter(configuration config.Configuration, inner handlers.AdmissionHandler) handlers.AdmissionHandler {
	return handlers.Filter(configuration, inner)
}

func admission(logger logr.Logger, monitor *webhookconfig.Monitor, inner handlers.AdmissionHandler) http.HandlerFunc {
	return handlers.Monitor(monitor, handlers.Admission(logger, inner))
}
