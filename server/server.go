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

	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreTypes "k8s.io/kubernetes/pkg/apis/core"
)

// WebhookServer is a struct that describes
// TLS server with mutation webhook
type WebhookServer struct {
	server http.Server
	logger *log.Logger
}

type patchOperations struct {
	patches []patchOperation
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/mutate" {
		admissionReview := ws.parseAdmissionReview(r, w)
		if admissionReview == nil {
			return
		}

		admissionResponse := ws.mutate(admissionReview)
		if admissionResponse != nil {
			admissionReview.Response = admissionResponse
			if admissionReview.Request != nil {
				admissionReview.Response.UID = admissionReview.Request.UID
			}
		}

		responseJson, err := json.Marshal(admissionReview)
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
			return
		}

		ws.logger.Printf("!!! Writing success !!! Response body: %v", string(responseJson))
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

func (ws *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	request := ar.Request

	ws.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	if admissionRequired(request) {
		var configMap coreTypes.ConfigMap
		if err := json.Unmarshal(request.Object.Raw, &configMap); err != nil {
			ws.logger.Printf("Could not unmarshal raw object: %v", err)
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		/*patch := patchOperation{
			Path: "/labels",
			Op:   "add",
			Value: map[string]string{
				"is-mutated": "true",
			},
		}*/
		patch := `[ {"op":"add","path":"/metadata/labels","value":{"is-mutated":"true"}} ]`

		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Patch:   []byte(patch),
			PatchType: func() *v1beta1.PatchType {
				pt := v1beta1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	} else {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
}

func admissionRequired(request *v1beta1.AdmissionRequest) bool {
	return request.Kind.Kind == "ConfigMap"
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

// NewWebhookServer creates new instance of WebhookServer and configures it
func NewWebhookServer(certFile string, keyFile string, logger *log.Logger) *WebhookServer {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}

	var config tls.Config
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		logger.Fatal("Unable to load certificate and key: ", err)
	}
	config.Certificates = []tls.Certificate{pair}

	mux := http.NewServeMux()

	ws := &WebhookServer{
		server: http.Server{
			Addr:         ":443", // Listen on port for HTTPS requests
			TLSConfig:    &config,
			Handler:      mux,
			ErrorLog:     logger,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		logger: logger,
	}

	mux.HandleFunc("/mutate", ws.serve)

	return ws
}
