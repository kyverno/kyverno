package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
)

const limit = 256

func (inner HttpHandler) WithTrace(name string) HttpHandler {
	return func(writer http.ResponseWriter, request *http.Request) {
		tracing.Span(
			request.Context(),
			"webhooks/handlers",
			fmt.Sprintf("%s %s %s", name, request.Method, request.URL.Path),
			func(ctx context.Context, span trace.Span) {
				inner(writer, request.WithContext(ctx))
			},
			trace.WithAttributes(
				semconv.HTTPRequestContentLengthKey.Int64(request.ContentLength),
				semconv.HTTPHostKey.String(request.Host),
				semconv.HTTPMethodKey.String(request.Method),
				semconv.HTTPURLKey.String(request.RequestURI),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
	}
}

func (inner AdmissionHandler) WithTrace(name string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		return tracing.Span1(
			ctx,
			"webhooks/handlers",
			fmt.Sprintf("%s %s %s", name, request.Operation, request.Kind),
			func(ctx context.Context, span trace.Span) *admissionv1.AdmissionResponse {
				response := inner(ctx, logger, request, startTime)
				if response != nil {
					span.SetAttributes(
						tracing.ResponseUidKey.String(string(response.UID)),
						tracing.ResponseAllowedKey.Bool(response.Allowed),
						tracing.ResponseWarningsKey.StringSlice(response.Warnings),
					)
					if response.Result != nil {
						message := response.Result.Message
						if len(message) > limit {
							message = message[:limit-3]
							message += "..."
						}
						span.SetAttributes(
							tracing.ResponseResultStatusKey.String(response.Result.Status),
							tracing.ResponseResultMessageKey.String(message),
							tracing.ResponseResultReasonKey.String(string(response.Result.Reason)),
							tracing.ResponseResultCodeKey.Int(int(response.Result.Code)),
						)
					}
					if response.PatchType != nil {
						span.SetAttributes(
							tracing.ResponsePatchTypeKey.String(string(*response.PatchType)),
						)
					}
				}
				return response
			},
			trace.WithAttributes(
				tracing.RequestNameKey.String(request.Name),
				tracing.RequestNamespaceKey.String(request.Namespace),
				tracing.RequestUidKey.String(string(request.UID)),
				tracing.RequestOperationKey.String(string(request.Operation)),
				tracing.RequestDryRunKey.Bool(request.DryRun != nil && *request.DryRun),
				tracing.RequestKindGroupKey.String(request.Kind.Group),
				tracing.RequestKindVersionKey.String(request.Kind.Version),
				tracing.RequestKindKindKey.String(request.Kind.Kind),
				tracing.RequestSubResourceKey.String(request.SubResource),
				tracing.RequestRequestKindGroupKey.String(request.RequestKind.Group),
				tracing.RequestRequestKindVersionKey.String(request.RequestKind.Version),
				tracing.RequestRequestKindKindKey.String(request.RequestKind.Kind),
				tracing.RequestRequestSubResourceKey.String(request.RequestSubResource),
				tracing.RequestResourceGroupKey.String(request.Resource.Group),
				tracing.RequestResourceVersionKey.String(request.Resource.Version),
				tracing.RequestResourceResourceKey.String(request.Resource.Resource),
				tracing.RequestRequestResourceGroupKey.String(request.RequestResource.Group),
				tracing.RequestRequestResourceVersionKey.String(request.RequestResource.Version),
				tracing.RequestRequestResourceResourceKey.String(request.RequestResource.Resource),
				tracing.RequestUserNameKey.String(request.UserInfo.Username),
				tracing.RequestUserUidKey.String(request.UserInfo.UID),
				tracing.RequestUserGroupsKey.StringSlice(request.UserInfo.Groups),
			),
		)
	}
}
