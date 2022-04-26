package cleanup

import (
	"context"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControlInterface manages resource deletes
type ControlInterface interface {
	Delete(gr string) error
}

//Control provides implementation to manage resource
type Control struct {
	client kyvernoclient.Interface
}

//Delete deletes the specified resource
func (c Control) Delete(gr string) error {
	return c.client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(context.TODO(), gr, metav1.DeleteOptions{})
}
