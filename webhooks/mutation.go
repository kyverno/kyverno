package webhooks

import (
	"encoding/json"
	"errors"
	"log"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreTypes "k8s.io/kubernetes/pkg/apis/core"
)

type MutationWebhook struct {
	logger *log.Logger
}

func NewMutationWebhook(logger *log.Logger) (*MutationWebhook, error) {
	if logger == nil {
		return nil, errors.New("Logger must be set for the mutation webhook")
	}
	return &MutationWebhook{logger: logger}, nil
}

func (mw *MutationWebhook) Mutate(request *v1beta1.AdmissionRequest, policies []types.Policy) *v1beta1.AdmissionResponse {
	mw.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	if len(policies) == 0 {
		return nil
	}

	var configMap coreTypes.ConfigMap
	if err := json.Unmarshal(request.Object.Raw, &configMap); err != nil {
		mw.logger.Printf("Could not unmarshal raw object: %v", err)
		return errorToResponse(err)
	}
	/*patch := patchOperation{
		Path: "/labels",
		Op:   "add",
		Value: map[string]string{
			"is-mutated": "true",
		},
	}*/
	patch := `[ {"op":"add","path":"/metadata/labels","value":{"is-mutated":"true"}} ]`

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   []byte(patch),
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func errorToResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
