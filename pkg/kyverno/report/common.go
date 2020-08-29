package report

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/daemon/logger"
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
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"os"
	"reflect"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
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
		var reports map[string][]kyvernov1.PolicyViolationTemplate
		var policyreports map[string]policyreportv1alpha1.PolicyReport
		var clusterPolicyreports map[string]policyreportv1alpha1.ClusterPolicyReport
		var results map[string][]policyreportv1alpha1.PolicyReportResult
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
			if *policyContext.Policy.Spec.Background {
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
			pv := policyreport.GeneratePRsFromEngineResponse(engineResponses, log.Log)

			for _, v := range pv {
				var appname string
				switch os.Getenv("SCOPE") {
				case "HELM":
					//TODO GET Labels
					resource, err := dClient.GetResource(v.Resource.GetAPIVersion(), v.Resource.GetKind(), v.Resource.GetNamespace(), v.Resource.GetName())
					if err != nil {
						log.Log.Error(err, "failed to get resource")
						continue
					}
					labels := resource.GetLabels()
					_, okChart := labels["app"]
					_, okRelease := labels["release"]
					if okChart && okRelease {
						appname = fmt.Sprintf("kyverno-policyreport-%s-%s", labels["app"], policyContext.NewResource.GetNamespace())
					}
					break
				case "NAMESPACE":
					appname = fmt.Sprintf("kyverno-policyreport-%s", policyContext.NewResource.GetNamespace())
					resourceMap = resourceNamespaceMap
					break
				case "CLUSTER":
					appname = fmt.Sprintf("kyverno-clusterpolicyreport")

					break
				}
				builder := policyreport.NewPrBuilder()
				pv := builder.Generate(v)
				reports[appname] = append(reports[appname], pv)

				if _, ok := clusterPolicyreports[appname]; !ok {
					if os.Getenv("SCOPE") == "CLUSTER" {
						availablepr, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get(appname, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								availablepr = &policyreportv1alpha1.ClusterPolicyReport{
									Scope: &corev1.ObjectReference{
										Kind: "Cluster",
									},
									Summary: policyreportv1alpha1.PolicyReportSummary{},
									Results: []*policyreportv1alpha1.PolicyReportResult{},
								}
								labelMap := map[string]string{
									"policy-scope": "cluster",
								}
								availablepr.SetName(appname)
								availablepr.SetLabels(labelMap)
							}
						}
						clusterPolicyreports[appname] = *availablepr
					}
				}
				if _, ok := policyreports[appname]; !ok {
					if os.Getenv("SCOPE") == "NAMESPACE" {
						availablepr, err := kclient.PolicyV1alpha1().PolicyReports(v.Resource.GetNamespace()).Get(appname, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								availablepr = &policyreportv1alpha1.PolicyReport{
									Scope: &corev1.ObjectReference{
										Kind: "Cluster",
									},
									Summary: policyreportv1alpha1.PolicyReportSummary{},
									Results: []*policyreportv1alpha1.PolicyReportResult{},
								}
								labelMap := map[string]string{
									"policy-scope": "namespace",
								}
								availablepr.SetName(appname)
								availablepr.SetLabels(labelMap)
							}
						}
						policyreports[appname] = *availablepr
					} else {
						availablepr, err := kclient.PolicyV1alpha1().PolicyReports(v.Resource.GetNamespace()).Get(appname, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								availablepr = &policyreportv1alpha1.PolicyReport{
									Scope: &corev1.ObjectReference{
										Kind: "Helm",
									},
									Summary: policyreportv1alpha1.PolicyReportSummary{},
									Results: []*policyreportv1alpha1.PolicyReportResult{},
								}
								labelMap := map[string]string{
									"policy-scope": "helm",
								}
								availablepr.SetName(appname)
								availablepr.SetLabels(labelMap)
							}
						}
						policyreports[appname] = *availablepr
					}
				}

			}

		}
		if os.Getenv("SCOPE") == "HELM" || os.Getenv("SCOPE") == "NAMESPACE" {
			for appname, _ := range policyreports {
				if len(policyreports[appname].Results) > 0 {
					for _, v := range reports[appname] {
						for j, events := range policyreports[appname].Results {
							for k, violation := range v.Spec.ViolatedRules {
								if events.Policy == v.Spec.Policy && events.Rule == violation.Name && v.Spec.APIVersion == events.Resource.APIVersion && v.Spec.Kind == events.Resource.Kind && v.Spec.Namespace == events.Resource.Namespace && v.Spec.Name == events.Resource.Name {
									if violation.Check != string(events.Status) {
										events.Message = violation.Message
									}
									events.Data["status"] = "scan"
									if len(v.Spec.ViolatedRules) > 1 {
										v.Spec.ViolatedRules = append(v.Spec.ViolatedRules[:k], v.Spec.ViolatedRules[k+1:]...)
										continue
									} else if len(v.Spec.ViolatedRules) == 1 {
										v.Spec.ViolatedRules = []kyvernov1.ViolatedRule{}
									}
								}
							}
						}
						for _, e := range v.Spec.ViolatedRules {
							result := &policyreportv1alpha1.PolicyReportResult{
								Policy:  v.Spec.Policy,
								Rule:    e.Name,
								Message: e.Message,
								Status:  policyreportv1alpha1.PolicyStatus(e.Check),
								Resource: &corev1.ObjectReference{
									Kind:       v.Spec.Kind,
									Namespace:  v.Spec.Namespace,
									APIVersion: v.Spec.APIVersion,
									Name:       v.Spec.Name,
								},
							}
							result.Data["status"] = "scan"
							policyreports[appname].Results = append(policyreports[appname].Results, result)
						}

					}
				}
				str := strings.Split(appname, "-")
				var ns string
				if len(str) == 2 {
					ns = str[1]
				} else if len(str) == 3 {
					ns = str[2]
				}
				_, err := kclient.PolicyV1alpha1().PolicyReports(ns).Get(appname, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err := kclient.PolicyV1alpha1().PolicyReports(ns).Update(&policyreports[appname])
						if err != nil {
							log.Log.Error(err, "Error in update polciy report")
						}
					}
				} else {
					_, err := kclient.PolicyV1alpha1().PolicyReports(ns).Create(&policyreports[appname])
					if err != nil {
						log.Log.Error(err, "Error in create polciy report")
					}
				}
			}

		} else {
			for appname, _ := range clusterPolicyreports {
				if len(clusterPolicyreports[appname].Results) > 0 {
					for _, v := range reports[appname] {
						for j, events := range clusterPolicyreports[appname].Results {
							for k, violation := range v.Spec.ViolatedRules {
								if events.Policy == v.Spec.Policy && events.Rule == violation.Name && v.Spec.APIVersion == events.Resource.APIVersion && v.Spec.Kind == events.Resource.Kind && v.Spec.Namespace == events.Resource.Namespace && v.Spec.Name == events.Resource.Name {
									if violation.Check != string(events.Status) {
										events.Message = violation.Message
									}
									events.Data["status"] = "scan"
									if len(v.Spec.ViolatedRules) > 1 {
										v.Spec.ViolatedRules = append(v.Spec.ViolatedRules[:k], v.Spec.ViolatedRules[k+1:]...)
										continue
									} else if len(v.Spec.ViolatedRules) == 1 {
										v.Spec.ViolatedRules = []kyvernov1.ViolatedRule{}
									}
								}
							}
						}
						for _, e := range v.Spec.ViolatedRules {
							result := &policyreportv1alpha1.PolicyReportResult{
								Policy:  v.Spec.Policy,
								Rule:    e.Name,
								Message: e.Message,
								Status:  policyreportv1alpha1.PolicyStatus(e.Check),
								Resource: &corev1.ObjectReference{
									Kind:       v.Spec.Kind,
									Namespace:  v.Spec.Namespace,
									APIVersion: v.Spec.APIVersion,
									Name:       v.Spec.Name,
								},
							}
							result.Data["status"] = "scan"
							clusterPolicyreports[appname].Results = append(clusterPolicyreports[appname].Results, result)
						}
					}
				}
				_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get(appname, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Update(&clusterPolicyreports[appname])
						if err != nil {
							log.Log.Error(err, "Error in update polciy report")
						}
					}
				} else {
					_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Create(&clusterPolicyreports[appname])
					if err != nil {
						log.Log.Error(err, "Error in create polciy report")
					}
				}
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

	var appNames map[string][]policyreportv1alpha1.PolicyReportResult
	var clusterReport []policyreportv1alpha1.PolicyReportResult
	var namespaceReport map[string][]policyreportv1alpha1.PolicyReportResult

	configmap, err := dClient.GetResource("", "Configmap", config.KubePolicyNamespace, "kyverno-event")
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
	} else if os.Getenv("SCOPE") == "HELM" {
		data = events.Helm[n]
	} else {
		data = events.Namespace[n]
	}
	type PolicyReport struct {
		Helm      map[string][]policyreportv1alpha1.PolicyReportResult
		Namespace map[string][]policyreportv1alpha1.PolicyReportResult
		Cluster   []policyreportv1alpha1.PolicyReportResult
		mux       sync.Mutex
	}
	var ns []string
	for _, v := range data {
		for _, r := range v.Rules {
			builder := policyreport.NewPrBuilder()
			pv := builder.Generate(v)
			result := &policyreportv1alpha1.PolicyReportResult{
				Policy:  pv.Spec.Policy,
				Rule:    r.Name,
				Message: r.Message,
				Status:  policyreportv1alpha1.PolicyStatus(r.Check),
				Resource: &corev1.ObjectReference{
					Kind:       pv.Spec.Kind,
					Namespace:  pv.Spec.Namespace,
					APIVersion: pv.Spec.APIVersion,
					Name:       pv.Spec.Name,
				},
			}
			if !strings.Contains(strings.Join(ns, ","), v.Resource.GetNamespace()) {
				ns = append(ns, v.Resource.GetNamespace())
			}

			// Increase Count
			if os.Getenv("SCOPE") == "CLUSTER" {
				clusterReport = append(clusterReport, *result)
			} else if os.Getenv("SCOPE") == "HELM" {
				resource, err := dClient.GetResource(v.Resource.GetAPIVersion(), v.Resource.GetKind(), v.Resource.GetNamespace(), v.Resource.GetName())
				if err != nil {
					log.Log.Error(err, "failed to get resource")
					continue
				}
				labels := resource.GetLabels()
				_, okChart := labels["app"]
				_, okRelease := labels["release"]
				if okChart && okRelease {
					appNames[fmt.Sprintf("kyverno-policyreport%s-%s", labels["app"], v.Resource.GetNamespace())] = append(appNames[fmt.Sprintf("kyverno-policyreport%s-%s", labels["app"], v.Resource.GetNamespace())], *result)
				}
			} else {
				namespaceReport[v.Resource.GetNamespace()] = append(namespaceReport[v.Resource.GetNamespace()], *result)
			}
		}
	}
	if os.Getenv("SCOPE") == "Namespace" {
		for _, k := range ns {
			isExist := true
			appName := fmt.Sprintf("kyverno-policyreport-%s", k)
			availablepr, err := kclient.PolicyV1alpha1().PolicyReports(k).Get(reportName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					isExist = false
					availablepr = &policyreportv1alpha1.PolicyReport{
						Scope: &corev1.ObjectReference{
							Kind: "Namespace",
						},
						Summary: policyreportv1alpha1.PolicyReportSummary{},
						Results: []*policyreportv1alpha1.PolicyReportResult{},
					}

					availablepr.SetNamespace(k)
					labelMap := map[string]string{
						"policy-scope": "namespace",
					}
					availablepr.SetLabels(labelMap)
					availablepr.ObjectMeta.Name = appName
				}
			}
			if len(availablepr.Results) > 0 {
				for k, _ := range namespaceReport {
					for nsReportEvent, e := range namespaceReport[k] {
						for j, events := range availablepr.Results {
							if events.Policy == e.Policy && events.Rule == e.Rule && e.Resource.APIVersion == events.Resource.APIVersion && e.Resource.Kind == events.Resource.Kind && e.Resource.Namespace == events.Resource.Namespace && e.Resource.Name == events.Resource.Name {
								if string(e.Status) != string(availablepr.Results[j].Status) {
									availablepr.Results[j].Message = e.Message
								}
								availablepr.Results[j].Data["status"] = "scan"
								if len(namespaceReport[k]) > 1 {
									namespaceReport[k] = append(namespaceReport[k][:nsReportEvent], namespaceReport[k][nsReportEvent+1:]...)
									continue
								} else if len(namespaceReport[k]) == 1 {
									namespaceReport[k] = []policyreportv1alpha1.PolicyReportResult{}
								}
							}
						}

					}
				}
			}
			for k, _ := range namespaceReport {
				for _, events := range namespaceReport[k] {
					result := &policyreportv1alpha1.PolicyReportResult{
						Policy:  events.Policy,
						Rule:    events.Rule,
						Message: events.Message,
						Status:  policyreportv1alpha1.PolicyStatus(events.Status),
						Resource: &corev1.ObjectReference{
							Kind:       events.Resource.Kind,
							Namespace:  events.Resource.Namespace,
							APIVersion: events.Resource.APIVersion,
							Name:       events.Resource.Name,
						},
					}
					result.Data["status"] = "scan"
					availablepr.Results = append(availablepr.Results, result)
				}

			}
			if isExist {
				_, err := kclient.PolicyV1alpha1().PolicyReports(n).Update(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in update polciy report")
				}
			} else {
				_, err := kclient.PolicyV1alpha1().PolicyReports(n).Create(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in create polciy report")
				}
			}
		}
	} else if os.Getenv("SCOPE") == "Helm" {
		for k, _ := range appNames {
			str := strings.Split(k, "-")
			isExist := true
			availablepr, err := kclient.PolicyV1alpha1().PolicyReports(str[2]).Get(k, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					isExist = false
					availablepr = &policyreportv1alpha1.PolicyReport{
						Scope: &corev1.ObjectReference{
							Kind: "Helm",
						},
						Summary: policyreportv1alpha1.PolicyReportSummary{},
						Results: []*policyreportv1alpha1.PolicyReportResult{},
					}
					labelMap := map[string]string{
						"policy-scope": "namespace",
					}
					availablepr.SetNamespace(str[2])
					availablepr.SetName(k)
					availablepr.SetLabels(labelMap)
				}
			}
			if len(availablepr.Results) > 0 {
				for _, e := range appNames[k] {
					for _, events := range availablepr.Results {
						if events.Policy == e.Policy && events.Rule == e.Rule && e.Resource.APIVersion == events.Resource.APIVersion && e.Resource.Kind == events.Resource.Kind && e.Resource.Namespace == events.Resource.Namespace && e.Resource.Name == events.Resource.Name {
							if string(e.Status) != string(events.Status) {
								events.Message = e.Message
							}
							events.Status = e.Status
							events.Data["status"] = "scan"
							if len(clusterReport) > 1 {
								clusterReport = append(clusterReport[:i], clusterReport[i+1:]...)
								continue
							} else if len(clusterReport) == 1 {
								clusterReport = []policyreportv1alpha1.PolicyReportResult{}
							}
						}
					}
				}
			}
			for k, _ := range appNames {
				for _, e := range appNames[k] {
					result := &policyreportv1alpha1.PolicyReportResult{
						Policy:  e.Policy,
						Rule:    e.Rule,
						Message: e.Message,
						Status:  policyreportv1alpha1.PolicyStatus(e.Status),
						Resource: &corev1.ObjectReference{
							Kind:       e.Resource.Kind,
							Namespace:  e.Resource.Namespace,
							APIVersion: e.Resource.APIVersion,
							Name:       e.Resource.Name,
						},
					}
					result.Data["status"] = "scan"
					availablepr.Results = append(availablepr.Results, result)

				}
			}
			if isExist {
				_, err := kclient.PolicyV1alpha1().PolicyReports(str[2]).Update(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in update polciy report", "appreport", k)
				}
			} else {
				_, err := kclient.PolicyV1alpha1().PolicyReports(str[2]).Create(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in create polciy report", "appreport", k)
				}
			}
		}
	} else {
		isExist := true
		availablepr, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get("kyverno-clusterpolicyreport", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				isExist = false
				availablepr = &policyreportv1alpha1.ClusterPolicyReport{
					Scope: &corev1.ObjectReference{
						Kind: "Cluster",
					},
					Summary: policyreportv1alpha1.PolicyReportSummary{},
					Results: []*policyreportv1alpha1.PolicyReportResult{},
				}
				labelMap := map[string]string{
					"policy-scope": "namespace",
				}
				availablepr.SetName("kyverno-clusterpolicyreport")
				availablepr.SetLabels(labelMap)
			}
		}
		if len(availablepr.Results) > 0 {
			for i, e := range clusterReport {
				for _, events := range availablepr.Results {
					if events.Policy == e.Policy && events.Rule == e.Rule && e.Resource.APIVersion == events.Resource.APIVersion && e.Resource.Kind == events.Resource.Kind && e.Resource.Namespace == events.Resource.Namespace && e.Resource.Name == events.Resource.Name {
						if string(e.Status) != string(events.Status) {
							events.Message = e.Message
						}
						events.Status = e.Status
						events.Data["status"] = "scan"
						if len(clusterReport) > 1 {
							clusterReport = append(clusterReport[:i], clusterReport[i+1:]...)
							continue
						} else if len(clusterReport) == 1 {
							clusterReport = []policyreportv1alpha1.PolicyReportResult{}
						}
					}
				}
			}
		}
		for _, e := range clusterReport {
			result := &policyreportv1alpha1.PolicyReportResult{
				Policy:  e.Policy,
				Rule:    e.Rule,
				Message: e.Message,
				Status:  policyreportv1alpha1.PolicyStatus(e.Status),
				Resource: &corev1.ObjectReference{
					Kind:       e.Resource.Kind,
					Namespace:  e.Resource.Namespace,
					APIVersion: e.Resource.APIVersion,
					Name:       e.Resource.Name,
				},
			}
			result.Data["status"] = "scan"
			availablepr.Results = append(availablepr.Results, result)

		}
		if isExist {
			_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Update(availablepr)
			if err != nil {
				log.Log.Error(err, "Error in update polciy report")
			}
		} else {
			_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Create(availablepr)
			if err != nil {
				log.Log.Error(err, "Error in create polciy report")
			}
		}
	}

}
