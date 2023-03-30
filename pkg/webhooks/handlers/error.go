package handlers

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
)

func HttpError(ctx context.Context, writer http.ResponseWriter, request *http.Request, logger logr.Logger, err error, code int) {
	logger.Error(err, "an error has occurred", "url", request.URL.String())
	tracing.SetHttpStatus(ctx, err, code)
	http.Error(writer, err.Error(), code)
}
