package backwardcompatibility

import (
	"context"
	"fmt"
	"strings"
	"time"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AddLabels - adds labels to all the existing generate requests
func AddLabels(client *kyvernoclient.Clientset, grInformer kyvernoinformer.GenerateRequestInformer) {
	// Get all the GR's that are existing
	// Extract and Update all of them with the with the labels
	grList, err := grInformer.Lister().List(labels.NewSelector())
	if err != nil {
		log.Log.Error(err, "failed to get generate request list")
		return
	}

	for _, gr := range grList {

		grLabels := gr.Labels
		if len(grLabels) == 0 {
			grLabels = make(map[string]string)
		}
		grLabels["generate.kyverno.io/policy-name"] = gr.Spec.Policy
		grLabels["generate.kyverno.io/resource-name"] = gr.Spec.Resource.Name
		grLabels["generate.kyverno.io/resource-kind"] = gr.Spec.Resource.Kind
		grLabels["generate.kyverno.io/resource-namespace"] = gr.Spec.Resource.Namespace

		gr.SetLabels(grLabels)

		_, err = client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(context.TODO(), gr, metav1.UpdateOptions{})
		if err != nil {
			log.Log.V(4).Info(fmt.Sprintf("failed to update the GR %v. error: %v", gr.Name, err))
			for n := 0; n <= 3; n++ {
				log.Log.V(4).Info(fmt.Sprintf("retrying to get GR %v", gr.Name))
				time.Sleep(100 * time.Millisecond)
				errInGettingGR := addLabelForGR(gr.Name, gr.Namespace, client, grInformer)
				if errInGettingGR != nil {
					continue
				} else {
					break
				}
			}
		}
	}
}

func addLabelForGR(name string, namespace string, client *kyvernoclient.Clientset, grInformer kyvernoinformer.GenerateRequestInformer) error {
	gr, err := grInformer.Lister().GenerateRequests(namespace).Get(name)
	if err != nil {
		log.Log.Error(err, fmt.Sprintf("failed to update the GR %v", name))
		return err
	}

	grLabels := gr.Labels
	if len(grLabels) == 0 {
		grLabels = make(map[string]string)
	}
	grLabels["generate.kyverno.io/policy-name"] = gr.Spec.Policy
	grLabels["generate.kyverno.io/resource-name"] = gr.Spec.Resource.Name
	grLabels["generate.kyverno.io/resource-kind"] = gr.Spec.Resource.Kind
	grLabels["generate.kyverno.io/resource-namespace"] = gr.Spec.Resource.Namespace

	gr.SetLabels(grLabels)

	_, err = client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(context.TODO(), gr, metav1.UpdateOptions{})
	if err != nil {
		log.Log.Error(err, fmt.Sprintf("failed to update the GR %v", gr.Name))
		return err
	}

	return nil
}

// AddCloneLabel - add label to the source resource about the new clone
func AddCloneLabel(client *dclient.Client, pInformer kyvernoinformer.ClusterPolicyInformer) {
	// Get all the Generate Policies which has clone
	// Get the resource with Kind, NameSpace, Name
	// Add Policy name if label not found
	policies, err := pInformer.Lister().List(labels.NewSelector())
	if err != nil {
		log.Log.Error(err, "failed to get policies")
		return
	}

	for _, policy := range policies {
		for _, rule := range policy.Spec.Rules {
			if rule.HasGenerate() {
				clone := rule.Generation.Clone
				if clone.Name != "" {
					namespace := clone.Namespace
					name := clone.Name
					kind := rule.Generation.Kind

					obj, err := client.GetResource("", kind, namespace, name)
					if err != nil {
						log.Log.Error(err, fmt.Sprintf("source not found  name:%v namespace:%v kind:%v", name, namespace, kind))
						continue
					}
					updateSource := true

					// add label
					label := obj.GetLabels()
					if len(label) == 0 {
						label = make(map[string]string)
						label["generate.kyverno.io/clone-policy-name"] = policy.GetName()
					} else {
						if label["generate.kyverno.io/clone-policy-name"] != "" {
							policyNames := label["generate.kyverno.io/clone-policy-name"]
							if !strings.Contains(policyNames, policy.GetName()) {
								policyNames = policyNames + "," + policy.GetName()
								label["generate.kyverno.io/clone-policy-name"] = policyNames
							} else {
								updateSource = false
							}
						} else {
							label["generate.kyverno.io/clone-policy-name"] = policy.GetName()
						}
					}

					if updateSource {
						log.Log.V(4).Info("updating existing clone source")
						obj.SetLabels(label)
						_, err = client.UpdateResource(obj.GetAPIVersion(), kind, namespace, obj, false)
						if err != nil {
							log.Log.Error(err, fmt.Sprintf("failed to update source  name:%v namespace:%v kind:%v\n", obj.GetName(), obj.GetNamespace(), obj.GetKind()))
							return
						}
						log.Log.V(4).Info(fmt.Sprintf("updated source  name:%v namespace:%v kind:%v\n", obj.GetName(), obj.GetNamespace(), obj.GetKind()))
					}
				}
			}
		}
	}
}
