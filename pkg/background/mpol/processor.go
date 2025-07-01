package mpol

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	libs "github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type processor struct {
	kyvernoClient versioned.Interface
	client        dclient.Interface

	engine        mpolengine.Engine
	mapper        meta.RESTMapper
	context       libs.Context
	statusControl common.StatusControlInterface
}

func NewProcessor(client dclient.Interface, kyvernoClient versioned.Interface, mpolEngine mpolengine.Engine, mapper meta.RESTMapper, context libs.Context, statusControl common.StatusControlInterface) *processor {
	return &processor{
		client:        client,
		kyvernoClient: kyvernoClient,
		engine:        mpolEngine,
		mapper:        mapper,
		context:       context,
		statusControl: statusControl,
	}
}

func (p *processor) Process(ur *kyvernov2.UpdateRequest) error {
	var target *unstructured.Unstructured
	var failures []error

	if ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		request := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
		newResource, err := admissionutils.ConvertResource(request.Object.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err != nil {
			failures = append(failures, fmt.Errorf("failed to parse target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
		}
		target = &newResource
	}
	mapping, err := p.mapper.RESTMapping(target.GroupVersionKind().GroupKind(), target.GroupVersionKind().Version)
	if err != nil {
		failures = append(failures, fmt.Errorf("failed to get resource version for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
	}

	attr := admission.NewAttributesRecord(
		target,
		nil,
		schema.GroupVersionKind(target.GroupVersionKind()),
		target.GetNamespace(),
		target.GetName(),
		mapping.Resource,
		"",
		admission.Operation(""),
		nil,
		false,
		// TODO
		nil,
	)

	response, err := p.engine.Evaluate(context.TODO(), attr, mpolengine.MatchNames(ur.Spec.Policy))
	if err != nil {
		failures = append(failures, fmt.Errorf("failed to evaluate mpol %s: %v", ur.Spec.GetPolicyKey(), err))
		return err
	}
	if response.PatchedResource != nil {
		target, err = p.client.GetResource(context.TODO(), target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName())
		new := response.PatchedResource
		new.SetResourceVersion(target.GetResourceVersion())
		if err != nil {
			failures = append(failures, fmt.Errorf("failed to refresh targe resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
		}
		if _, err := p.client.UpdateResource(context.TODO(), new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new.Object, false, ""); err != nil {
			failures = append(failures, fmt.Errorf("failed to update target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
		}
	}
	return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}
