package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AdmissionRequest struct {
	// AdmissionRequest is the original admission request.
	admissionv1.AdmissionRequest

	// Roles is a list of possible role send the request.
	Roles []string

	// ClusterRoles is a list of possible clusterRoles send the request.
	ClusterRoles []string

	// GroupVersionKind is the top level GVK.
	GroupVersionKind schema.GroupVersionKind
}

type AdmissionResponse = admissionv1.AdmissionResponse

type (
	AdmissionHandler func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse
	HttpHandler      func(http.ResponseWriter, *http.Request)
)

func FromAdmissionFunc(name string, h AdmissionHandler) AdmissionHandler {
	return h.WithTrace(name)
}

func (h HttpHandler) ToHandlerFunc(operation string) http.HandlerFunc {
	handler := otelhttp.NewHandler(http.HandlerFunc(h), operation)
	return handler.ServeHTTP
}
