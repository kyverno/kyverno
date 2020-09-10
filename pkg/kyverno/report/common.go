package report

import (
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyreport"
	"github.com/nirmata/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"os"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	Helm      string = "Helm"
	Namespace string = "Namespace"
	Cluster   string = "Cluster"
)

func backgroundScan(n, scope,policychange string, wg *sync.WaitGroup, restConfig *rest.Config) {
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
		log.Log.Error(err, "Failed to Create kubernetes client")
		os.Exit(1)
	}
	pclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		os.Exit(1)
	}
	var stopCh <-chan struct{}
	const resyncPeriod = 15 * time.Minute

	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, resyncPeriod)
	ci := kubeInformer.Core().V1().ConfigMaps()
	pi := pInformer.Kyverno().V1().Policies()
	np := kubeInformer.Core().V1().Namespaces()

	go np.Informer().Run(stopCh)

	nSynced := np.Informer().HasSynced

	cpi := pInformer.Kyverno().V1().ClusterPolicies()
	go ci.Informer().Run(stopCh)
	go pi.Informer().Run(stopCh)
	go cpi.Informer().Run(stopCh)
	cSynced := ci.Informer().HasSynced
	piSynced := pi.Informer().HasSynced
	cpiSynced := cpi.Informer().HasSynced
	if !cache.WaitForCacheSync(stopCh, cSynced, piSynced, cpiSynced, nSynced) {
		log.Log.Error(err, "Failed to Create kubernetes client")
		os.Exit(1)
	}

	configData := config.NewConfigData(
		kubeClient,
		ci,
		"",
		"",
		"",
		600000,
		log.Log.WithName("ConfigData"),
	)
	var cpolicies []*kyvernov1.ClusterPolicy
	var removePolicy []string
	policySelector := strings.Split(policychange,",")
	if len(policySelector) > 0 {
		for _,v := range policySelector {
			cpolicy, err := cpi.Lister().Get(v);
			if err != nil {
				if apierrors.IsNotFound(err){
					removePolicy = append(removePolicy,cpolicy.GetName())
				}
			}else{
				cpolicies = append(cpolicies, cpolicy)
			}

				for _,v := range policySelector {
					policies, err := pi.Lister().List(labels.Everything())
					if err == nil {
						for _, p := range policies {
							if v == p.GetName() {
								cp := policy.ConvertPolicyToClusterPolicy(p)
								cpolicies = append(cpolicies, cp)
							}

						}
					}
				}

		}
	}else{
		cpolicies, err = cpi.Lister().List(labels.Everything())
		if err != nil {
			os.Exit(1)
		}
		policies, err := pi.Lister().List(labels.Everything())
		if err != nil {
			os.Exit(1)
		}

		for _, p := range policies {
			cp := policy.ConvertPolicyToClusterPolicy(p)
			cpolicies = append(cpolicies, cp)
		}
	}



	// key uid
	resourceMap := map[string]unstructured.Unstructured{}
	var engineResponses []response.EngineResponse
	for _, p := range cpolicies {
		for _, rule := range p.Spec.Rules {
			for _, k := range rule.MatchResources.Kinds {
				resourceSchema, _, err := dClient.DiscoveryClient.FindResource("", k)
				if err != nil {
					log.Log.Error(err, "failed to find resource", "kind", k)
					continue
				}

				if !resourceSchema.Namespaced && scope == Cluster {
					rMap := policy.GetResourcesPerNamespace(k, dClient, "", rule, configData, log.Log)
					policy.MergeResources(resourceMap, rMap)
				} else if resourceSchema.Namespaced {
					namespaces := policy.GetNamespacesForRule(&rule, np.Lister(), log.Log)
					for _, ns := range namespaces {
						if ns == n {
							rMap := policy.GetResourcesPerNamespace(k, dClient, ns, rule, configData, log.Log)
							for _, r := range rMap {
								labels := r.GetLabels()
								_, okChart := labels["app"]
								_, okRelease := labels["release"]
								if okChart && okRelease && scope == Helm {
									policy.MergeResources(resourceMap, rMap)
								} else if scope == Namespace && r.GetNamespace() != "" {
									policy.MergeResources(resourceMap, rMap)
								}
							}
						}

					}
				}
			}
		}

		if p.HasAutoGenAnnotation() {
			resourceMap = policy.ExcludePod(resourceMap, log.Log)
		}
		results := make(map[string][]policyreportv1alpha1.PolicyReportResult)
		for _, resource := range resourceMap {
			policyContext := engine.PolicyContext{
				NewResource:      resource,
				Context:          context.NewContext(),
				Policy:           *p,
				ExcludeGroupRole: configData.GetExcludeGroupRole(),
			}

			engineResponse := engine.Validate(policyContext)

			if len(engineResponse.PolicyResponse.Rules) > 0 {
				engineResponses = append(engineResponses, engineResponse)
			}

			engineResponse = engine.Mutate(policyContext)
			if len(engineResponse.PolicyResponse.Rules) > 0 {
				engineResponses = append(engineResponses, engineResponse)
			}

			pv := policyreport.GeneratePRsFromEngineResponse(engineResponses, log.Log)

			for _, v := range pv {
				var appname string
				switch scope {
				case Helm:
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
				case Namespace:
					appname = fmt.Sprintf("kyverno-policyreport-%s", policyContext.NewResource.GetNamespace())
					break
				case Cluster:
					appname = fmt.Sprintf("kyverno-clusterpolicyreport")
					break
				}
				builder := policyreport.NewPrBuilder()
				pv := builder.Generate(v)

				for _, e := range pv.Spec.ViolatedRules {
					result := &policyreportv1alpha1.PolicyReportResult{
						Policy:  pv.Spec.Policy,
						Rule:    e.Name,
						Message: e.Message,
						Status:  policyreportv1alpha1.PolicyStatus(e.Check),
						Resource: &corev1.ObjectReference{
							Kind:       pv.Spec.Kind,
							Namespace:  pv.Spec.Namespace,
							APIVersion: pv.Spec.APIVersion,
							Name:       pv.Spec.Name,
						},
					}
					results[appname] = append(results[appname], *result)
				}
			}
		}

		for k, _ := range results {
			if k == "" {
				continue
			}
			if scope == Helm || scope == Namespace {
				availablepr, err := kclient.PolicyV1alpha1().PolicyReports(n).Get(k, metav1.GetOptions{})

				if err != nil {
					if apierrors.IsNotFound(err) {
						availablepr = initPolicyReport(scope, n, k)
					}
				}
				availablepr, action := mergeReport(availablepr, results[k], removePolicy)
				if action == "Create" {
					_, err := kclient.PolicyV1alpha1().PolicyReports(n).Create(availablepr)
					if err != nil {
						log.Log.Error(err, "Error in Create polciy report", "appreport", k)
					}
				} else {
					_, err := kclient.PolicyV1alpha1().PolicyReports(n).Update(availablepr)
					if err != nil {
						log.Log.Error(err, "Error in update polciy report", "appreport", k)
					}
				}
			} else {
				availablepr, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get(k, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						availablepr = initClusterPolicyReport(scope, k)
					}
				}
				availablepr, action := mergeClusterReport(availablepr, results[k])
				if action == "Create" {
					_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Create(availablepr)
					if err != nil {
						log.Log.Error(err, "Error in Create polciy report", "appreport", k)
					}
				} else {
					_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Update(availablepr)
					if err != nil {
						log.Log.Error(err, "Error in update polciy report", "appreport", k)
					}
				}
			}

		}
	}

	// Create Policy Report
}

func configmapScan(n, scope string, wg *sync.WaitGroup, restConfig *rest.Config) {
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

	configmap, err := dClient.GetResource("", "ConfigMap", config.KubePolicyNamespace, "kyverno-event")
	if err != nil {

		os.Exit(1)
	}
	var job *v1.ConfigMap
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(configmap.UnstructuredContent(), &job); err != nil {
		os.Exit(1)
	}
	var response map[string][]policyreport.Info
	if scope == Cluster {
		if err := json.Unmarshal([]byte(job.Data["Namespace"]), &response); err != nil {
			log.Log.Error(err, "")
		}
	} else if scope == Helm {
		if err := json.Unmarshal([]byte(job.Data["Helm"]), &response); err != nil {
			log.Log.Error(err, "")
		}
	} else {
		if err := json.Unmarshal([]byte(job.Data["Namespace"]), &response); err != nil {
			log.Log.Error(err, "")
		}
	}
	var results = make(map[string][]policyreportv1alpha1.PolicyReportResult)
	var ns []string
	for k := range response {
		for _, v := range response[k] {
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
				var appname string
				// Increase Count
				if scope == Cluster {
					appname = fmt.Sprintf("kyverno-clusterpolicyreport")
				} else if scope == Helm {
					resource, err := dClient.GetResource(v.Resource.GetAPIVersion(), v.Resource.GetKind(), v.Resource.GetNamespace(), v.Resource.GetName())
					if err != nil {
						log.Log.Error(err, "failed to get resource")
						continue
					}
					labels := resource.GetLabels()
					_, okChart := labels["app"]
					_, okRelease := labels["release"]
					if okChart && okRelease {
						appname = fmt.Sprintf("kyverno-policyreport-%s-%s", labels["app"], v.Resource.GetNamespace())

					}
				} else {
					appname = fmt.Sprintf("kyverno-policyreport-%s", v.Resource.GetNamespace())
				}
				results[appname] = append(results[appname], *result)
			}

		}
	}

	for k := range results {
		if scope == Helm || scope == Namespace {
			availablepr, err := kclient.PolicyV1alpha1().PolicyReports(n).Get(k, metav1.GetOptions{})
			str := strings.Split(k, "-")
			var namespace string
			if len(str) == 2 {
				namespace = str[1]
			} else if len(str) == 3 {
				namespace = str[2]
			}
			if err != nil {
				if apierrors.IsNotFound(err) {
					availablepr = initPolicyReport(scope, namespace, k)
				}
			}

			availablepr, action := mergeReport(availablepr, results[k],[]string{})
			if action == "Create" {
				availablepr.SetLabels(map[string]string{
					"policy-state": "state",
				})
				_, err := kclient.PolicyV1alpha1().PolicyReports(availablepr.GetNamespace()).Create(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in Create polciy report", "appreport", k)
				}
			} else {
				_, err := kclient.PolicyV1alpha1().PolicyReports(availablepr.GetNamespace()).Update(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in update polciy report", "appreport", k)
				}
			}
		} else {
			availablepr, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get(k, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					availablepr = initClusterPolicyReport(scope, k)
				}
			}
			availablepr, action := mergeClusterReport(availablepr, results[k])
			if action == "Create" {
				_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Create(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in Create polciy report", "appreport", action)
				}
			} else {
				_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Update(availablepr)
				if err != nil {
					log.Log.Error(err, "Error in update polciy report", "appreport", action)
				}
			}
		}

	}
}

func mergeReport(pr *policyreportv1alpha1.PolicyReport, results []policyreportv1alpha1.PolicyReportResult,removePolicy []string) (*policyreportv1alpha1.PolicyReport, string) {
	labels := pr.GetLabels()
	var action string
	if labels["policy-state"] == "init" {
		action = "Create"
		pr.SetLabels(map[string]string{
			"policy-state": "Process",
		})
	} else {
		action = "Update"
	}
	var uniqueResponse []*policyreportv1alpha1.PolicyReportResult
	for _, r := range results {
		var isExist = false
		for _, v := range uniqueResponse {
			if r.Policy == v.Policy && r.Rule == v.Rule && r.Resource.APIVersion == v.Resource.APIVersion && r.Resource.Kind == v.Resource.Kind && r.Resource.Namespace == v.Resource.Namespace && r.Resource.Name == v.Resource.Name {
				v = &r
				isExist = true
				break
			}
		}
		if !isExist {
			uniqueResponse = append(uniqueResponse, &r)
		}
	}

	for _, r := range uniqueResponse {
		var isExist = false
		for _, v := range pr.Results {
			if r.Policy == v.Policy && r.Rule == v.Rule && r.Resource.APIVersion == v.Resource.APIVersion && r.Resource.Kind == v.Resource.Kind && r.Resource.Namespace == v.Resource.Namespace && r.Resource.Name == v.Resource.Name {
				v = r
				isExist = true
				if string(r.Status) != string(v.Status) {
					pr = changeCount(string(r.Status), string(v.Status), pr)
				}
				break
			}
		}
		if !isExist {
			pr = changeCount(string(r.Status), string(""), pr)
			pr.Results = append(pr.Results, r)
		}
	}

	if len(removePolicy) > 0 {
		for _, v := range removePolicy {
			for i, r := range pr.Results {
				if r.Policy == v {
					pr.Results = append(pr.Results[:i], pr.Results[i+1:]...)
				}
			}
		}
	}
	return pr, action
}

func mergeClusterReport(pr *policyreportv1alpha1.ClusterPolicyReport, results []policyreportv1alpha1.PolicyReportResult) (*policyreportv1alpha1.ClusterPolicyReport, string) {
	labels := pr.GetLabels()
	var action string
	if labels["policy-state"] == "init" {
		action = "Create"
		pr.SetLabels(map[string]string{
			"policy-state": "Process",
		})
	} else {
		action = "Update"
	}

	var uniqueResponse []*policyreportv1alpha1.PolicyReportResult
	for _, r := range results {
		var isExist = false
		for _, v := range uniqueResponse {
			if r.Policy == v.Policy && r.Rule == v.Rule && r.Resource.APIVersion == v.Resource.APIVersion && r.Resource.Kind == v.Resource.Kind && r.Resource.Namespace == v.Resource.Namespace && r.Resource.Name == v.Resource.Name {
				v = &r
				isExist = true
				break
			}
		}
		if !isExist {
			uniqueResponse = append(uniqueResponse, &r)
		}
	}

	if len(pr.Results) == 0 {
		pr.Results = append(pr.Results, uniqueResponse...)
		return pr, action
	}

	for _, r := range uniqueResponse {
		var isExist = false
		for _, v := range pr.Results {
			if r.Policy == v.Policy && r.Rule == v.Rule && r.Resource.APIVersion == v.Resource.APIVersion && r.Resource.Kind == v.Resource.Kind && r.Resource.Namespace == v.Resource.Namespace && r.Resource.Name == v.Resource.Name {
				v = r
				isExist = true
				if string(r.Status) != string(v.Status) {
					pr = changeClusterReportCount(string(r.Status), string(v.Status), pr)
				}
				break
			}
		}
		if !isExist {
			pr = changeClusterReportCount(string(r.Status), string(""), pr)
			pr.Results = append(pr.Results, r)
		}
	}

	return pr, action
}

func changeCount(status, oldStatus string, report *policyreportv1alpha1.PolicyReport) *policyreportv1alpha1.PolicyReport {
	switch oldStatus {
	case "Pass":
		if report.Summary.Pass--; report.Summary.Pass < 0 {
			report.Summary.Pass = 0
		}
		break
	case "Fail":
		if report.Summary.Fail--; report.Summary.Fail < 0 {
			report.Summary.Fail = 0
		}
		break
	default:
		break
	}
	switch status {
	case "Pass":
		report.Summary.Pass++
		break
	case "Fail":
		report.Summary.Fail++
		break
	default:
		break
	}
	return report
}

func changeClusterReportCount(status, oldStatus string, report *policyreportv1alpha1.ClusterPolicyReport) *policyreportv1alpha1.ClusterPolicyReport {
	switch oldStatus {
	case "Pass":
		if report.Summary.Pass--; report.Summary.Pass < 0 {
			report.Summary.Pass = 0
		}
		break
	case "Fail":
		if report.Summary.Fail--; report.Summary.Fail < 0 {
			report.Summary.Fail = 0
		}
		break
	default:
		break
	}
	switch status {
	case "Pass":
		report.Summary.Pass++
		break
	case "Fail":
		report.Summary.Fail++
		break
	default:
		break
	}
	return report
}

func initPolicyReport(scope, namespace, name string) *policyreportv1alpha1.PolicyReport {
	availablepr := &policyreportv1alpha1.PolicyReport{
		Scope: &corev1.ObjectReference{
			Kind:      scope,
			Namespace: namespace,
		},
		Summary: policyreportv1alpha1.PolicyReportSummary{},
		Results: []*policyreportv1alpha1.PolicyReportResult{},
	}
	labelMap := map[string]string{
		"policy-scope": scope,
		"policy-state": "init",
	}
	availablepr.SetName(name)
	availablepr.SetNamespace(namespace)
	availablepr.SetLabels(labelMap)
	availablepr.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "PolicyReport",
		Version: "v1alpha1",
		Group:   "policy.kubernetes.io",
	})
	return availablepr
}

func initClusterPolicyReport(scope, name string) *policyreportv1alpha1.ClusterPolicyReport {
	availablepr := &policyreportv1alpha1.ClusterPolicyReport{
		Scope: &corev1.ObjectReference{
			Kind: scope,
		},
		Summary: policyreportv1alpha1.PolicyReportSummary{},
		Results: []*policyreportv1alpha1.PolicyReportResult{},
	}
	labelMap := map[string]string{
		"policy-scope": scope,
		"policy-state": "init",
	}
	availablepr.SetName(name)
	availablepr.SetLabels(labelMap)
	availablepr.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "PolicyReport",
		Version: "v1alpha1",
		Group:   "policy.kubernetes.io",
	})
	return availablepr
}
