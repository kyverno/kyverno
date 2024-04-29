package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func handleRequest(logger logr.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Body == nil {
			HttpError(request.Context(), writer, request, logger, errors.New("empty body"), http.StatusBadRequest)
			return
		}
		defer request.Body.Close()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusBadRequest)
			return
		}
		contentType := request.Header.Get("Content-Type")
		if contentType != "application/json" {
			HttpError(request.Context(), writer, request, logger, errors.New("invalid Content-Type"), http.StatusUnsupportedMediaType)
			return
		}
		var conversionReview v1.ConversionReview
		if err := json.Unmarshal(body, &conversionReview); err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusExpectationFailed)
			return
		}

		// perform the conversion
		conversionReview.Response = Convert(logger, conversionReview.Request)
		conversionReview.Response.UID = conversionReview.Request.UID

		// reset the request, it is not needed in a response.
		conversionReview.Request = &v1.ConversionRequest{}

		responseJSON, err := json.Marshal(conversionReview)
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

func HttpError(ctx context.Context, writer http.ResponseWriter, request *http.Request, logger logr.Logger, err error, code int) {
	logger.Error(err, "an error has occurred", "url", request.URL.String())
	tracing.SetHttpStatus(ctx, err, code)
	http.Error(writer, err.Error(), code)
}
