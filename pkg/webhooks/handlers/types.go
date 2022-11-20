package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
)

type (
	AdmissionHandler func(context.Context, logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
	HttpHandler      func(http.ResponseWriter, *http.Request)
)
