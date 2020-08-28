package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/utils"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"os"
	"reflect"

	"github.com/nirmata/kyverno/pkg/engine/context"

	"strings"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"

	"github.com/nirmata/kyverno/pkg/kyverno/common"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nirmata/kyverno/pkg/engine"

	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	var namespace, kubeconfig string
	var cluster bool
	type Resource struct {
		Name   string            `json:"name"`
		Values map[string]string `json:"values"`
	}

	type Policy struct {
		Name      string     `json:"name"`
		Resources []Resource `json:"resources"`
	}

	type Values struct {
		Policies []Policy `json:"policies"`
	}

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	cmd = &cobra.Command{
		Use:     "report",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {

			var dClient *client.Client
			var kclient *kyvernoclient.Clientset

			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}

			dClient, err = client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
			if err != nil {
				os.Exit(1)
			}

			kclient, err = kyvernoclient.NewForConfig(restConfig)
			if err != nil {
				os.Exit(1)
			}
			kubeClient, err := utils.NewKubeClient(restConfig)
			if err != nil {
				setupLog.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}
			const resyncPeriod = 15 * time.Minute

			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)

			configData := config.NewConfigData(
				kubeClient,
				kubeInformer.Core().V1().ConfigMaps(),
				"",
				"",
				"",
				log.Log.WithName("ConfigData"),
			)

			ns, err := dClient.ListResource("", "Namespace", "", &kyvernov1.LabelSelector{})
			if err != nil {
				os.Exit(1)
			}
			var engineResponses []response.EngineResponse
			for _, n := range ns.Items {
				policies, err := kclient.KyvernoV1().Policies(n.GetName()).List(kyvernov1.ListOption{})
				if err != nil {
					os.Exit(1)
				}
				// key uid
				resourceMap := map[string]unstructured.Unstructured{}
				for _, p := range policies.Items {
					for _, rule := range p.Spec.Rules {
						for _, k := range rule.MatchResources.Kinds {
							resourceSchema, _, err := dClient.DiscoveryClient.FindResource("", k)
							if err != nil {
								log.Log.Error(err, "failed to find resource", "kind", k)
								continue
							}

							if !resourceSchema.Namespaced && os.Getenv("SCOPE") == "CLUSTER" {
								rMap := getResourcesPerNamespace(k, dClient, "", rule, configData, log.Log)
								mergeResources(resourceMap, rMap)
							} else if resourceSchema.Namespaced && os.Getenv("SCOPE") == "NAMESPACE" {
								namespaces := getNamespacesForRule(&rule, kubeInformer.Core().V1().Namespaces().Lister(), log.Log)
								for _, ns := range namespaces {
									rMap := getResourcesPerNamespace(k, dClient, ns, rule, configData, log.Log)
									mergeResources(resourceMap, rMap)
								}
							}
						}
					}
					if p.HasAutoGenAnnotation() {
						resourceMap = excludePod(resourceMap, log.Log)
					}
					pol := policy.ConvertPolicyToClusterPolicy(&p)
					for _, resource := range resourceMap {
						policyContext := engine.PolicyContext{
							NewResource:      resource,
							OldResource:      nil,
							Context:          context.Background(),
							Policy:           *pol,
							ExcludeGroupRole: configData.GetExcludeGroupRole(),
						}
						engineResponse := engine.Validate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}

						engineResponse = engine.Mutate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}

						engineResponse = engine.Generate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}
					}

				}

			}
			cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace")
			cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig")
			cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
			cmd.Flags().BoolVarP(&helm, "helm", "h", false, "Checks if policies should be applied to cluster in the current context")
			return err
		},
	}
}

// merge b into a map
func mergeResources(a, b map[string]unstructured.Unstructured) {
	for k, v := range b {
		a[k] = v
	}
}

// excludePod filter out the pods with ownerReference
func excludePod(resourceMap map[string]unstructured.Unstructured, log logr.Logger) map[string]unstructured.Unstructured {
	for uid, r := range resourceMap {
		if r.GetKind() != "Pod" {
			continue
		}

		if len(r.GetOwnerReferences()) > 0 {
			log.V(4).Info("exclude Pod", "namespace", r.GetNamespace(), "name", r.GetName())
			delete(resourceMap, uid)
		}
	}

	return resourceMap
}

func getNamespacesForRule(rule *kyverno.Rule, nslister listerv1.NamespaceLister, log logr.Logger) []string {
	if len(rule.MatchResources.Namespaces) == 0 {
		return getAllNamespaces(nslister, log)
	}

	var wildcards []string
	var results []string
	for _, nsName := range rule.MatchResources.Namespaces {
		if hasWildcard(nsName) {
			wildcards = append(wildcards, nsName)
		}

		results = append(results, nsName)
	}

	if len(wildcards) > 0 {
		wildcardMatches := getMatchingNamespaces(wildcards, nslister, log)
		results = append(results, wildcardMatches...)
	}

	return results
}

func hasWildcard(s string) bool {
	if s == "" {
		return false
	}

	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

func getMatchingNamespaces(wildcards []string, nslister listerv1.NamespaceLister, log logr.Logger) []string {
	all := getAllNamespaces(nslister, log)
	if len(all) == 0 {
		return all
	}

	var results []string
	for _, wc := range wildcards {
		for _, ns := range all {
			if wildcard.Match(wc, ns) {
				results = append(results, ns)
			}
		}
	}

	return results
}

func getAllNamespaces(nslister listerv1.NamespaceLister, log logr.Logger) []string {
	var results []string
	namespaces, err := nslister.List(labels.NewSelector())
	if err != nil {
		log.Error(err, "Failed to list namespaces")
	}

	for _, n := range namespaces {
		name := n.GetName()
		results = append(results, name)
	}

	return results
}

func getResourcesPerNamespace(kind string, client *client.Client, namespace string, rule kyverno.Rule, configHandler config.Interface, log logr.Logger) map[string]unstructured.Unstructured {
	resourceMap := map[string]unstructured.Unstructured{}
	ls := rule.MatchResources.Selector

	if kind == "Namespace" {
		namespace = ""
	}

	list, err := client.ListResource("", kind, namespace, ls)
	if err != nil {
		log.Error(err, "failed to list resources", "kind", kind, "namespace", namespace)
		return nil
	}
	// filter based on name
	for _, r := range list.Items {
		if r.GetDeletionTimestamp() != nil {
			continue
		}

		if r.GetKind() == "Pod" {
			if !isRunningPod(r) {
				continue
			}
		}

		// match name
		if rule.MatchResources.Name != "" {
			if !wildcard.Match(rule.MatchResources.Name, r.GetName()) {
				continue
			}
		}
		// Skip the filtered resources
		if configHandler.ToFilter(r.GetKind(), r.GetNamespace(), r.GetName()) {
			continue
		}

		//TODO check if the group version kind is present or not
		resourceMap[string(r.GetUID())] = r
	}

	// exclude the resources
	// skip resources to be filtered
	excludeResources(resourceMap, rule.ExcludeResources.ResourceDescription, configHandler, log)
	return resourceMap
}

func excludeResources(included map[string]unstructured.Unstructured, exclude kyverno.ResourceDescription, configHandler config.Interface, log logr.Logger) {
	if reflect.DeepEqual(exclude, (kyverno.ResourceDescription{})) {
		return
	}
	excludeName := func(name string) Condition {
		if exclude.Name == "" {
			return NotEvaluate
		}
		if wildcard.Match(exclude.Name, name) {
			return Skip
		}
		return Process
	}

	excludeNamespace := func(namespace string) Condition {
		if len(exclude.Namespaces) == 0 {
			return NotEvaluate
		}
		if utils.ContainsNamepace(exclude.Namespaces, namespace) {
			return Skip
		}
		return Process
	}

	excludeSelector := func(labelsMap map[string]string) Condition {
		if exclude.Selector == nil {
			return NotEvaluate
		}
		selector, err := metav1.LabelSelectorAsSelector(exclude.Selector)
		// if the label selector is incorrect, should be fail or
		if err != nil {
			log.Error(err, "failed to build label selector")
			return Skip
		}
		if selector.Matches(labels.Set(labelsMap)) {
			return Skip
		}
		return Process
	}

	findKind := func(kind string, kinds []string) bool {
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}

	excludeKind := func(kind string) Condition {
		if len(exclude.Kinds) == 0 {
			return NotEvaluate
		}

		if findKind(kind, exclude.Kinds) {
			return Skip
		}

		return Process
	}

	// check exclude condition for each resource
	for uid, resource := range included {
		// 0 -> dont check
		// 1 -> is not to be exclude
		// 2 -> to be exclude
		excludeEval := []Condition{}

		if ret := excludeName(resource.GetName()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeNamespace(resource.GetNamespace()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeSelector(resource.GetLabels()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeKind(resource.GetKind()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		// exclude the filtered resources
		if configHandler.ToFilter(resource.GetKind(), resource.GetNamespace(), resource.GetName()) {
			delete(included, uid)
			continue
		}

		func() bool {
			for _, ret := range excludeEval {
				if ret == Process {
					// Process the resources
					continue
				}
			}
			// Skip the resource from processing
			delete(included, uid)
			return false
		}()
	}
}
