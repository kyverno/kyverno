package context

import (
	"context"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type impl struct {
	types.Adapter
	client kubernetes.Interface
}

func (c *impl) context_get_cm(arg ref.Val) ref.Val {
	if ref, err := utils.ConvertToNative[ConfigMapReference](arg); err != nil {
		return types.WrapErr(err)
	} else {
		cm, err := c.client.CoreV1().ConfigMaps(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get resource: %v", err)
		}
		out, err := utils.ConvertObjectToUnstructured(cm)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to convert to unstructured: %v", err)
		}
		return c.NativeToValue(out.UnstructuredContent())
	}
}
