package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewBackgroundContext(dclient dclient.Interface, ur *kyvernov1beta1.UpdateRequest,
	policy kyvernov1.PolicyInterface,
	trigger *unstructured.Unstructured,
	cfg config.Configuration,
	informerCacheResolvers resolvers.ConfigmapResolver,
	namespaceLabels map[string]string,
	logger logr.Logger,
) (*engine.PolicyContext, bool, error) {
	ctx := context.NewContext()
	var new, old unstructured.Unstructured
	var err error

	if ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		if err := ctx.AddRequest(ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest); err != nil {
			return nil, false, errors.Wrap(err, "failed to load request in context")
		}

		new, old, err = admissionutils.ExtractResources(nil, ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to load request in context")
		}

		if !reflect.DeepEqual(new, unstructured.Unstructured{}) {
			if !check(&new, trigger) {
				err := fmt.Errorf("resources don't match")
				return nil, false, errors.Wrapf(err, "resource %v", ur.Spec.Resource)
			}
		}
	}

	if trigger == nil {
		trigger = &old
	}

	if trigger == nil {
		return nil, false, errors.New("trigger resource does not exist")
	}

	err = ctx.AddResource(trigger.Object)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to load resource in context")
	}

	err = ctx.AddOldResource(old.Object)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to load resource in context")
	}

	err = ctx.AddUserInfo(ur.Spec.Context.UserRequestInfo)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load SA in context")
	}

	err = ctx.AddServiceAccount(ur.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load UserInfo in context")
	}

	if err := ctx.AddImageInfos(trigger); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	policyContext := engine.NewPolicyContextWithJsonContext(ctx).
		WithPolicy(policy).
		WithNewResource(*trigger).
		WithOldResource(old).
		WithAdmissionInfo(ur.Spec.Context.UserRequestInfo).
		WithConfiguration(cfg).
		WithNamespaceLabels(namespaceLabels).
		WithClient(dclient).
		WithInformerCacheResolver(informerCacheResolvers)

	return policyContext, false, nil
}

func check(admissionRsc, existingRsc *unstructured.Unstructured) bool {
	if existingRsc == nil {
		return admissionRsc == nil
	}
	if admissionRsc.GetName() != existingRsc.GetName() {
		return false
	}
	if admissionRsc.GetNamespace() != existingRsc.GetNamespace() {
		return false
	}
	if admissionRsc.GetKind() != existingRsc.GetKind() {
		return false
	}
	if admissionRsc.GetAPIVersion() != existingRsc.GetAPIVersion() {
		return false
	}
	return true
}
