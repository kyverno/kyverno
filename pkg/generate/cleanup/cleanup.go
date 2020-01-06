package cleanup

import (
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/api/errors"
)

const timoutMins = 2
const timeout = time.Minute * timoutMins // 5 minutes

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	// 1-Corresponding policy has been deleted
	_, err := c.pLister.Get(gr.Spec.Policy)
	if errors.IsNotFound(err) {
		return c.control.Delete(gr.Name)
	}

	// 2- Check for elapsed time since update
	// - If status.state is [Pending,Failed]
	if gr.Status.State == kyverno.Completed {
		if !ownerResourceExists(c.client, gr) {
			if err := deleteGeneratedResources(c.client, gr); err != nil {
				return err
			}
			return c.control.Delete(gr.Name)
		}
		return nil
	}
	createTime := gr.GetCreationTimestamp()
	if time.Since(createTime.UTC()) > timeout {
		// the GR was in state [Pending,Failed] for more than timeout
		glog.V(4).Infof("GR %s was not processed succesfully in %d minutes", gr.Name, timoutMins)
		return c.control.Delete(gr.Name)
	}
	return nil
}

func ownerResourceExists(client *dclient.Client, gr kyverno.GenerateRequest) bool {
	_, err := client.GetResource(gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name)
	if err != nil {
		return false
	}
	return true
}

func deleteGeneratedResources(client *dclient.Client, gr kyverno.GenerateRequest) error {
	for _, genResource := range gr.Status.GeneratedResources {
		err := client.DeleteResource(genResource.Kind, genResource.Namespace, genResource.Name, false)
		if errors.IsNotFound(err) {
			glog.V(4).Infof("resource %s/%s/%s not found, will no delete", genResource.Kind, genResource.Namespace, genResource.Name)
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}
