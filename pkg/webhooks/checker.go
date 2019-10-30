package webhooks

import (
	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) handleVerifyRequest(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	glog.V(4).Infof("Receive request in mutating webhook '/verify': Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
