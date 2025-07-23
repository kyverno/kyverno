package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	regex "github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/event"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	validationpolicy "github.com/kyverno/kyverno/pkg/validation/policy"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type GenerateController struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	statusControl common.StatusControlInterface
	engine        engineapi.Engine

	// listers
	urLister      kyvernov2listers.UpdateRequestNamespaceLister
	nsLister      corev1listers.NamespaceLister
	policyLister  kyvernov1listers.ClusterPolicyLister
	npolicyLister kyvernov1listers.PolicyLister

	configuration config.Configuration
	eventGen      event.Interface

	log logr.Logger
	jp  jmespath.Interface

	reportsConfig reportutils.ReportingConfiguration
}

// NewGenerateController returns an instance of the Generate-Request Controller
func NewGenerateController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	statusControl common.StatusControlInterface,
	engine engineapi.Engine,
	policyLister kyvernov1listers.ClusterPolicyLister,
	npolicyLister kyvernov1listers.PolicyLister,
	urLister kyvernov2listers.UpdateRequestNamespaceLister,
	nsLister corev1listers.NamespaceLister,
	dynamicConfig config.Configuration,
	eventGen event.Interface,
	log logr.Logger,
	jp jmespath.Interface,
	reportsConfig reportutils.ReportingConfiguration,
) *GenerateController {
	c := GenerateController{
		client:        client,
		kyvernoClient: kyvernoClient,
		statusControl: statusControl,
		engine:        engine,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		urLister:      urLister,
		nsLister:      nsLister,
		configuration: dynamicConfig,
		eventGen:      eventGen,
		log:           log,
		jp:            jp,
		reportsConfig: reportsConfig,
	}
	return &c
}

func (c *GenerateController) ProcessUR(ur *kyvernov2.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey())
	var genResources []kyvernov1.ResourceSpec
	logger.V(2).Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	var failures []error
	policy, err := c.getPolicyObject(*ur)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error in fetching policy: %v", err)
	}

	for i := 0; i < len(ur.Spec.RuleContext); i++ {
		rule := ur.Spec.RuleContext[i]
		trigger, err := common.GetTrigger(c.client, ur.Spec, i, c.log)
		if err != nil || trigger == nil {
			logger.V(4).Info("the trigger resource does not exist or is pending creation")
			failures = append(failures, fmt.Errorf("rule %s failed: failed to fetch trigger resource: %v", rule.Rule, err))
			continue
		}

		genResources, err = c.applyGenerate(*trigger, *ur, policy, i)
		if err != nil {
			if strings.Contains(err.Error(), doesNotApply) {
				logger.V(3).Info(fmt.Sprintf("skipping rule %s: %v", rule.Rule, err.Error()))
			}

			events := event.NewBackgroundFailedEvent(err, policy, ur.Spec.RuleContext[i].Rule, event.GeneratePolicyController,
				kyvernov1.ResourceSpec{Kind: trigger.GetKind(), Namespace: trigger.GetNamespace(), Name: trigger.GetName()})
			c.eventGen.Add(events...)
		}
	}

	return updateStatus(c.statusControl, *ur, multierr.Combine(failures...), genResources)
}

const doesNotApply = "policy does not apply to resource"

func (c *GenerateController) applyGenerate(trigger unstructured.Unstructured, ur kyvernov2.UpdateRequest, policy kyvernov1.PolicyInterface, i int) ([]kyvernov1.ResourceSpec, error) {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey())
	logger.V(3).Info("applying generate policy")

	ruleContext := ur.Spec.RuleContext[i]
	if ruleContext.DeleteDownstream || policy == nil {
		return nil, c.deleteDownstream(policy, ruleContext, &ur)
	}

	p, ok := buildPolicyWithAppliedRules(policy, ruleContext.Rule)
	if !ok {
		logger.V(4).Info("skip rule application as the rule does not exist in the updaterequest", "rule", ruleContext.Rule)
		return nil, nil
	}

	namespaceLabels, err := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), c.nsLister, []kyvernov1.PolicyInterface{p}, logger)
	if err != nil {
		return nil, err
	}

	policyContext, err := common.NewBackgroundContext(logger, c.client, ur.Spec.Context, p, &trigger, c.configuration, c.jp, namespaceLabels)
	if err != nil {
		return nil, err
	}

	admissionRequest := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	if admissionRequest != nil {
		var gvk schema.GroupVersionKind
		gvk, err = c.client.Discovery().GetGVKFromGVR(schema.GroupVersionResource(admissionRequest.Resource))
		if err != nil {
			return nil, err
		}
		policyContext = policyContext.WithResourceKind(gvk, admissionRequest.SubResource)
	}

	// check if the policy still applies to the resource
	engineResponse := c.engine.Generate(context.Background(), policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info(doesNotApply)
		return nil, errors.New(doesNotApply)
	}

	var applicableRules []string
	for _, r := range engineResponse.PolicyResponse.Rules {
		if r.Status() == engineapi.RuleStatusPass {
			applicableRules = append(applicableRules, r.Name())
		}
	}

	// Apply the generate rule on resource
	genResourcesMap, err := c.ApplyGeneratePolicy(logger, policyContext, applicableRules)
	if err != nil {
		return nil, err
	}

	for i, v := range engineResponse.PolicyResponse.Rules {
		if resources, ok := genResourcesMap[v.Name()]; ok {
			unstResources, err := c.GetUnstrResources(resources)
			if err != nil {
				c.log.Error(err, "failed to get unst resource names report")
			}
			engineResponse.PolicyResponse.Rules[i] = *v.WithGeneratedResources(unstResources)
		}
	}

	if c.needsReports(trigger) {
		if err := c.createReports(context.TODO(), policyContext.NewResource(), engineResponse); err != nil {
			c.log.Error(err, "failed to create report")
		}
	}

	genResources := make([]kyvernov1.ResourceSpec, 0)
	for _, v := range genResourcesMap {
		genResources = append(genResources, v...)
	}

	for _, res := range genResources {
		e := event.NewResourceGenerationEvent(ur.Spec.Policy, ur.Spec.RuleContext[i].Rule, event.GeneratePolicyController, res)
		c.eventGen.Add(e)
	}

	e := event.NewBackgroundSuccessEvent(event.GeneratePolicyController, policy, genResources)
	c.eventGen.Add(e...)

	return genResources, err
}

// getPolicyObject gets the policy spec from the ClusterPolicy/Policy
func (c *GenerateController) getPolicyObject(ur kyvernov2.UpdateRequest) (kyvernov1.PolicyInterface, error) {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return nil, err
	}

	if pNamespace == "" {
		policyObj, err := c.policyLister.Get(pName)
		if err != nil {
			return nil, err
		}
		return policyObj, err
	}
	npolicyObj, err := c.npolicyLister.Policies(pNamespace).Get(pName)
	if err != nil {
		return nil, err
	}
	return npolicyObj, nil
}

func (c *GenerateController) ApplyGeneratePolicy(log logr.Logger, policyContext *engine.PolicyContext, applicableRules []string) (map[string][]kyvernov1.ResourceSpec, error) {
	genResources := make(map[string][]kyvernov1.ResourceSpec)
	policy := policyContext.Policy()
	resource := policyContext.NewResource()
	// To manage existing resources, we compare the creation time for the default resource to be generated and policy creation time
	ruleNameToProcessingTime := make(map[string]time.Duration)
	applyRules := policy.GetSpec().GetApplyRules()
	applyCount := 0
	log = log.WithValues("policy", policy.GetName(), "trigger", resource.GetNamespace()+"/"+resource.GetName())

	for _, rule := range policy.GetSpec().Rules {
		var err error
		if !rule.HasGenerate() {
			continue
		}

		if !slices.Contains(applicableRules, rule.Name) {
			continue
		}
		if rule.Generation.Synchronize {
			ruleRaw, err := json.Marshal(rule.DeepCopy())
			if err != nil {
				return nil, fmt.Errorf("failed to serialize the policy: %v", err)
			}
			vars := regex.RegexVariables.FindAllStringSubmatch(string(ruleRaw), -1)

			for _, s := range vars {
				for _, banned := range validationpolicy.ForbiddenUserVariables {
					if banned.Match([]byte(s[2])) {
						log.V(2).Info("warning: resources with admission request variables may not be regenerated", "policy", policy.GetName(), "rule", rule.Name, "variable", s[2])
					}
				}
			}
		}

		startTime := time.Now()
		var genResource []kyvernov1.ResourceSpec
		if applyRules == kyvernov1.ApplyOne && applyCount > 0 {
			break
		}
		logger := log.WithValues("rule", rule.Name)
		contextLoader := c.engine.ContextLoader(policy, rule)
		if err := contextLoader(context.TODO(), rule.Context, policyContext.JSONContext()); err != nil {
			if _, ok := err.(gojmespath.NotFoundError); ok {
				logger.V(3).Info("failed to load rule level context", "reason", err.Error())
			} else {
				logger.Error(err, "failed to load rule level context")
			}
			return nil, fmt.Errorf("failed to load rule level context: %v", err)
		}

		if rule.Generation.ForEachGeneration != nil {
			g := newForeachGenerator(c.client, logger, policyContext, policy, rule, rule.Context, rule.GetAnyAllConditions(), policyContext.NewResource(), rule.Generation.ForEachGeneration, contextLoader)
			genResource, err = g.generateForeach()
		} else {
			g := newGenerator(c.client, logger, policyContext, policy, rule, rule.Context, rule.GetAnyAllConditions(), policyContext.NewResource(), rule.Generation.GeneratePattern, contextLoader)
			genResource, err = g.generate()
		}

		if err != nil {
			log.Error(err, "failed to apply generate rule")
			return nil, err
		}
		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		genResources[rule.Name] = genResource
		applyCount++
	}

	return genResources, nil
}

// NewGenerateControllerWithOnlyClient returns an instance of Controller with only the client.
func NewGenerateControllerWithOnlyClient(client dclient.Interface, engine engineapi.Engine) *GenerateController {
	c := GenerateController{
		client: client,
		engine: engine,
	}
	return &c
}

// GetUnstrResource converts ResourceSpec object to type Unstructured
func (c *GenerateController) GetUnstrResources(genResourceSpecs []kyvernov1.ResourceSpec) ([]*unstructured.Unstructured, error) {
	resources := []*unstructured.Unstructured{}
	for _, genResourceSpec := range genResourceSpecs {
		resource, err := c.client.GetResource(context.TODO(), genResourceSpec.APIVersion, genResourceSpec.Kind, genResourceSpec.Namespace, genResourceSpec.Name)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (c *GenerateController) needsReports(trigger unstructured.Unstructured) bool {
	createReport := c.reportsConfig.GenerateReportsEnabled()
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(trigger.GroupVersionKind()) {
		createReport = false
	}

	return createReport
}

func (c *GenerateController) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	engineResponses ...engineapi.EngineResponse,
) error {
	report := reportutils.BuildGenerateReport(resource.GetNamespace(), resource.GroupVersionKind(), resource.GetName(), resource.GetUID(), engineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, c.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func updateStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec) error {
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

func GetUnstrRule(rule *kyvernov1.Generation) (*unstructured.Unstructured, error) {
	ruleData, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return kubeutils.BytesToUnstructured(ruleData)
}
