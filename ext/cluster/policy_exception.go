package cluster

import (
	"context"
	"slices"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type policyExceptionSelector struct {
	additional    []*kyvernov2.PolicyException
	kyvernoClient versioned.Interface
	namespace     string
}

func (c policyExceptionSelector) Find(policy, rule string) ([]*kyvernov2.PolicyException, error) {
	var exceptions []*kyvernov2.PolicyException
	if c.kyvernoClient != nil {
		list, err := c.kyvernoClient.KyvernoV2().PolicyExceptions(c.namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			for i, exc := range list.Items {
				for _, e := range exc.Spec.Exceptions {
					if e.PolicyName == policy && slices.Contains(e.RuleNames, rule) {
						pe := list.Items[i]
						exceptions = append(exceptions, &pe)
					}
				}
			}
		} else if !kerrors.IsNotFound(err) {
			return nil, err
		}
	}
	for _, exception := range c.additional {
		if c.namespace == "" || exception.GetNamespace() == c.namespace {
			exceptions = append(exceptions, exception)
		}
	}
	return exceptions, nil
}

func NewPolicyExceptionSelector(namespace string, client versioned.Interface, exceptions ...*kyvernov2.PolicyException) engineapi.PolicyExceptionSelector {
	return policyExceptionSelector{
		additional:    exceptions,
		kyvernoClient: client,
		namespace:     namespace,
	}
}
