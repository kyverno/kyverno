package interfaces

import (
	controllerinterfaces "github.com/nirmata/kube-policy/controller/interfaces"
	utils "github.com/nirmata/kube-policy/pkg/violation/utils"
)

type ViolationGenerator interface {
	SetController(controller controllerinterfaces.PolicyGetter)
	Create(info utils.ViolationInfo) error
}
