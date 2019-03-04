package controller

import (
	"errors"
	"log"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	clientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
)

// PolicyController for CRD
type PolicyController struct {
	policyInformerFactory informers.SharedInformerFactory
	policyLister          lister.PolicyLister
	logger                *log.Logger
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

	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, time.Second*30)
	policyInformer := policyInformerFactory.Nirmata().V1alpha1().Policies()

	controller := &PolicyController{
		policyInformerFactory: policyInformerFactory,
		policyLister:          policyInformer.Lister(),
		logger:                logger,
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

	return policies
}

func (c *PolicyController) createPolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Created policy: %s\n", key)
}

func (c *PolicyController) updatePolicyHandler(oldResource, newResource interface{}) {
	oldKey := c.getResourceKey(oldResource)
	newKey := c.getResourceKey(newResource)

	c.logger.Printf("Updated policy from %s to %s\n", oldKey, newKey)
}

func (c *PolicyController) deletePolicyHandler(resource interface{}) {
	key := c.getResourceKey(resource)
	c.logger.Printf("Deleted policy: %s\n", key)
}

func (c *PolicyController) getResourceKey(resource interface{}) string {
	if key, err := cache.MetaNamespaceKeyFunc(resource); err != nil {
		c.logger.Fatalf("Error retrieving policy key: %v\n", err)
	} else {
		return key
	}

	return ""
}
