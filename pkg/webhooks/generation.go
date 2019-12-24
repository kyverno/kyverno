package webhooks

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	glog.V(4).Infof("Handle Generate: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)
	// Generate Stats wont be used here, as we delegate the generate rule
	// - Filter policies that apply on this resource
	// - - build CR context(userInfo+roles+clusterRoles)
	// - Create CR
	// - send Success
	// HandleGeneration  always returns success

	// Filter Policies
	return false, ""
}
