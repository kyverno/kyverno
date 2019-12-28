package cleanup

import (
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

const timoutMins = 5
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
		return nil
	}
	createTime := gr.GetCreationTimestamp()
	if time.Since(createTime.UTC()) > timeout {
		// the GR was in state [Pending,Failed] for more than timeout
		glog.V(4).Info("GR %s was not processed succesfully in %d", gr.Name, timoutMins)
		return c.control.Delete(gr.Name)
	}
	return nil
}
