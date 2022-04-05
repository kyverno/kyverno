package generate

import (
	"context"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PatchGenerateRequest patches a generate request object
func PatchGenerateRequest(gr *kyverno.GenerateRequest, patch jsonutils.Patch, client kyvernoclient.Interface, subresources ...string) (*kyverno.GenerateRequest, error) {
	data, err := patch.ToPatchBytes()
	if nil != err {
		return gr, err
	}
	newGR, err := client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Patch(context.TODO(), gr.Name, types.JSONPatchType, data, metav1.PatchOptions{}, subresources...)
	if err != nil {
		return gr, err
	}
	return newGR, nil
}
