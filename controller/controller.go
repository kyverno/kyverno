package controller

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	internalinterfaces "github.com/nirmata/kube-policy/controller/internalinterfaces"
	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	clientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	policies "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/typed/policy/v1alpha1"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	eventinternalinterfaces "github.com/nirmata/kube-policy/pkg/event/internalinterfaces"
	eventutils "github.com/nirmata/kube-policy/pkg/event/utils"
	violation "github.com/nirmata/kube-policy/pkg/violation"
	violationinternalinterfaces "github.com/nirmata/kube-policy/pkg/violation/internalinterfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	mergetypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// PolicyController API
type PolicyController interface {
	internalinterfaces.PolicyGetter
	createPolicyHandler(resource interface{})
	updatePolicyHandler(oldResource, newResource interface{})
	deletePolicyHandler(resource interface{})
	getResourceKey(resource interface{}) string
}

//policyController for CRD
type policyController struct {
	policyInformerFactory informers.SharedInformerFactory
	policyLister          lister.PolicyLister
	policiesInterface     policies.PolicyInterface
	logger                *log.Logger
	violationBuilder      violationinternalinterfaces.ViolationGenerator
	eventBuilder          eventinternalinterfaces.BuilderInternal
}

// NewPolicyController from cmd args
func NewPolicyController(config *rest.Config, logger *log.Logger, kubeClient *kubeClient.KubeClient) (PolicyController, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "Policy Controller: ", log.LstdFlags|log.Lshortfile)
	}

	if config == nil {
		return nil, errors.New("Client Config should be set for controller")
	}

	policyClientset, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, 0)
	policyInformer := policyInformerFactory.Nirmata().V1alpha1().Policies()

	// generate Event builder
	eventBuilder, err := event.NewEventBuilder(kubeClient, logger)
	if err != nil {
		return nil, err
	}

	// generate Violation builer
	violationBuilder, err := violation.NewViolationBuilder(kubeClient, eventBuilder, logger)

	controller := &policyController{
		policyInformerFactory: policyInformerFactory,
		policyLister:          policyInformer.Lister(),
		policiesInterface:     policyClientset.NirmataV1alpha1().Policies("default"),
		logger:                logger,
		violationBuilder:      violationBuilder,
		eventBuilder:          eventBuilder,
	}
	policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.createPolicyHandler,
		UpdateFunc: controller.updatePolicyHandler,
		DeleteFunc: controller.deletePolicyHandler,
	})
	// Set the controller
	eventBuilder.SetController(controller)
	violationBuilder.SetController(controller)
	return controller, nil
}

func (c *policyController) GetCacheInformerSync() cache.InformerSynced {
	return c.policyInformerFactory.Nirmata().V1alpha1().Policies().Informer().HasSynced
}

// Run is main controller thread
func (c *policyController) Run(stopCh <-chan struct{}) {
	c.policyInformerFactory.Start(stopCh)
	c.eventBuilder.Run(eventutils.EventWorkerThreadCount, stopCh)
}

func (c *policyController) GetPolicies() ([]types.Policy, error) {
	// Create nil Selector to grab all the policies
	selector := labels.NewSelector()
	cachedPolicies, err := c.policyLister.List(selector)
	if err != nil {
		c.logger.Printf("Error: %v", err)
		return nil, err
	}

	var policies []types.Policy
	for _, elem := range cachedPolicies {
		policies = append(policies, *elem.DeepCopy())
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreationTimestamp.Time.Before(policies[j].CreationTimestamp.Time)
	})
	return policies, nil
}

// Writes error message to the policy logs in status section
func (c *policyController) LogPolicyError(name, text string) {
	c.addPolicyLog(name, "[ERROR] "+text)
}

// Writes info message to the policy logs in status section
func (c *policyController) LogPolicyInfo(name, text string) {
	c.addPolicyLog(name, "[ INFO] "+text)
}

// This is the maximum number of records that can be written to the log object of the policy.
// If this number is exceeded, the older entries will be deleted.
const policyLogMaxRecords int = 50

// Appends given log text to the status/logs array.
func (c *policyController) addPolicyLog(name, text string) {
	getOptions := metav1.GetOptions{
		ResourceVersion:      "1",
		IncludeUninitialized: true,
	}
	policy, err := c.policiesInterface.Get(name, getOptions)
	if err != nil {
		c.logger.Printf("Unable to get policy %s: %s", name, err)
		return
	}

	// Add new log record
	text = time.Now().Format("2006 Jan 02 15:04:05.999 ") + text
	policy.Status.Logs = append(policy.Status.Logs, text)
	//	Pop front extra log records
	logsCount := len(policy.Status.Logs)
	if logsCount > policyLogMaxRecords {
		policy.Status.Logs = policy.Status.Logs[logsCount-policyLogMaxRecords:]
	}
	// Save logs to policy object
	_, err = c.policiesInterface.UpdateStatus(policy)
	if err != nil {
		c.logger.Printf("Unable to update logs for policy %s: %s", name, err)
	}
}

func (c *policyController) createPolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Policy created: %s", key)
}

func (c *policyController) updatePolicyHandler(oldResource, newResource interface{}) {
	oldKey := c.getResourceKey(oldResource)
	newKey := c.getResourceKey(newResource)
	c.logger.Printf("Policy %s updated to %s", oldKey, newKey)
}

func (c *policyController) deletePolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Policy deleted: %s", key)
}

func (c *policyController) getResourceKey(resource interface{}) string {
	if key, err := cache.MetaNamespaceKeyFunc(resource); err != nil {
		c.logger.Fatalf("Error retrieving policy key: %v", err)
	} else {
		return key
	}
	return ""
}
func (c *policyController) GetPolicy(name string) (*types.Policy, error) {
	policyNamespace, policyName, err := cache.SplitMetaNamespaceKey(name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", name))
		return nil, err
	}
	return c.getPolicyInterface(policyNamespace).Get(policyName)
}

func (c *policyController) getPolicyInterface(namespace string) lister.PolicyNamespaceLister {
	return c.policyLister.Policies(namespace)
}

func (c *policyController) PatchPolicy(policy string, pt mergetypes.PatchType, data []byte) (*types.Policy, error) {
	return c.policiesInterface.Patch(policy, pt, data)
}
