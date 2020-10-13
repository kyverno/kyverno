package report

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/constant"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

func backgroundScan(n, scope, policychange string, wg *sync.WaitGroup, restConfig *rest.Config, logger logr.Logger) {
	lgr := logger.WithValues("namespace", n, "scope", scope, "policychange", policychange)
	defer func() {
		wg.Done()
	}()
	dClient, err := client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), lgr)
	if err != nil {
		lgr.Error(err, "Error in creating dcclient with provided rest config")
		os.Exit(1)
	}

	kclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		lgr.Error(err, "Error in creating kyverno client with provided rest config")
		os.Exit(1)
	}
	kubeClient, err := utils.NewKubeClient(restConfig)
	if err != nil {
		lgr.Error(err, "Error in creating kube client with provided rest config")
		os.Exit(1)
	}
	pclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		lgr.Error(err, "Error in creating kyverno client for policy with provided rest config")
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
		lgr.Error(err, "Failed to Create kubernetes client")
		os.Exit(1)
	}

	configData := config.NewConfigData(
		kubeClient,
		ci,
		"",
		"",
		"",
		lgr.WithName("ConfigData"),
	)
	var cpolicies []*kyvernov1.ClusterPolicy
	removePolicy := []string{}
	policySelector := strings.Split(policychange, ",")
	if len(policySelector) > 0 && policychange != "" {
		for _, v := range policySelector {
			cpolicy, err := cpi.Lister().Get(v)
			if err != nil {
				if apierrors.IsNotFound(err) {
					removePolicy = append(removePolicy, v)
				}
			} else {
				cpolicies = append(cpolicies, cpolicy)
			}
			for _, v := range policySelector {
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
	} else {
		cpolicies, err = cpi.Lister().List(labels.Everything())
		if err != nil {
			lgr.Error(err, "Error in geting cluster policy list")
			os.Exit(1)
		}
		policies, err := pi.Lister().List(labels.Everything())
		if err != nil {
			lgr.Error(err, "Error in geting policy list")
			os.Exit(1)
		}

		for _, p := range policies {
			cp := policy.ConvertPolicyToClusterPolicy(p)
			cpolicies = append(cpolicies, cp)
		}
	}

	// key uid
	resourceMap := map[string]map[string]unstructured.Unstructured{}
	for _, p := range cpolicies {
		for _, rule := range p.Spec.Rules {
			for _, k := range rule.MatchResources.Kinds {
				resourceSchema, _, err := dClient.DiscoveryClient.FindResource("", k)
				if err != nil {
					lgr.Error(err, "failed to find resource", "kind", k)
					continue
				}
				if !resourceSchema.Namespaced {
					rMap := policy.GetResourcesPerNamespace(k, dClient, "", rule, configData, log.Log)
					if len(resourceMap[constant.Cluster]) == 0 {
						resourceMap[constant.Cluster] = make(map[string]unstructured.Unstructured)
					}
					policy.MergeResources(resourceMap[constant.Cluster], rMap)
				} else {
					namespaces := policy.GetNamespacesForRule(&rule, np.Lister(), log.Log)
					for _, ns := range namespaces {
						if ns == n {
							rMap := policy.GetResourcesPerNamespace(k, dClient, ns, rule, configData, log.Log)
							for _, r := range rMap {
								labels := r.GetLabels()
								_, okChart := labels["app"]

								if okChart {
									if len(resourceMap[constant.App]) == 0 {
										resourceMap[constant.App] = make(map[string]unstructured.Unstructured)
									}
									policy.MergeResources(resourceMap[constant.App], rMap)
								} else {
									fmt.Println(r.GetName())
									fmt.Println(labels["app"])
									fmt.Println("========")
									if len(resourceMap[constant.Namespace]) == 0 {
										resourceMap[constant.Namespace] = make(map[string]unstructured.Unstructured)
									}
									policy.MergeResources(resourceMap[constant.Namespace], rMap)
								}
							}
						}

					}
				}
			}
		}
		if p.HasAutoGenAnnotation() {
			switch scope {
			case constant.Cluster:
				resourceMap[constant.Cluster] = policy.ExcludePod(resourceMap[constant.Cluster], log.Log)
				delete(resourceMap, constant.Namespace)
				delete(resourceMap, constant.App)
				break
			case constant.Namespace:
				resourceMap[constant.Namespace] = policy.ExcludePod(resourceMap[constant.Namespace], log.Log)
				delete(resourceMap, constant.Cluster)
				delete(resourceMap, constant.App)
				break
			case constant.App:
				resourceMap[constant.App] = policy.ExcludePod(resourceMap[constant.App], log.Log)
				delete(resourceMap, constant.Namespace)
				delete(resourceMap, constant.Cluster)
				break
			case constant.All:
				resourceMap[constant.Cluster] = policy.ExcludePod(resourceMap[constant.Cluster], log.Log)
				resourceMap[constant.Namespace] = policy.ExcludePod(resourceMap[constant.Namespace], log.Log)
				resourceMap[constant.App] = policy.ExcludePod(resourceMap[constant.App], log.Log)
			}
		}

		results := make(map[string][]policyreportv1alpha1.PolicyReportResult)
		for key, _ := range resourceMap {
			for _, resource := range resourceMap[key] {
				policyContext := engine.PolicyContext{
					NewResource:      resource,
					Context:          context.NewContext(),
					Policy:           *p,
					ExcludeGroupRole: configData.GetExcludeGroupRole(),
				}

				results = createResults(policyContext, key, results)
			}
		}

		for k, _ := range results {
			if k == "" {
				continue
			}

			err := createReport(kclient, k, results[k], removePolicy, lgr)
			if err != nil {
				continue
			}
		}
	}
}

func createReport(kclient *kyvernoclient.Clientset, name string, results []policyreportv1alpha1.PolicyReportResult, removePolicy []string, lgr logr.Logger) error {

	var scope, ns string
	if strings.Contains(name, "clusterpolicyreport") {
		scope = constant.Cluster
	} else if strings.Contains(name, "policyreport-app-") {
		scope = constant.App
		ns = strings.ReplaceAll(name, "policyreport-app-", "")
		str := strings.Split(ns, "--")
		ns = str[1]
	} else if strings.Contains(name, "policyreport-ns-") {
		scope = constant.Namespace
		ns = strings.ReplaceAll(name, "policyreport-ns-", "")
	}

	if scope == constant.App || scope == constant.Namespace {
		availablepr, err := kclient.PolicyV1alpha1().PolicyReports(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				availablepr = initPolicyReport(scope, ns, name)
			} else {
				return err
			}
		}
		availablepr, action := mergeReport(availablepr, results, removePolicy)

		if action == "Create" {
			availablepr.SetLabels(map[string]string{
				"policy-state": "state",
			})
			_, err := kclient.PolicyV1alpha1().PolicyReports(availablepr.GetNamespace()).Create(availablepr)
			if err != nil {
				lgr.Error(err, "Error in Create policy report", "appreport", name)
				return err
			}
		} else {
			_, err := kclient.PolicyV1alpha1().PolicyReports(availablepr.GetNamespace()).Update(availablepr)
			if err != nil {
				lgr.Error(err, "Error in update policy report", "appreport", name)
				return err
			}
		}
	} else {
		availablepr, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				availablepr = initClusterPolicyReport(scope, name)
			} else {
				return err
			}
		}
		availablepr, action := mergeClusterReport(availablepr, results, removePolicy)

		if action == "Create" {
			_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Create(availablepr)
			if err != nil {
				lgr.Error(err, "Error in Create policy report", "appreport", availablepr)
				return err
			}
		} else {
			_, err := kclient.PolicyV1alpha1().ClusterPolicyReports().Update(availablepr)
			if err != nil {
				lgr.Error(err, "Error in update policy report", "appreport", name)
				return err
			}
		}
	}
	return nil
}

func createResults(policyContext engine.PolicyContext, key string, results map[string][]policyreportv1alpha1.PolicyReportResult) map[string][]policyreportv1alpha1.PolicyReportResult {

	var engineResponses []response.EngineResponse
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
		labels := policyContext.NewResource.GetLabels()
		_, okChart := labels["app"]
		if key == constant.App {
			if okChart {
				appname = fmt.Sprintf("policyreport-app-%s--%s", labels["app"], policyContext.NewResource.GetNamespace())
			}
		} else if key == constant.Namespace {
			appname = fmt.Sprintf("policyreport-ns-%s", policyContext.NewResource.GetNamespace())
		} else {
			appname = fmt.Sprintf("clusterpolicyreport")
		}
		if appname != "" {
			builder := policyreport.NewPrBuilder()
			pv := builder.Generate(v)

			for _, e := range pv.Spec.ViolatedRules {
				result := &policyreportv1alpha1.PolicyReportResult{
					Policy:  pv.Spec.Policy,
					Rule:    e.Name,
					Message: e.Message,
				}
				rd := &policyreportv1alpha1.ResourceStatus{
					Resource: &corev1.ObjectReference{
						Kind:       pv.Spec.Kind,
						Namespace:  pv.Spec.Namespace,
						APIVersion: pv.Spec.APIVersion,
						Name:       pv.Spec.Name,
					},
					Status: policyreportv1alpha1.PolicyStatus(e.Check),
				}
				result.Resources = append(result.Resources, rd)
				results[appname] = append(results[appname], *result)
			}
		}
	}
	return results
}

func configmapScan(scope string, wg *sync.WaitGroup, restConfig *rest.Config, logger logr.Logger) {
	defer func() {
		wg.Done()
	}()
	lgr := logger.WithValues("scope", scope)
	dClient, err := client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), lgr)
	if err != nil {
		lgr.Error(err, "Error in creating dcclient with provided rest config")
		os.Exit(1)
	}

	kclient, err := kyvernoclient.NewForConfig(restConfig)
	if err != nil {
		lgr.Error(err, "Error in creating kyverno client with provided rest config")
		os.Exit(1)
	}

	configmap, err := dClient.GetResource("", "ConfigMap", config.KubePolicyNamespace, config.ConfimapNameForPolicyReport)
	if err != nil {
		lgr.Error(err, "Error in getting configmap")
		os.Exit(1)
	}
	var job *v1.ConfigMap
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(configmap.UnstructuredContent(), &job); err != nil {
		lgr.Error(err, "Error in converting resource to Default Unstructured Converter")
		os.Exit(1)
	}
	response := make(map[string]map[string][]policyreport.Info)
	var temp = map[string][]policyreport.Info{}
	if scope == constant.Cluster {
		if err := json.Unmarshal([]byte(job.Data[constant.Cluster]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.Cluster] = temp
		delete(job.Data, constant.Namespace)
		delete(job.Data, constant.App)
	} else if scope == constant.App {
		if err := json.Unmarshal([]byte(job.Data[constant.App]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.App] = temp
		delete(job.Data, constant.Cluster)
		delete(job.Data, constant.Namespace)
	} else if scope == constant.Namespace {
		if err := json.Unmarshal([]byte(job.Data[constant.Namespace]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.Namespace] = temp
		delete(job.Data, constant.Cluster)
		delete(job.Data, constant.App)
	} else {
		if err := json.Unmarshal([]byte(job.Data[constant.Cluster]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.Cluster] = temp
		temp = make(map[string][]policyreport.Info)
		if err := json.Unmarshal([]byte(job.Data[constant.App]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.App] = temp
		temp = make(map[string][]policyreport.Info)
		if err := json.Unmarshal([]byte(job.Data[constant.Namespace]), &temp); err != nil {
			lgr.Error(err, "Error in json marshal of namespace data")
		}
		response[constant.Namespace] = temp
		temp = make(map[string][]policyreport.Info)
	}
	var results = make(map[string][]policyreportv1alpha1.PolicyReportResult)
	var ns []string
	for k := range response {
		for n, infos := range response[k] {
			for _, v := range infos {
				for _, r := range v.Rules {
					builder := policyreport.NewPrBuilder()
					pv := builder.Generate(v)
					result := &policyreportv1alpha1.PolicyReportResult{
						Policy:  pv.Spec.Policy,
						Rule:    r.Name,
						Message: r.Message,
					}
					rd := &policyreportv1alpha1.ResourceStatus{
						Resource: &corev1.ObjectReference{
							Kind:       pv.Spec.Kind,
							Namespace:  pv.Spec.Namespace,
							APIVersion: pv.Spec.APIVersion,
							Name:       pv.Spec.Name,
						},
						Status: policyreportv1alpha1.PolicyStatus(r.Check),
					}
					result.Resources = append(result.Resources, rd)

					if !strings.Contains(strings.Join(ns, ","), v.Resource.GetNamespace()) {
						ns = append(ns, n)
					}

					var appname string
					resource, err := dClient.GetResource(v.Resource.GetAPIVersion(), v.Resource.GetKind(), v.Resource.GetNamespace(), v.Resource.GetName())
					if err != nil {
						lgr.Error(err, "failed to get resource")
						continue
					}
					labels := resource.GetLabels()
					_, okChart := labels["app"]
					_, okRelease := labels["release"]

					if k == constant.Cluster {
						appname = fmt.Sprintf("clusterpolicyreport")
					}
					if k == constant.App {
						if okChart && okRelease {
							appname = fmt.Sprintf("policyreport-app-%s--%s", labels["app"], v.Resource.GetNamespace())
						}
					}
					if k == constant.Namespace {
						if !okChart && !okRelease {
							appname = fmt.Sprintf("policyreport-ns-%s", v.Resource.GetNamespace())
						}
					}
					results[appname] = append(results[appname], *result)
				}
			}
		}
	}

	for k := range results {
		if k == "" {
			continue
		}
		err := createReport(kclient, k, results[k], []string{}, lgr)
		if err != nil {
			continue
		}
	}
}

func mergeReport(pr *policyreportv1alpha1.PolicyReport, results []policyreportv1alpha1.PolicyReportResult, removePolicy []string) (*policyreportv1alpha1.PolicyReport, string) {
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
	rules := make(map[string]*policyreportv1alpha1.PolicyReportResult, 0)

	for _, v := range pr.Results {
		for _, r := range v.Resources {
			key := fmt.Sprintf("%s-%s-%s", v.Policy, v.Rule, pr.GetName())
			if _, ok := rules[key]; ok {
				isExist := false
				for _, resourceStatus := range rules[key].Resources {
					if resourceStatus.Resource.APIVersion == r.Resource.APIVersion && r.Resource.Kind == resourceStatus.Resource.Kind && r.Resource.Namespace == resourceStatus.Resource.Namespace && r.Resource.Name == resourceStatus.Resource.Name {
						isExist = true
						resourceStatus = r
					}
				}
				if !isExist {
					rules[key].Resources = append(rules[key].Resources, r)
				}
			} else {
				rules[key] = &policyreportv1alpha1.PolicyReportResult{
					Policy:    v.Policy,
					Rule:      v.Rule,
					Message:   v.Message,
					Resources: make([]*policyreportv1alpha1.ResourceStatus, 0),
				}

				rules[key].Resources = append(rules[key].Resources, r)
			}
		}
	}
	for _, v := range results {
		for _, r := range v.Resources {
			key := fmt.Sprintf("%s-%s-%s", v.Policy, v.Rule, pr.GetName())
			if _, ok := rules[key]; ok {
				isExist := false
				for _, resourceStatus := range rules[key].Resources {
					if resourceStatus.Resource.APIVersion == r.Resource.APIVersion && r.Resource.Kind == resourceStatus.Resource.Kind && r.Resource.Namespace == resourceStatus.Resource.Namespace && r.Resource.Name == resourceStatus.Resource.Name {
						isExist = true
						resourceStatus = r
					}
				}
				if !isExist {
					rules[key].Resources = append(rules[key].Resources, r)
				}
			} else {
				rules[key] = &policyreportv1alpha1.PolicyReportResult{
					Policy:    v.Policy,
					Rule:      v.Rule,
					Message:   v.Message,
					Resources: make([]*policyreportv1alpha1.ResourceStatus, 0),
				}
				rules[key].Resources = append(rules[key].Resources, r)
			}
		}
	}

	if len(removePolicy) > 0 {
		for _, v := range removePolicy {
			for k, r := range rules {
				if r.Policy == v {
					delete(rules, k)
				}
			}
		}
	}
	pr.Summary.Pass = 0
	pr.Summary.Fail = 0
	pr.Results = make([]*policyreportv1alpha1.PolicyReportResult, 0)
	for k, _ := range rules {
		pr.Results = append(pr.Results, rules[k])
		for _, r := range rules[k].Resources {
			if string(r.Status) == "Pass" {
				pr.Summary.Pass++
			} else {
				pr.Summary.Fail++
			}

		}
	}
	return pr, action
}

func mergeClusterReport(pr *policyreportv1alpha1.ClusterPolicyReport, results []policyreportv1alpha1.PolicyReportResult, removePolicy []string) (*policyreportv1alpha1.ClusterPolicyReport, string) {
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

	for _, r := range pr.Results {
		for _, v := range results {
			if r.Policy == v.Policy && r.Rule == v.Rule {
				for i, result := range r.Resources {
					for k, event := range v.Resources {
						if event.Resource.APIVersion == result.Resource.APIVersion && result.Resource.Kind == event.Resource.Kind && result.Resource.Namespace == event.Resource.Namespace && result.Resource.Name == event.Resource.Name {
							r.Resources[i] = v.Resources[k]
							if string(event.Status) != string(result.Status) {
								pr = changeClusterReportCount(string(event.Status), string(result.Status), pr)
							}
							v.Resources = append(v.Resources[:k], v.Resources[k+1:]...)
							break
						}
					}
					for _, resource := range v.Resources {
						pr = changeClusterReportCount(string(resource.Status), string(""), pr)
						r.Resources = append(r.Resources, resource)
					}
				}
			}
		}
	}

	if len(removePolicy) > 0 {
		for _, v := range removePolicy {
			for i, r := range pr.Results {
				if r.Policy == v {
					for _, v := range r.Resources {
						pr = changeClusterReportCount("", string(v.Status), pr)
					}
					pr.Results = append(pr.Results[:i], pr.Results[i+1:]...)
				}
			}
		}
	}
	return pr, action
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
	return availablepr
}
