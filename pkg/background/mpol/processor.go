package mpol

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	libs "github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policy"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	reportsConfig reportutils.ReportingConfiguration
	eventGen      event.Interface
}

func NewProcessor(client dclient.Interface,
	kyvernoClient versioned.Interface,
	mpolEngine mpolengine.Engine,
	mapper meta.RESTMapper,
	context libs.Context,
	reportsConfig reportutils.ReportingConfiguration,
	statusControl common.StatusControlInterface,
	eventGen event.Interface,
) *processor {
	return &processor{
		client:        client,
		kyvernoClient: kyvernoClient,
		engine:        mpolEngine,
		mapper:        mapper,
		context:       context,
		statusControl: statusControl,
		reportsConfig: reportsConfig,
		eventGen:      eventGen,
	}
}

func (p *processor) Process(ur *kyvernov2.UpdateRequest) error {
	mpol, err := p.GetPolicy(ur)
	if mpol == nil {
		return err
	}

	var failures []error
	targetConstraints := mpol.GetMatchConstraints()
	if len(mpol.GetTargetMatchConstraints().ResourceRules) != 0 {
		targetConstraints = mpol.GetTargetMatchConstraints()
	}

	var targets *unstructured.UnstructuredList
	results := collectGVK(p.client, p.mapper, targetConstraints, mpol.GetNamespace())
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

		response, err := p.engine.Evaluate(context.TODO(), attr, *ar, mpolengine.And(mpolengine.MatchNames(ur.Spec.Policy), mpolengine.Or(mpolengine.ClusteredPolicy(), mpolengine.NamespacedPolicy(attr.GetNamespace()))))
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

			err := p.audit(object, &response)
			if err != nil {
				logger.Error(err, "failed to create reports for mpol", "mpol", ur.Spec.GetPolicyKey())
			}
		}
	}
	return updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}

func (p *processor) audit(object *unstructured.Unstructured, response *mpolengine.EngineResponse) error {
	allEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	reportableEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: r.Rules,
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))
		allEngineResponses = append(allEngineResponses, engineResponse)
		if reportutils.IsPolicyReportable(r.Policy) {
			reportableEngineResponses = append(reportableEngineResponses, engineResponse)
		}
	}

	events := webhookutils.GenerateEvents(allEngineResponses, false)
	p.eventGen.Add(events...)

	if !p.reportsConfig.MutateExistingReportsEnabled() {
		return nil
	}

	report := reportutils.BuildMutateExistingReport(object.GetNamespace(), object.GroupVersionKind(), object.GetName(), object.GetUID(), reportableEngineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(context.TODO(), func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, p.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func collectGVK(client dclient.Interface, mapper meta.RESTMapper, m admissionregistrationv1.MatchResources, ns string) map[string]sets.Set[schema.GroupVersionKind] {
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

	if ns != "" {
		namespace, err := client.GetResource(context.TODO(), "v1", "Namespace", "", ns)
		if err != nil {
			return result
		}
		result[namespace.GetName()] = gvkSet
		return result
	} else if m.NamespaceSelector != nil {
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

func (p *processor) GetPolicy(ur *kyvernov2.UpdateRequest) (v1beta1.MutatingPolicyLike, error) {
	var mpol v1beta1.MutatingPolicyLike
	var err error

	var failures []error
	mpol, err = p.kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(context.TODO(), ur.Spec.Policy, metav1.GetOptions{})
	if err == nil {
		return mpol, nil
	}

	// Try NamespacedMutatingPolicy
	if errors.IsNotFound(err) {
		name, ns := policy.ParsePolicyKey(ur.Spec.Policy)
		mpol, err = p.kyvernoClient.PoliciesV1beta1().NamespacedMutatingPolicies(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return mpol, nil
		}
	}

	failures = append(failures, fmt.Errorf("failed to fetch mpol %s: %v", ur.Spec.GetPolicyKey(), err))
	return nil, updateURStatus(p.statusControl, *ur, multierr.Combine(failures...), nil)
}
