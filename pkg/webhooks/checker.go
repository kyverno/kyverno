package webhooks

import (
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) verifyHandler(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithName("verifyHandler").WithValues("action", "verify", "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	logger.V(3).Info("incoming request", "last admission request timestamp", ws.webhookMonitor.Time())

	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
