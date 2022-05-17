package common

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PatchUpdateRequest patches a update request object
func PatchUpdateRequest(ur *kyvernov1beta1.UpdateRequest, patch jsonutils.Patch, client kyvernoclient.Interface, subresources ...string) (*kyvernov1beta1.UpdateRequest, error) {
	data, err := patch.ToPatchBytes()
	if nil != err {
		return ur, err
	}
	newUR, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Patch(context.TODO(), ur.Name, types.JSONPatchType, data, metav1.PatchOptions{}, subresources...)
	if err != nil {
		return ur, err
	}
	return newUR, nil
}
