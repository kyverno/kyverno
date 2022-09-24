package v1alpha2

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
)

type client struct {
	v1alpha2.KyvernoV1alpha2Interface
	clientQueryMetric utils.ClientQueryMetric
}

func Wrap(inner v1alpha2.KyvernoV1alpha2Interface, m utils.ClientQueryMetric) v1alpha2.KyvernoV1alpha2Interface {
	return &client{inner, m}
}
