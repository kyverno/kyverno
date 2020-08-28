package report

import (
	kyvernov1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyreport"
	"github.com/nirmata/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"os"
	"encoding/json"
	"reflect"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

func createEngineRespone(n string, wg *sync.WaitGroup, restConfig *rest.Config) {
	defer func() {
		wg.Done()
	}()
	dClient, err := client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
	if err != nil {
		os.Exit(1)
	}

	kclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		os.Exit(1)
	}
	kubeClient, err := utils.NewKubeClient(restConfig)
	if err != nil {
		log.Log.Error(err, "Failed to create kubernetes client")
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
	var cpolicies *kyvernov1.ClusterPolicyList
	if os.Getenv("SCOPE") == "CLUSTER" {
		cpolicies, err = kclient.KyvernoV1().ClusterPolicies().List(metav1.ListOptions{})
		if err != nil {
			os.Exit(1)
		}
	} else {
		policies, err := kclient.KyvernoV1().Policies(n).List(metav1.ListOptions{})
		for _, p := range policies.Items {
			cp := policy.ConvertPolicyToClusterPolicy(&p)
			cpolicies.Items = append(cpolicies.Items, *cp)
		}
		if err != nil {
			os.Exit(1)
		}
	}

	// key uid
	resourceClusterMap := map[string]unstructured.Unstructured{}
	resourceNamespaceMap := map[string]unstructured.Unstructured{}
	resourceHelmMap := map[string]unstructured.Unstructured{}
	resourceMap := map[string]unstructured.Unstructured{}
	var engineResponses []response.EngineResponse
	for _, p := range cpolicies.Items {
		for _, rule := range p.Spec.Rules {
			for _, k := range rule.MatchResources.Kinds {
				resourceSchema, _, err := dClient.DiscoveryClient.FindResource("", k)
				if err != nil {
					log.Log.Error(err, "failed to find resource", "kind", k)
					continue
				}

				if !resourceSchema.Namespaced && os.Getenv("SCOPE") == "CLUSTER" {
					rMap := policy.GetResourcesPerNamespace(k, dClient, "", rule, configData, log.Log)
					policy.MergeResources(resourceClusterMap, rMap)
				} else if resourceSchema.Namespaced {
					namespaces := policy.GetNamespacesForRule(&rule, kubeInformer.Core().V1().Namespaces().Lister(), log.Log)
					for _, ns := range namespaces {
						if ns == n {
							rMap := policy.GetResourcesPerNamespace(k, dClient, ns, rule, configData, log.Log)
							for _, r := range rMap {
								labels := r.GetLabels()
								_, okChart := labels["app"]
								_, okRelease := labels["release"]
								if okChart && okRelease && os.Getenv("SCOPE") == "HELM" {
									policy.MergeResources(resourceHelmMap, rMap)
								} else if os.Getenv("SCOPE") == "NAMESPACE" {
									policy.MergeResources(resourceNamespaceMap, rMap)
								}
							}
						}
					}
				}
			}
		}
		switch os.Getenv("SCOPE") {
		case "HELM":
			resourceMap = resourceHelmMap
			break
		case "NAMESPACE":
			resourceMap = resourceNamespaceMap
			break
		case "CLUSTER":
			resourceMap = resourceClusterMap
			break
		}
		if p.HasAutoGenAnnotation() {
			resourceMap = policy.ExcludePod(resourceMap, log.Log)
		}
		for _, resource := range resourceMap {
			policyContext := engine.PolicyContext{
				NewResource:      resource,
				Context:          context.NewContext(),
				Policy:           p,
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
	// Create Policy Report
}

func createEngineResponse(n string, wg *sync.WaitGroup, restConfig *rest.Config) {
	defer func() {
		wg.Done()
	}()
	dClient, err := client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
	if err != nil {
		os.Exit(1)
	}

	kclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		os.Exit(1)
	}

	kubeClient, err := utils.NewKubeClient(restConfig)
	if err != nil {
		log.Log.Error(err, "Failed to create kubernetes client")
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

	configmap, err := dClient.GetResource("","Configmap",config.KubePolicyNamespace,"kyverno-event");
	if err != nil {
		os.Exit(1)
	}

	genData, _, err := unstructured.NestedMap(configmap.Object, "data")
	if err != nil {
		os.Exit(1)
	}
	jsonString, _ := json.Marshal(genData)
	events := policyreport.PVEvent{}
	json.Unmarshal(jsonString, &events)
	var data []policyreport.Info
	var reportName string
	if os.Getenv("SCOPE") == "CLUSTER" {
		reportName = fmt.Sprintf("kyverno-clusterpolicyreport")
		data = events.Cluster
	}else if os.Getenv("SCOPE") == "HELM" {
		data = events.Helm[n]
	}else{
		data = events.Namespace[n]
	}
	type PolicyReport struct {
		Helm  map[string]policyreportv1alpha1.PolicyReport
		Namespace map[string]policyreportv1alpha1.PolicyReport
		Cluster policyreportv1alpha1.ClusterPolicyReport
	}

	for _,v := range data {

	}

	pr, err := hpr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Get(reportName, v1.GetOptions{})
}
