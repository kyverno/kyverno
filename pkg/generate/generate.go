package generate

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	// 1 - Check if the resource exists
	resource, err := getResource(c.client, gr.Spec.Resource)
	if err != nil {
		// Dont update status
		glog.V(4).Info("resource does not exist or is yet to be created, requeuing: %v", err)
		return err
	}
	// 2 - Apply the generate policy on the resource
	err = applyGenerate(*resource, gr)
	// 3 - Update Status
	return updateStatus(c.statusControl, gr, err)
}

func applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest) error {
	return nil
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error())
	}

	// Generate request successfully processed
	return statusControl.Success(gr)
}
