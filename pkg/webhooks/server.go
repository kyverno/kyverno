package webhooks

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
)

type Handlers interface {
	// Mutate performs the mutation of policy resources
	Mutate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
	// Validate performs the validation check on policy resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
}

// WebhookServer contains configured TLS server with MutationWebhook.
type WebhookServer struct {
	server          *http.Server
	configuration   config.Configuration
	webhookRegister *webhookconfig.Register
	webhookMonitor  *webhookconfig.Monitor
	cleanUp         chan<- struct{}
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	policyHandlers Handlers,
	resourceHandlers Handlers,
	tlsPair func() ([]byte, []byte, error),
	configuration config.Configuration,
	webhookRegistrationClient *webhookconfig.Register,
	webhookMonitor *webhookconfig.Monitor,
	cleanUp chan<- struct{},
) (*WebhookServer, error) {
	if tlsPair == nil {
		return nil, errors.New("NewWebhookServer is not initialized properly")
	}
	ws := &WebhookServer{
		configuration:   configuration,
		webhookRegister: webhookRegistrationClient,
		webhookMonitor:  webhookMonitor,
		cleanUp:         cleanUp,
	}
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	verifyLogger := logger.WithName("verify")
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, ws.admissionHandler(resourceLogger.WithName("mutate"), true, resourceHandlers.Mutate))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, ws.admissionHandler(resourceLogger.WithName("validate"), true, resourceHandlers.Validate))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, ws.admissionHandler(policyLogger.WithName("mutate"), true, policyHandlers.Mutate))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, ws.admissionHandler(policyLogger.WithName("validate"), true, policyHandlers.Validate))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, ws.admissionHandler(verifyLogger.WithName("mutate"), false, handlers.Verify(ws.webhookMonitor)))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(ws.webhookRegister.Check))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(nil))
	ws.server = &http.Server{
		Addr: ":9443", // Listen on port for HTTPS requests
		TLSConfig: &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				certPem, keyPem, err := tlsPair()
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
	}
	return ws, nil
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	go func() {
		logger.V(3).Info("started serving requests", "addr", ws.server.Addr)
		if err := ws.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Error(err, "failed to listen to requests")
		}
	}()
	logger.Info("starting service")
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	// remove the static webhook configurations
	go ws.webhookRegister.Remove(ws.cleanUp)
	// shutdown http.Server with context timeout
	err := ws.server.Shutdown(ctx)
	if err != nil {
		// Error from closing listeners, or context timeout:
		logger.Error(err, "shutting down server")
		err = ws.server.Close()
		if err != nil {
			logger.Error(err, "server shut down failed")
		}
	}
}

func (ws *WebhookServer) admissionHandler(logger logr.Logger, filter bool, inner handlers.AdmissionHandler) http.HandlerFunc {
	if filter {
		inner = handlers.Filter(ws.configuration, inner)
	}
	return handlers.Monitor(ws.webhookMonitor, handlers.Admission(logger, inner))
}
