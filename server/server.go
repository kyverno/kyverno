package server

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"

    controller "github.com/nirmata/kube-policy/controller"
    webhooks "github.com/nirmata/kube-policy/webhooks"
    v1beta1 "k8s.io/api/admission/v1beta1"
)

// WebhookServer is a struct that describes
// TLS server with mutation webhook
type WebhookServer struct {
    server           http.Server
    logger           *log.Logger
    policyController *controller.PolicyController
    mutationWebhook  *webhooks.MutationWebhook
}

// NewWebhookServer creates new instance of WebhookServer and configures it
func NewWebhookServer(certFile string, keyFile string, controller *controller.PolicyController, logger *log.Logger) *WebhookServer {
    if logger == nil {
        logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
    }
    if controller == nil {
        logger.Fatal("Controller is not specified for webhook server")
    }

    var config tls.Config
    pair, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        logger.Fatal("Unable to load certificate and key: ", err)
    }
    config.Certificates = []tls.Certificate{pair}

    mw, err := webhooks.NewMutationWebhook(logger)
    if err != nil {
        logger.Fatal("Unable to create mutation webhook: ", err)
    }

    ws := &WebhookServer{
        logger:           logger,
        policyController: controller,
        mutationWebhook:  mw,
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/mutate", ws.serve)

    ws.server = http.Server{
        Addr:         ":443", // Listen on port for HTTPS requests
        TLSConfig:    &config,
        Handler:      mux,
        ErrorLog:     logger,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }

    return ws
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/mutate" {
        admissionReview := ws.parseAdmissionReview(r, w)
        if admissionReview == nil {
            return
        }

        var admissionResponse *v1beta1.AdmissionResponse
        if webhooks.AdmissionIsRequired(admissionReview.Request) {
            admissionResponse = ws.mutationWebhook.Mutate(admissionReview.Request, ws.policyController.GetPolicies())
        }

        if admissionResponse == nil {
            admissionResponse = &v1beta1.AdmissionResponse{
                Allowed: true,
            }
        }

        admissionReview.Response = admissionResponse
        admissionReview.Response.UID = admissionReview.Request.UID

        responseJson, err := json.Marshal(admissionReview)
        if err != nil {
            http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
            return
        }

        ws.logger.Printf("Response body\n:%v", string(responseJson))
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        if _, err := w.Write(responseJson); err != nil {
            http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
        }
    } else {
        http.Error(w, fmt.Sprintf("Unexpected method path: %v", r.URL.Path), http.StatusNotFound)
    }
}

// Answers to the http.ResponseWriter if request is not valid
func (ws *WebhookServer) parseAdmissionReview(request *http.Request, writer http.ResponseWriter) *v1beta1.AdmissionReview {
    var body []byte
    if request.Body != nil {
        if data, err := ioutil.ReadAll(request.Body); err == nil {
            body = data
        }
    }
    if len(body) == 0 {
        ws.logger.Print("Error: empty body")
        http.Error(writer, "empty body", http.StatusBadRequest)
        return nil
    }

    contentType := request.Header.Get("Content-Type")
    if contentType != "application/json" {
        ws.logger.Printf("Error: invalid Content-Type: %v", contentType)
        http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
        return nil
    }

    admissionReview := &v1beta1.AdmissionReview{}
    if err := json.Unmarshal(body, &admissionReview); err != nil {
        ws.logger.Printf("Error: Can't decode body as AdmissionReview: %v", err)
        http.Error(writer, "Can't decode body as AdmissionReview", http.StatusExpectationFailed)
        return nil
    } else {
        ws.logger.Printf("Request body:\n%v", string(body))
        return admissionReview
    }
}

// RunAsync runs TLS server in separate
// thread and returns control immediately
func (ws *WebhookServer) RunAsync() {
    go func(ws *WebhookServer) {
        err := ws.server.ListenAndServeTLS("", "")
        if err != nil {
            ws.logger.Fatal(err)
        }
    }(ws)
}

// Stop stops TLS server
func (ws *WebhookServer) Stop() {
    err := ws.server.Shutdown(context.Background())
    if err != nil {
        // Error from closing listeners, or context timeout:
        ws.logger.Printf("Server Shutdown error: %v", err)
        ws.server.Close()
    }
}
