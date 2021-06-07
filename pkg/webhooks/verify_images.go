package webhooks

import (
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) handleVerifyImages(request *v1beta1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []*v1.ClusterPolicy,
	admissionRequestTimestamp int64) (*response.EngineResponse) {




	return nil
}
