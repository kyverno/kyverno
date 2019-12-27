package cleanup

import (
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ControlInterface interface {
	Delete(gr string) error
}

type Control struct {
	client kyvernoclient.Interface
}

func (c Control) Delete(gr string) error {
	return c.client.KyvernoV1().GenerateRequests("kyverno").Delete(gr,&metav1.DeleteOptions{})
}
