package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	admissionv1 "k8s.io/api/admission/v1"
)

// maxAdmissionRequestBytes bounds the AdmissionReview body read from the network.
// The API server caps request bodies at 3MiB, and an AdmissionReview can carry both
// the object and its old version, so 7MiB matches controller-runtime's webhook limit
// while preventing a caller that reaches the service directly from exhausting memory.
const maxAdmissionRequestBytes int64 = 7 * 1024 * 1024

func (inner AdmissionHandler) WithAdmission(logger logr.Logger) HttpHandler {
	return inner.withAdmission(logger).WithMetrics(logger).WithTrace("ADMISSION")
}

func (inner AdmissionHandler) withAdmission(logger logr.Logger) HttpHandler {
	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		if request.Body == nil {
			HttpError(request.Context(), writer, request, logger, errors.New("empty body"), http.StatusBadRequest)
			return
		}
		defer request.Body.Close()
		contentType := request.Header.Get("Content-Type")
		if contentType != "application/json" {
			HttpError(request.Context(), writer, request, logger, errors.New("invalid Content-Type"), http.StatusUnsupportedMediaType)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxAdmissionRequestBytes))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				HttpError(request.Context(), writer, request, logger, err, http.StatusRequestEntityTooLarge)
				return
			}
			HttpError(request.Context(), writer, request, logger, err, http.StatusBadRequest)
			return
		}
		var admissionReview admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &admissionReview); err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusExpectationFailed)
			return
		}
		if admissionReview.Request == nil {
			HttpError(request.Context(), writer, request, logger, errors.New("AdmissionReview request is nil"), http.StatusBadRequest)
			return
		}
		logger := logger.WithValues(
			"gvk", admissionReview.Request.Kind,
			"gvr", admissionReview.Request.Resource.String(),
			"namespace", admissionReview.Request.Namespace,
			"name", admissionReview.Request.Name,
			"operation", admissionReview.Request.Operation,
			"uid", admissionReview.Request.UID,
			"user", admissionReview.Request.UserInfo,
		)

		params := httprouter.ParamsFromContext(request.Context())
		admissionRequest := AdmissionRequest{
			AdmissionRequest: *admissionReview.Request,
			URLParams:        params.ByName("policy"),
		}
		// One CEL cost budget shared by every policy this admission request evaluates
		// (Handle for every CEL family reads it via program.go's ContextEval wrapper,
		// pkg/cel/requestbudget); ctx propagates it down through request.Context().
		ctx, _ := requestbudget.Ensure(request.Context())
		admissionResponse := inner(ctx, logger, admissionRequest, startTime)
		admissionReview.Response = &admissionResponse
		responseJSON, err := json.Marshal(admissionReview)
		if err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := writer.Write(responseJSON); err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusInternalServerError)
			return
		}
	}
}
