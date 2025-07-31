package mpol

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	libs "github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
)

type processor struct {
	kyvernoClient versioned.Interface
	client        dclient.Interface

	engine        mpolengine.Engine
	mapper        meta.RESTMapper
	context       libs.Context
	statusControl common.StatusControlInterface

	reportsConfig  reportutils.ReportingConfiguration
	reportsBreaker breaker.Breaker
}

func NewProcessor(client dclient.Interface,
	kyvernoClient versioned.Interface,
	mpolEngine mpolengine.Engine,
	mapper meta.RESTMapper,
	context libs.Context,
	reportsConfig reportutils.ReportingConfiguration,
	reportsBreaker breaker.Breaker,
	statusControl common.StatusControlInterface,
) *processor {
	return &processor{
		client:         client,
		kyvernoClient:  kyvernoClient,
		engine:         mpolEngine,
		mapper:         mapper,
		context:        context,
		statusControl:  statusControl,
		reportsConfig:  reportsConfig,
		reportsBreaker: reportsBreaker,
	}
}

func (p *processor) Process(ur *kyvernov2.UpdateRequest) error {
	var failures []error
	mpol, err := p.kyvernoClient.PoliciesV1alpha1().MutatingPolicies().Get(context.TODO(), ur.Spec.Policy, metav1.GetOptions{})
	if err != nil {
		failures = append(failures, fmt.Errorf("failed to fetch mpol %s: %v", ur.Spec.GetPolicyKey(), err))
		return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
	}

	targetConstraints := mpol.GetSpec().GetMatchConstraints()
	if len(mpol.GetSpec().GetTargetMatchConstraints().ResourceRules) != 0 {
		targetConstraints = mpol.GetSpec().GetTargetMatchConstraints()
	}

	var targets *unstructured.UnstructuredList
	results := collectGVK(p.client, p.mapper, targetConstraints)
	for ns, gvks := range results {
		for r := range gvks {
			if r.Kind == "Namespace" || ns == "*" {
				ns = ""
			}
			targets, err = p.client.ListResource(context.TODO(), r.GroupVersion().String(), r.Kind, ns, targetConstraints.ObjectSelector)
			if err != nil {
				failures = append(failures, fmt.Errorf("failed to fetch targets %s for mpol %s: %v", r.String(), ur.Spec.GetPolicyKey(), err))
			}
		}
	}

	if targets == nil {
		return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
	}

	ar := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	for _, target := range targets.Items {
		object := &target
		mapping, err := p.mapper.RESTMapping(target.GroupVersionKind().GroupKind(), target.GroupVersionKind().Version)
		if err != nil {
			failures = append(failures, fmt.Errorf("failed to get resource version for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}

		attr := admission.NewAttributesRecord(
			object,
			nil,
			object.GroupVersionKind(),
			object.GetNamespace(),
			object.GetName(),
			mapping.Resource,
			"",
			admission.Operation(""),
			nil,
			false,
			// TODO
			nil,
		)

		response, err := p.engine.Evaluate(context.TODO(), attr, *ar, mpolengine.MatchNames(ur.Spec.Policy))
		if err != nil {
			failures = append(failures, fmt.Errorf("failed to evaluate mpol %s: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		if response.PatchedResource != nil {
			object, err = p.client.GetResource(context.TODO(), object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())
			new := response.PatchedResource
			new.SetResourceVersion(object.GetResourceVersion())
			if err != nil {
				failures = append(failures, fmt.Errorf("failed to refresh target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
			}
			if _, err := p.client.UpdateResource(context.TODO(), new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new.Object, false, ""); err != nil {
				failures = append(failures, fmt.Errorf("failed to update target resource for mpol %s: %v", ur.Spec.GetPolicyKey(), err))
			}

			if p.reportsConfig.MutateExistingReportsEnabled() {
				err := p.createReports(object, &response)
				if err != nil {
					logger.Error(err, "failed to create reports for mpol", "mpol", ur.Spec.GetPolicyKey())
				}
			}
		}
	}
	return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}

func (p *processor) createReports(object *unstructured.Unstructured, response *mpolengine.EngineResponse) error {
	engineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, res := range response.Policies {
		engineResponses = append(engineResponses, engineapi.EngineResponse{
			Resource: *response.PatchedResource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: res.Rules,
			},
		}.WithPolicy(engineapi.NewMutatingPolicy(res.Policy)))
	}

	report := reportutils.BuildMutateExistingReport(object.GetNamespace(), object.GroupVersionKind(), object.GetName(), object.GetUID(), engineResponses...)
	if len(report.GetResults()) > 0 {
		err := p.reportsBreaker.Do(context.TODO(), func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, p.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func collectGVK(client dclient.Interface, mapper meta.RESTMapper, m admissionregistrationv1.MatchResources) map[string]sets.Set[schema.GroupVersionKind] {
	result := make(map[string]sets.Set[schema.GroupVersionKind])

	gvkSet := sets.New[schema.GroupVersionKind]()
	for _, rule := range m.ResourceRules {
		for _, group := range rule.APIGroups {
			for _, version := range rule.APIVersions {
				for _, resource := range rule.Resources {
					baseResource := resource
					if strings.Contains(resource, "/") {
						baseResource = strings.Split(resource, "/")[0]
					}
					gvr := schema.GroupVersionResource{
						Group:    group,
						Version:  version,
						Resource: baseResource,
					}
					gvk, err := mapper.KindFor(gvr)
					if err != nil {
						continue
					}
					gvkSet.Insert(gvk)
				}
			}
		}
	}

	if m.NamespaceSelector != nil {
		namespaces, err := client.ListResource(context.TODO(), "v1", "Namespace", "", m.NamespaceSelector)
		if err != nil {
			return result
		}

		for _, ns := range namespaces.Items {
			result[ns.GetName()] = gvkSet
		}
	} else {
		result["*"] = gvkSet
	}
	return result
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
