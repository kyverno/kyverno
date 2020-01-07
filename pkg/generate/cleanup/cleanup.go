package cleanup

import (
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/api/errors"
)

const timoutMins = 2
const timeout = time.Minute * timoutMins // 2 minutes

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	glog.V(4).Info("processGR cleanup")
	// 1-Corresponding policy has been deleted
	_, err := c.pLister.Get(gr.Spec.Policy)
	if errors.IsNotFound(err) {
		glog.V(4).Infof("delete GR %s", gr.Name)
		return c.control.Delete(gr.Name)
	}

	// 2- Check for elapsed time since update
	if gr.Status.State == kyverno.Completed {
		glog.V(4).Infof("checking if owner exists for gr %s", gr.Name)
		if !ownerResourceExists(c.client, gr) {
			glog.V(4).Infof("delete GR %s", gr.Name)
			return c.control.Delete(gr.Name)
		}
		return nil
	}
	createTime := gr.GetCreationTimestamp()
	glog.V(4).Infof("state %s", string(gr.Status.State))
	if time.Since(createTime.UTC()) > timeout {
		// the GR was in state ["",Failed] for more than timeout
		glog.V(4).Infof("GR %s was not processed succesfully in %d minutes", gr.Name, timoutMins)
		glog.V(4).Infof("delete GR %s", gr.Name)
		return c.control.Delete(gr.Name)
	}
	return nil
}

func ownerResourceExists(client *dclient.Client, gr kyverno.GenerateRequest) bool {
	_, err := client.GetResource(gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name)
	if err != nil {
		glog.V(4).Info("cleanup Resource does not exits")
		return false
	}
	glog.V(4).Info("cleanup Resource does exits")
	return true
}
