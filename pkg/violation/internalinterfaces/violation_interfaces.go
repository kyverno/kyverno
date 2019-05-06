package internalinterfaces

import (
	"github.com/nirmata/kube-policy/controller/internalinterfaces"
	utils "github.com/nirmata/kube-policy/pkg/violation/utils"
)

type ViolationGenerator interface {
	SetController(controller internalinterfaces.PolicyGetter)
	Create(info utils.ViolationInfo) error
}
