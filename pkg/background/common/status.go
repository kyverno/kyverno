package common

import (
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
)

// StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(name string, message string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error)
	Success(name string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error)
	Skip(name string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error)
}

// statusControl is default implementaation of GRStatusControlInterface
type statusControl struct {
	client   versioned.Interface
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister
}

func NewStatusControl(client versioned.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister) StatusControlInterface {
	return &statusControl{
		client:   client,
		urLister: urLister,
	}
}

// Failed sets ur status.state to failed with message
func (sc *statusControl) Failed(name, message string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error) {
	return UpdateStatus(sc.client, sc.urLister, name, kyverno.Failed, message, genResources)
}

// Success sets the ur status.state to completed and clears message
func (sc *statusControl) Success(name string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error) {
	return UpdateStatus(sc.client, sc.urLister, name, kyverno.Completed, "", genResources)
}

// Success sets the ur status.state to completed and clears message
func (sc *statusControl) Skip(name string, genResources []kyverno.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error) {
	return UpdateStatus(sc.client, sc.urLister, name, kyverno.Skip, "", genResources)
}
