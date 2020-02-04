package cleanup

import (
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
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
	return c.client.KyvernoV1().GenerateRequests("kyverno").Delete(gr, &metav1.DeleteOptions{})
}
