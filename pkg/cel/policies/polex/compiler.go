package polex

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Compiler interface {
	Compile(exception policiesv1beta1.PolicyException) (*Exception, field.ErrorList)
}
