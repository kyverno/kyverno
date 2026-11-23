package common

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
)

// StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(name string, message string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error)
	Success(name string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error)
	Skip(name string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error)
}

// statusControl is default implementaation of GRStatusControlInterface
type statusControl struct {
	client   versioned.Interface
	urLister kyvernov2listers.UpdateRequestNamespaceLister
}

func NewStatusControl(client versioned.Interface, urLister kyvernov2listers.UpdateRequestNamespaceLister) StatusControlInterface {
	return &statusControl{
		client:   client,
		urLister: urLister,
	}
}

// Failed sets ur status.state to failed with message
func (sc *statusControl) Failed(name, message string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return UpdateStatus(context.TODO(), sc.client, sc.urLister, name, kyvernov2.Failed, message, genResources)
}

// Success sets the ur status.state to completed and clears message
func (sc *statusControl) Success(name string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return UpdateStatus(context.TODO(), sc.client, sc.urLister, name, kyvernov2.Completed, "", genResources)
}

// Success sets the ur status.state to completed and clears message
func (sc *statusControl) Skip(name string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return UpdateStatus(context.TODO(), sc.client, sc.urLister, name, kyvernov2.Skip, "", genResources)
}
