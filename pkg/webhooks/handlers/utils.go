package handlers

import (
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
)

func httpError(writer http.ResponseWriter, request *http.Request, logger logr.Logger, err error, code int) {
	logger.Error(err, "an error has occurred", "url", request.URL.String())
	tracing.SetHttpStatus(request.Context(), err, code)
	http.Error(writer, err.Error(), code)
}
