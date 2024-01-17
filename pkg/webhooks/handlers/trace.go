package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

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
				semconv.NetSockPeerAddrKey.String(tracing.StringValue(request.Host)),
				semconv.HTTPMethodKey.String(tracing.StringValue(request.Method)),
				semconv.HTTPURLKey.String(tracing.StringValue(request.RequestURI)),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
	}
}

func (inner AdmissionHandler) WithTrace(name string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		return tracing.Span1(
			ctx,
			"webhooks/handlers",
			fmt.Sprintf("%s %s %s", name, request.Operation, request.Kind),
			func(ctx context.Context, span trace.Span) AdmissionResponse {
				response := inner(ctx, logger, request, startTime)
				span.SetAttributes(
					tracing.ResponseUidKey.String(tracing.StringValue(string(response.UID))),
					tracing.ResponseAllowedKey.Bool(response.Allowed),
					tracing.ResponseWarningsKey.StringSlice(response.Warnings),
				)
				if response.Result != nil {
					span.SetAttributes(
						tracing.ResponseResultStatusKey.String(tracing.StringValue(response.Result.Status)),
						tracing.ResponseResultMessageKey.String(tracing.StringValue(response.Result.Message)),
						tracing.ResponseResultReasonKey.String(tracing.StringValue(string(response.Result.Reason))),
						tracing.ResponseResultCodeKey.Int(int(response.Result.Code)),
					)
				}
				if response.PatchType != nil {
					span.SetAttributes(
						tracing.ResponsePatchTypeKey.String(tracing.StringValue(string(*response.PatchType))),
					)
				}
				return response
			},
			trace.WithAttributes(
				tracing.RequestNameKey.String(tracing.StringValue(request.Name)),
				tracing.RequestNamespaceKey.String(tracing.StringValue(request.Namespace)),
				tracing.RequestUidKey.String(tracing.StringValue(string(request.UID))),
				tracing.RequestOperationKey.String(tracing.StringValue(string(request.Operation))),
				tracing.RequestDryRunKey.Bool(admissionutils.IsDryRun(request.AdmissionRequest)),
				tracing.RequestKindGroupKey.String(tracing.StringValue(request.Kind.Group)),
				tracing.RequestKindVersionKey.String(tracing.StringValue(request.Kind.Version)),
				tracing.RequestKindKindKey.String(tracing.StringValue(request.Kind.Kind)),
				tracing.RequestSubResourceKey.String(tracing.StringValue(request.SubResource)),
				tracing.RequestRequestKindGroupKey.String(tracing.StringValue(request.RequestKind.Group)),
				tracing.RequestRequestKindVersionKey.String(tracing.StringValue(request.RequestKind.Version)),
				tracing.RequestRequestKindKindKey.String(tracing.StringValue(request.RequestKind.Kind)),
				tracing.RequestRequestSubResourceKey.String(tracing.StringValue(request.RequestSubResource)),
				tracing.RequestResourceGroupKey.String(tracing.StringValue(request.Resource.Group)),
				tracing.RequestResourceVersionKey.String(tracing.StringValue(request.Resource.Version)),
				tracing.RequestResourceResourceKey.String(tracing.StringValue(request.Resource.Resource)),
				tracing.RequestRequestResourceGroupKey.String(tracing.StringValue(request.RequestResource.Group)),
				tracing.RequestRequestResourceVersionKey.String(tracing.StringValue(request.RequestResource.Version)),
				tracing.RequestRequestResourceResourceKey.String(tracing.StringValue(request.RequestResource.Resource)),
				tracing.RequestUserNameKey.String(tracing.StringValue(request.UserInfo.Username)),
				tracing.RequestUserUidKey.String(tracing.StringValue(request.UserInfo.UID)),
				tracing.RequestRolesKey.StringSlice(request.Roles),
				tracing.RequestClusterRolesKey.StringSlice(request.ClusterRoles),
				tracing.RequestGroupKey.String(request.GroupVersionKind.Group),
				tracing.RequestVersionKey.String(request.GroupVersionKind.Version),
				tracing.RequestKindKey.String(request.GroupVersionKind.Kind),
				tracing.RequestUserGroupsKey.StringSlice(request.UserInfo.Groups),
			),
		)
	}
}
