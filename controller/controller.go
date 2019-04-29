package controller

import (
	"errors"
	"log"
	"os"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	clientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	policies "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/typed/policy/v1alpha1"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	violation "github.com/nirmata/kube-policy/pkg/violation"
)

// PolicyController for CRD
type PolicyController struct {
	policyInformerFactory informers.SharedInformerFactory
	policyLister          lister.PolicyLister
	policiesInterface     policies.PolicyInterface
	logger                *log.Logger
	violationBuilder      *violation.Builder
}

// NewPolicyController from cmd args
func NewPolicyController(config *rest.Config, logger *log.Logger) (*PolicyController, error) {
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
	//	Initialize Kube Client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Print(kubeClient)

	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, time.Second*30)
	policyInformer := policyInformerFactory.Nirmata().V1alpha1().Policies()

	// generate Violation builder
	builder, err := violation.NewViolationHelper(kubeClient, policyClientset, logger, policyInformer)
	if err != nil {
		return nil, err
	}
	controller := &PolicyController{
		policyInformerFactory: policyInformerFactory,
		policyLister:          policyInformer.Lister(),
		policiesInterface:     policyClientset.NirmataV1alpha1().Policies("default"),
		logger:                logger,
		violationBuilder:      builder,
	}

	policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.createPolicyHandler,
		UpdateFunc: controller.updatePolicyHandler,
		DeleteFunc: controller.deletePolicyHandler,
	})

	return controller, nil
}

// Run is main controller thread
func (c *PolicyController) Run(stopCh <-chan struct{}) {
	c.policyInformerFactory.Start(stopCh)
	// Un-comment to run the violation Builder
	//	c.violationBuilder.Run(1, stopCh)
}

// GetPolicies retrieves all policy resources
// from cache. Cache is refreshed by informer
func (c *PolicyController) GetPolicies() []types.Policy {
	// Create nil Selector to grab all the policies
	selector := labels.NewSelector()
	cachedPolicies, err := c.policyLister.List(selector)

	if err != nil {
		c.logger.Printf("Error: %v", err)
		return nil
	}

	var policies []types.Policy
	for _, elem := range cachedPolicies {
		policies = append(policies, *elem.DeepCopy())
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreationTimestamp.Time.Before(policies[j].CreationTimestamp.Time)
	})

	return policies
}

// Writes error message to the policy logs in status section
func (c *PolicyController) LogPolicyError(name, text string) {
	c.addPolicyLog(name, "[ERROR] "+text)
}

// Writes info message to the policy logs in status section
func (c *PolicyController) LogPolicyInfo(name, text string) {
	c.addPolicyLog(name, "[ INFO] "+text)
}

// This is the maximum number of records that can be written to the log object of the policy.
// If this number is exceeded, the older entries will be deleted.
const policyLogMaxRecords int = 50

// Appends given log text to the status/logs array.
func (c *PolicyController) addPolicyLog(name, text string) {
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
	// Pop front extra log records
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

func (c *PolicyController) createPolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Policy created: %s", key)
}

func (c *PolicyController) updatePolicyHandler(oldResource, newResource interface{}) {
	oldKey := c.getResourceKey(oldResource)
	newKey := c.getResourceKey(newResource)

	c.logger.Printf("Policy %s updated to %s", oldKey, newKey)
}

func (c *PolicyController) deletePolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Policy deleted: %s", key)
}

func (c *PolicyController) getResourceKey(resource interface{}) string {
	if key, err := cache.MetaNamespaceKeyFunc(resource); err != nil {
		c.logger.Fatalf("Error retrieving policy key: %v", err)
	} else {
		return key
	}

	return ""
}
