package utils

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
)

type UpdateRequestUpdater interface {
	// UpdateAnnotation updates UR annotation, triggering reprocessing of UR and recreation/updation of generated resource
	UpdateAnnotation(logger logr.Logger, name string)
}

type updateRequestUpdater struct {
	client versioned.Interface
	lister kyvernov1beta1listers.UpdateRequestNamespaceLister
}

func NewUpdateRequestUpdater(client versioned.Interface, lister kyvernov1beta1listers.UpdateRequestNamespaceLister) UpdateRequestUpdater {
	return &updateRequestUpdater{
		client: client,
		lister: lister,
	}
}

func (h *updateRequestUpdater) updateAnnotation(logger logr.Logger, name string) {
	if _, err := common.Update(h.client, h.lister, name, func(ur *kyvernov1beta1.UpdateRequest) {
		urAnnotations := ur.Annotations
		if len(urAnnotations) == 0 {
			urAnnotations = make(map[string]string)
		}
		urAnnotations["generate.kyverno.io/updation-time"] = time.Now().String()
		ur.SetAnnotations(urAnnotations)
	}); err != nil {
		logger.Error(err, "failed to update update request update-time annotations for the resource", "update request", name)
	}
}

func (h *updateRequestUpdater) setPendingStatus(logger logr.Logger, name string) {
	if _, err := common.UpdateStatus(h.client, h.lister, name, kyvernov1beta1.Pending, "", nil); err != nil {
		logger.Error(err, "failed to set UpdateRequest state to Pending", "update request", name)
	}
}

func (h *updateRequestUpdater) UpdateAnnotation(logger logr.Logger, name string) {
	h.updateAnnotation(logger, name)
	h.setPendingStatus(logger, name)
}
