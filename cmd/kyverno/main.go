package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/breaker"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/admissionpolicygenerator"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	genericloggingcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/logging"
	genericwebhookcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/webhook"
	globalcontextcontroller "github.com/kyverno/kyverno/pkg/controllers/globalcontext"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	updaterequestmetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/updaterequest"
	policycachecontroller "github.com/kyverno/kyverno/pkg/controllers/policycache"
	policystatuscontroller "github.com/kyverno/kyverno/pkg/controllers/policystatus"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils/generator"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookscelexception "github.com/kyverno/kyverno/pkg/webhooks/celexception"
	webhooksexception "github.com/kyverno/kyverno/pkg/webhooks/exception"
	webhooksglobalcontext "github.com/kyverno/kyverno/pkg/webhooks/globalcontext"
	webhookspolicy "github.com/kyverno/kyverno/pkg/webhooks/policy"
	webhooksresource "github.com/kyverno/kyverno/pkg/webhooks/resource"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/ivpol"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/mpol"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	"github.com/prometheus/client_golang/prometheus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery/cached/memory"
	kubeinformers "k8s.io/client-go/informers"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1informers "k8s.io/client-go/informers/admissionregistration/v1beta1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	exceptionWebhookControllerName    = "exception-webhook-controller"
	celExceptionWebhookControllerName = "celexception-webhook-controller"
	gctxWebhookControllerName         = "global-context-webhook-controller"
)

var (
	caSecretName                 string
	tlsSecretName                string
	disableCertManagerController bool
)

type workqueueMetricsProvider struct{}

func (p *workqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "workqueue_depth",
		Help:        "Current depth of workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Gauge)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	m := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "workqueue_adds_total",
		Help:        "Total number of adds handled by workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Counter)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	m := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "workqueue_queue_duration_seconds",
		Help:        "How long in seconds an item stays in workqueue before being requested",
		ConstLabels: prometheus.Labels{"name": name},
		Buckets:     prometheus.ExponentialBuckets(10e-9, 10, 10),
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Histogram)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	m := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "workqueue_work_duration_seconds",
		Help:        "How long in seconds processing an item from workqueue takes.",
		ConstLabels: prometheus.Labels{"name": name},
		Buckets:     prometheus.ExponentialBuckets(10e-9, 10, 10),
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Histogram)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "workqueue_unfinished_work_seconds",
		Help:        "How many seconds of work has been done that is in progress and hasn't been observed by work_duration. Large values indicate stuck threads.",
		ConstLabels: prometheus.Labels{"name": name},
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Gauge)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "workqueue_longest_running_processor_seconds",
		Help:        "How many seconds has the longest running processor for workqueue been running.",
		ConstLabels: prometheus.Labels{"name": name},
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Gauge)
		}
	}
	return m
}

func (p *workqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	m := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "workqueue_retries_total",
		Help:        "Total number of retries handled by workqueue",
		ConstLabels: prometheus.Labels{"name": name},
	})
	if err := crmetrics.Registry.Register(m); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Counter)
		}
	}
	return m
}

func init() {
	workqueue.SetProvider(&workqueueMetricsProvider{})
}

func showWarnings(ctx context.Context, logger logr.Logger) {
	logger = logger.WithName("warnings")
	if toggle.FromContext(ctx).ForceFailurePolicyIgnore() {
		logger.V(2).Info("'ForceFailurePolicyIgnore' is enabled, all policies with policy failures will be set to Ignore")
	}
}

func sanityChecks(apiserverClient apiserver.Interface) error {
	return kubeutils.CRDsInstalled(apiserverClient, "clusterpolicies.kyverno.io", "policies.kyverno.io")
}

func createNonLeaderControllers(
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	dynamicClient dclient.Interface,
	policyCache policycache.Cache,
) ([]internal.Controller, func(context.Context) error) {
	policyCacheController := policycachecontroller.NewController(
		dynamicClient,
		policyCache,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
	)
	return []internal.Controller{
			internal.NewController(policycachecontroller.ControllerName, policyCacheController, policycachecontroller.Workers),
		},
		func(ctx context.Context) error {
			if err := policyCacheController.WarmUp(); err != nil {
				return err
			}
			return nil
		}
}

func createrLeaderControllers(
	admissionReports bool,
	serverIP string,
	reportsServiceAccountName string,
	webhookTimeout int,
	autoUpdateWebhooks bool,
	excludeBootstrapResources bool,
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	caInformer corev1informers.SecretInformer,
	tlsInformer corev1informers.SecretInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	certRenewer tls.CertRenewer,
	runtime runtimeutils.Runtime,
	servicePort int32,
	configuration config.Configuration,
	eventGenerator event.Interface,
	stateRecorder webhookcontroller.StateRecorder,
) ([]internal.Controller, func(context.Context) error, error) {
	var leaderControllers []internal.Controller
	if !disableCertManagerController {
		certManager := certmanager.NewController(
			caInformer,
			tlsInformer,
			certRenewer,
			caSecretName,
			tlsSecretName,
			config.KyvernoNamespace(),
		)
		leaderControllers = append(leaderControllers, internal.NewController(certmanager.ControllerName, certManager, certmanager.Workers))
	}
	webhookController := webhookcontroller.NewController(
		dynamicClient.Discovery(),
		kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
		kyvernoClient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Policies().V1beta1().ValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().GeneratingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedGeneratingPolicies(),
		kyvernoInformer.Policies().V1beta1().ImageValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedImageValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().MutatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedMutatingPolicies(),
		deploymentInformer,
		caInformer,
		kubeKyvernoInformer.Coordination().V1().Leases(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		serverIP,
		int32(webhookTimeout), //nolint:gosec
		servicePort,
		autoUpdateWebhooks,
		excludeBootstrapResources,
		admissionReports,
		runtime,
		configuration,
		caSecretName,
		stateRecorder,
	)
	exceptionWebhookController := genericwebhookcontroller.NewController(
		exceptionWebhookControllerName,
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		caInformer,
		config.ExceptionValidatingWebhookConfigurationName,
		config.ExceptionValidatingWebhookServicePath,
		serverIP,
		servicePort,
		nil,
		[]admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"kyverno.io"},
				APIVersions: []string{"v2alpha1", "v2beta1"},
				Resources:   []string{"policyexceptions"},
			},
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
		}},
		genericwebhookcontroller.Fail,
		genericwebhookcontroller.None,
		configuration,
		caSecretName,
	)
	celExceptionWebhookController := genericwebhookcontroller.NewController(
		celExceptionWebhookControllerName,
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		caInformer,
		config.CELExceptionValidatingWebhookConfigurationName,
		config.CELExceptionValidatingWebhookServicePath,
		serverIP,
		servicePort,
		nil,
		[]admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"policies.kyverno.io"},
				APIVersions: []string{"v1alpha1"},
				Resources:   []string{"policyexceptions"},
			},
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
		}},
		genericwebhookcontroller.Fail,
		genericwebhookcontroller.None,
		configuration,
		caSecretName,
	)
	gctxWebhookController := genericwebhookcontroller.NewController(
		gctxWebhookControllerName,
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		caInformer,
		config.GlobalContextValidatingWebhookConfigurationName,
		config.GlobalContextValidatingWebhookServicePath,
		serverIP,
		servicePort,
		nil,
		[]admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"kyverno.io"},
				APIVersions: []string{"v2alpha1"},
				Resources:   []string{"globalcontextentries"},
			},
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
		}},
		genericwebhookcontroller.Fail,
		genericwebhookcontroller.None,
		configuration,
		caSecretName,
	)
	policyStatusController := policystatuscontroller.NewController(
		dynamicClient,
		kyvernoClient,
		kyvernoInformer.Policies().V1beta1().ValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().ImageValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedImageValidatingPolicies(),
		kyvernoInformer.Policies().V1beta1().MutatingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedMutatingPolicies(),
		kyvernoInformer.Policies().V1beta1().GeneratingPolicies(),
		kyvernoInformer.Policies().V1beta1().NamespacedGeneratingPolicies(),
		reportsServiceAccountName,
		stateRecorder,
	)
	leaderControllers = append(leaderControllers, internal.NewController(webhookcontroller.ControllerName, webhookController, webhookcontroller.Workers))
	leaderControllers = append(leaderControllers, internal.NewController(exceptionWebhookControllerName, exceptionWebhookController, 1))
	leaderControllers = append(leaderControllers, internal.NewController(celExceptionWebhookControllerName, celExceptionWebhookController, 1))
	leaderControllers = append(leaderControllers, internal.NewController(gctxWebhookControllerName, gctxWebhookController, 1))
	leaderControllers = append(leaderControllers, internal.NewController(policystatuscontroller.ControllerName, policyStatusController, policystatuscontroller.Workers))

	vapsRegistered, _ := admissionpolicy.IsValidatingAdmissionPolicyRegistered(kubeClient)
	mapVersion, mapVersionErr := admissionpolicy.PreferredMutatingAdmissionPolicyVersion(kubeClient)
	mapsRegistered := mapVersionErr == nil
	if vapsRegistered || mapsRegistered {
		checker := checker.NewSelfChecker(kubeClient.AuthorizationV1().SelfSubjectAccessReviews())

		var vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer
		var vapBindingInformer admissionregistrationv1informers.ValidatingAdmissionPolicyBindingInformer
		var mapV1Informer admissionregistrationv1informers.MutatingAdmissionPolicyInformer
		var mapBindingV1Informer admissionregistrationv1informers.MutatingAdmissionPolicyBindingInformer
		if vapsRegistered {
			vapInformer = kubeInformer.Admissionregistration().V1().ValidatingAdmissionPolicies()
			vapBindingInformer = kubeInformer.Admissionregistration().V1().ValidatingAdmissionPolicyBindings()
		}

		var mapBetaInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyInformer
		var mapBindingBetaInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyBindingInformer
		var mapAlphaInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer
		var mapBindingAlphaInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyBindingInformer
		if mapsRegistered {
			switch mapVersion {
			case admissionpolicy.MutatingAdmissionPolicyVersionV1:
				logging.GlobalLogger().Info("Initializing MutatingAdmissionPolicy informers for v1")
				mapV1Informer = kubeInformer.Admissionregistration().V1().MutatingAdmissionPolicies()
				mapBindingV1Informer = kubeInformer.Admissionregistration().V1().MutatingAdmissionPolicyBindings()
			case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
				logging.GlobalLogger().Info("Initializing MutatingAdmissionPolicy informers for v1beta1")
				mapBetaInformer = kubeInformer.Admissionregistration().V1beta1().MutatingAdmissionPolicies()
				mapBindingBetaInformer = kubeInformer.Admissionregistration().V1beta1().MutatingAdmissionPolicyBindings()
			case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
				logging.GlobalLogger().Info("Initializing MutatingAdmissionPolicy informers for v1alpha1")
				mapAlphaInformer = kubeInformer.Admissionregistration().V1alpha1().MutatingAdmissionPolicies()
				mapBindingAlphaInformer = kubeInformer.Admissionregistration().V1alpha1().MutatingAdmissionPolicyBindings()
			default:
				logging.GlobalLogger().Info("Skipping unsupported MutatingAdmissionPolicy informer version", "version", mapVersion)
			}
		} else {
			logging.GlobalLogger().V(2).Info("MutatingAdmissionPolicy API is not registered, skipping MAP informers", "error", mapVersionErr)
		}

		admissionpolicyController := admissionpolicygenerator.NewController(
			kubeClient,
			kyvernoClient,
			dynamicClient.Discovery(),
			kyvernoInformer.Kyverno().V1().ClusterPolicies(),
			kyvernoInformer.Policies().V1beta1().ValidatingPolicies(),
			kyvernoInformer.Policies().V1beta1().NamespacedValidatingPolicies(),
			kyvernoInformer.Policies().V1beta1().MutatingPolicies(),
			kyvernoInformer.Policies().V1beta1().NamespacedMutatingPolicies(),
			kyvernoInformer.Kyverno().V2().PolicyExceptions(),
			kyvernoInformer.Policies().V1beta1().PolicyExceptions(),
			vapInformer,
			vapBindingInformer,
			mapV1Informer,
			mapBindingV1Informer,
			mapBetaInformer,
			mapBindingBetaInformer,
			mapAlphaInformer,
			mapBindingAlphaInformer,
			eventGenerator,
			checker,
		)
		leaderControllers = append(leaderControllers, internal.NewController(admissionpolicygenerator.ControllerName, admissionpolicyController, admissionpolicygenerator.Workers))
	}
	return leaderControllers, nil, nil
}

func main() {
	var (
		serverIP                        string
		webhookTimeout                  int
		maxQueuedEvents                 int
		omitEvents                      string
		autoUpdateWebhooks              bool
		excludeBootstrapResources       bool
		webhookRegistrationTimeout      time.Duration
		admissionReports                bool
		dumpPayload                     bool
		servicePort                     int
		webhookServerHost               string
		webhookServerPort               int
		backgroundServiceAccountName    string
		reportsServiceAccountName       string
		maxAPICallResponseLength        int64
		apiCallTimeout                  time.Duration
		renewBefore                     time.Duration
		maxAuditWorkers                 int
		maxAuditCapacity                int
		maxAdmissionReports             int
		maxGlobalContextEntries         int
		controllerRuntimeMetricsAddress string
		tlsKeyAlgorithm                 string
	)
	flagset := flag.NewFlagSet("kyverno", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.IntVar(&webhookTimeout, "webhookTimeout", webhookcontroller.DefaultWebhookTimeout, "Timeout for webhook configurations (number of seconds, integer).")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omitEvents", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omitEvents=PolicyApplied,PolicyViolation")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flagset.BoolVar(&excludeBootstrapResources, "excludeBootstrapResources", false, "Set this flag to 'true' to exclude cluster bootstrap resources (Node, CertificateSigningRequest) from Fail resource webhooks, avoiding a webhook deadlock when the cluster restarts with no Kyverno pods running. Policies targeting these resources are not enforced while this is enabled.")
	flagset.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flagset.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flagset.Func(toggle.ForceFailurePolicyIgnoreFlagName, toggle.ForceFailurePolicyIgnoreDescription, toggle.ForceFailurePolicyIgnore.Parse)
	flagset.Func(toggle.GenerateValidatingAdmissionPolicyFlagName, toggle.GenerateValidatingAdmissionPolicyDescription, toggle.GenerateValidatingAdmissionPolicy.Parse)
	flagset.Func(toggle.GenerateMutatingAdmissionPolicyFlagName, toggle.GenerateMutatingAdmissionPolicyDescription, toggle.GenerateMutatingAdmissionPolicy.Parse)
	flagset.Func(toggle.DumpMutatePatchesFlagName, toggle.DumpMutatePatchesDescription, toggle.DumpMutatePatches.Parse)
	flagset.Func(toggle.AllowHTTPInNamespacedPoliciesFlagName, toggle.AllowHTTPInNamespacedPoliciesDescription, toggle.AllowHTTPInNamespacedPolicies.Parse)
	flagset.Func(toggle.HTTPBlocklistFlagName, toggle.HTTPBlocklistDescription, toggle.HTTPBlocklist.Parse)
	flagset.Func(toggle.HTTPAllowlistFlagName, toggle.HTTPAllowlistDescription, toggle.HTTPAllowlist.Parse)
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.IntVar(&servicePort, "servicePort", 443, "Port used by the Kyverno Service resource and for webhook configurations.")
	flagset.StringVar(&webhookServerHost, "webhookServerHost", "", "Host used by the webhook server. If not set, it will default to [::] for IPv6 or 0.0.0.0 for IPv4.")
	flagset.IntVar(&webhookServerPort, "webhookServerPort", 9443, "Port used by the webhook server.")
	flagset.StringVar(&backgroundServiceAccountName, "backgroundServiceAccountName", "", "Background controller service account name.")
	flagset.StringVar(&reportsServiceAccountName, "reportsServiceAccountName", "", "Reports controller service account name.")
	flagset.StringVar(&caSecretName, "caSecretName", "", "Name of the secret containing CA.")
	flagset.StringVar(&tlsSecretName, "tlsSecretName", "", "Name of the secret containing TLS pair.")
	flagset.BoolVar(&disableCertManagerController, "disableCertManagerController", false, "Disable the in-process certificate manager controller.")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 10*1000*1000, "Configure the value of maximum allowed GET response size from API Calls")
	flagset.DurationVar(&apiCallTimeout, "apiCallTimeout", 30*time.Second, "Timeout for HTTP API calls made by policies. A value of 0 means no timeout.")
	flagset.DurationVar(&renewBefore, "renewBefore", 15*24*time.Hour, "The certificate renewal time before expiration")
	flagset.IntVar(&maxAuditWorkers, "maxAuditWorkers", 8, "Maximum number of workers for audit policy processing")
	flagset.IntVar(&maxAuditCapacity, "maxAuditCapacity", 1000, "Maximum capacity of the audit policy task queue")
	flagset.IntVar(&maxAdmissionReports, "maxAdmissionReports", 10000, "Maximum number of admission reports before we stop creating new ones")
	flagset.IntVar(&maxGlobalContextEntries, "maxGlobalContextEntries", 0, "Maximum number of entries in the global context store. When the limit is reached, new entries are rejected and retried. A value of 0 means unbounded.")
	flagset.StringVar(&controllerRuntimeMetricsAddress, "controllerRuntimeMetricsAddress", "", `Bind address for controller-runtime metrics server. It will be defaulted to ":8080" if unspecified. Set this to "0" to disable the metrics server.`)
	flagset.StringVar(&tlsKeyAlgorithm, "tlsKeyAlgorithm", "RSA", "Key algorithm for self-signed TLS certificates (RSA, ECDSA, Ed25519)")
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithTracing(),
		internal.WithMetrics(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithCosign(),
		internal.WithRegistryClient(),
		internal.WithImageVerifyCache(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithEventsClient(),
		internal.WithApiServerClient(),
		internal.WithMetadataClient(),
		internal.WithFlagSets(flagset),
		internal.WithReporting(),
		internal.WithRestConfig(),
	)

	// parse flags
	internal.ParseFlags(appConfig)
	apicall.SetScopedTokenClientTimeout(apiCallTimeout)
	if err := celcompiler.ValidateHTTPFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid HTTP flag configuration: %v\n", err)
		os.Exit(1)
	}
	var wg wait.Group
	func() {
		signalCtx, setup, sdown := internal.Setup(appConfig, "kyverno-admission-controller", false)
		defer sdown()
		if caSecretName == "" {
			setup.Logger.Error(errors.New("exiting... caSecretName is a required flag"), "exiting... caSecretName is a required flag")
			os.Exit(1)
		}
		if tlsSecretName == "" {
			setup.Logger.Error(errors.New("exiting... tlsSecretName is a required flag"), "exiting... tlsSecretName is a required flag")
			os.Exit(1)
		}
		keyAlgorithm, ok := tls.KeyAlgorithms[strings.ToUpper(tlsKeyAlgorithm)]
		if !ok {
			setup.Logger.Error(fmt.Errorf("unsupported key algorithm: %s (supported: RSA, ECDSA, Ed25519)", tlsKeyAlgorithm), "invalid tlsKeyAlgorithm flag")
			os.Exit(1)
		}
		generateMutatingAdmissionPolicy := toggle.FromContext(context.TODO()).GenerateMutatingAdmissionPolicy()
		if generateMutatingAdmissionPolicy {
			registered, err := admissionpolicy.IsMutatingAdmissionPolicyRegistered(setup.KubeClient)
			if !registered {
				setup.Logger.Error(err, "MutatingAdmissionPolicies isn't supported in the API server")
				os.Exit(1)
			}
		}

		caSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), caSecretName, setup.ResyncPeriod)
		tlsSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), tlsSecretName, setup.ResyncPeriod)
		kyvernoDeployment := informers.NewDeploymentInformer(setup.KubeClient, config.KyvernoNamespace(), config.KyvernoDeploymentName(), setup.ResyncPeriod)
		if !informers.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, caSecret, tlsSecret, kyvernoDeployment) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		showWarnings(signalCtx, setup.Logger)
		kyamlopenapi.Schema()
		if err := sanityChecks(setup.ApiServerClient); err != nil {
			setup.Logger.Error(err, "sanity checks failed")
			os.Exit(1)
		}
		kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, setup.ResyncPeriod)
		kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, setup.ResyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
		kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
		certRenewer := tls.NewCertRenewer(
			setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
			tls.CertRenewalInterval,
			tls.CAValidityDuration,
			tls.TLSValidityDuration,
			renewBefore,
			serverIP,
			config.KyvernoServiceName(),
			config.DnsNames(config.KyvernoServiceName(), config.KyvernoNamespace()),
			config.KyvernoNamespace(),
			caSecretName,
			tlsSecretName,
			keyAlgorithm,
		)
		policyCache := policycache.NewCache()
		notifyChan := make(chan string)
		stateRecorder := webhookcontroller.NewStateRecorder(notifyChan)
		eventGenerator := event.NewEventGenerator(
			setup.EventsClient,
			logging.WithName("EventGenerator"),
			maxQueuedEvents,
			setup.Configuration,
			strings.Split(omitEvents, ",")...,
		)
		gcstore := store.New(maxGlobalContextEntries)
		restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(setup.KubeClient.Discovery()))

		gceController := internal.NewController(
			globalcontextcontroller.ControllerName,
			globalcontextcontroller.NewController(
				kyvernoInformer.Kyverno().V2beta1().GlobalContextEntries(),
				setup.KubeClient,
				setup.KyvernoDynamicClient,
				setup.KyvernoClient,
				gcstore,
				eventGenerator,
				maxAPICallResponseLength,
				apiCallTimeout,
				true,
				setup.Jp,
			),
			globalcontextcontroller.Workers,
		)
		polexCache, polexController := internal.NewExceptionSelector(setup.Logger, kyvernoInformer)
		eventController := internal.NewController(
			event.ControllerName,
			eventGenerator,
			event.Workers,
		)
		policymetricscontroller.NewController(
			kyvernoInformer.Kyverno().V1().ClusterPolicies(),
			kyvernoInformer.Kyverno().V1().Policies(),
			&wg,
		)
		updaterequestmetricscontroller.NewController(
			kyvernoInformer.Kyverno().V2().UpdateRequests(),
		)
		genericloggingcontroller.NewController(
			setup.Logger.WithName("policy"),
			"Policy",
			kyvernoInformer.Kyverno().V1().Policies(),
			genericloggingcontroller.CheckGeneration,
		)
		genericloggingcontroller.NewController(
			setup.Logger.WithName("cluster-policy"),
			"ClusterPolicy",
			kyvernoInformer.Kyverno().V1().ClusterPolicies(),
			genericloggingcontroller.CheckGeneration,
		)
		runtime := runtimeutils.NewRuntime(
			setup.Logger.WithName("runtime-checks"),
			serverIP,
			kubeKyvernoInformer.Apps().V1().Deployments(),
			certRenewer,
		)
		engine := internal.NewEngine(
			signalCtx,
			setup.Logger,
			setup.Configuration,
			setup.Jp,
			setup.KyvernoDynamicClient,
			setup.ImageVerifyCacheClient,
			setup.KubeClient,
			setup.KyvernoClient,
			setup.RegistrySecretLister,
			apicall.NewAPICallConfiguration(maxAPICallResponseLength, apiCallTimeout),
			polexCache,
			gcstore,
		)
		nonLeaderControllers, nonLeaderBootstrap := createNonLeaderControllers(
			kyvernoInformer,
			setup.KyvernoDynamicClient,
			policyCache,
		)
		if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		if nonLeaderBootstrap != nil {
			if err := nonLeaderBootstrap(signalCtx); err != nil {
				setup.Logger.Error(err, "warning: failed to bootstrap non leader controllers")
			}
		}
		le, err := leaderelection.New(
			setup.Logger.WithName("leader-election"),
			"kyverno",
			config.KyvernoNamespace(),
			setup.LeaderElectionClient,
			config.KyvernoPodName(),
			internal.LeaderElectionRetryPeriod(),
			func(ctx context.Context) {
				logger := setup.Logger.WithName("leader")
				leaderControllers, warmup, err := createrLeaderControllers(
					admissionReports,
					serverIP,
					reportsServiceAccountName,
					webhookTimeout,
					autoUpdateWebhooks,
					excludeBootstrapResources,
					kubeInformer,
					kubeKyvernoInformer,
					kyvernoInformer,
					caSecret,
					tlsSecret,
					kyvernoDeployment,
					setup.KubeClient,
					setup.KyvernoClient,
					setup.KyvernoDynamicClient,
					certRenewer,
					runtime,
					int32(servicePort), //nolint:gosec
					setup.Configuration,
					eventGenerator,
					stateRecorder,
				)
				if err != nil {
					logger.Error(err, "failed to create leader controllers")
					os.Exit(1)
				}
				if !internal.StartInformersAndWaitForCacheSync(ctx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
					logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
					os.Exit(1)
				}
				if warmup != nil {
					if err := warmup(ctx); err != nil {
						logger.Error(err, "failed to run warmup")
						os.Exit(1)
					}
				}
				var wg wait.Group
				for _, controller := range leaderControllers {
					controller.Run(ctx, logger.WithName("controllers"), &wg)
				}
				wg.Wait()
			},
			nil,
		)
		if err != nil {
			setup.Logger.Error(err, "failed to initialize leader election")
			os.Exit(1)
		}
		urGenerator := generator.NewUpdateRequestGenerator(setup.Configuration, setup.MetadataClient)
		urgen := webhookgenerate.NewGenerator(
			setup.KyvernoClient,
			kyvernoInformer.Kyverno().V2().UpdateRequests(),
			urGenerator,
		)
		policyHandlers := webhookspolicy.NewHandlers(
			setup.KyvernoDynamicClient,
			setup.RegistrySecretLister,
			backgroundServiceAccountName,
			reportsServiceAccountName,
		)

		contextProvider, err := libs.NewContextProvider(
			setup.KyvernoDynamicClient,
			setup.RegistrySecretLister,
			gcstore,
			restMapper,
			false,
		)
		if err != nil {
			setup.Logger.Error(err, "failed to create cel context provider")
			os.Exit(1)
		}

		nsLister := kubeInformer.Core().V1().Namespaces().Lister()
		nsResolver := celengine.NewNamespaceResolver(setup.Logger.WithName("ns-resolver"), nsLister, setup.KubeClient)

		var vpolEngine vpolengine.Engine
		var ivpolEngine ivpolengine.Engine
		var mpolEngine mpolengine.Engine
		{
			scheme := kruntime.NewScheme()
			if err := policiesv1beta1.Install(scheme); err != nil {
				setup.Logger.Error(err, "failed to initialize scheme")
				os.Exit(1)
			}
			mgr, err := ctrl.NewManager(setup.RestConfig, ctrl.Options{
				Scheme: scheme,
				Metrics: server.Options{
					BindAddress: controllerRuntimeMetricsAddress,
				},
			})
			if err != nil {
				setup.Logger.Error(err, "failed to construct manager")
				os.Exit(1)
			}
			celExceptionLister := celengine.NewPolicyExceptionLister(kyvernoInformer.Policies().V1beta1().PolicyExceptions().Lister(), internal.ExceptionNamespace())
			compiler := vpolcompiler.NewCompiler()
			vpolProvider, err := vpolengine.NewKubeProvider(
				compiler,
				mgr,
				celExceptionLister,
				internal.PolicyExceptionEnabled(),
			)
			if err != nil {
				setup.Logger.Error(err, "failed to create vpol provider")
				os.Exit(1)
			}
			ivpolProvider, err := ivpolengine.NewKubeProvider(mgr, celExceptionLister, internal.PolicyExceptionEnabled())
			if err != nil {
				setup.Logger.Error(err, "failed to create ivpol provider")
				os.Exit(1)
			}
			mpolcompiler := mpolcompiler.NewCompiler()
			mpolProvider, typeConverter, err := mpolengine.NewKubeProvider(signalCtx, mpolcompiler, contextProvider, mgr, setup.KubeClient.Discovery().OpenAPIV3(), celExceptionLister, internal.PolicyExceptionEnabled())
			if err != nil {
				setup.Logger.Error(err, "failed to create mpol provider")
				os.Exit(1)
			}
			ctx, cancel := context.WithCancel(signalCtx)
			wg.StartWithContext(ctx, func(ctx context.Context) {
				defer cancel()
				if err := mgr.Start(ctx); err != nil {
					setup.Logger.Error(err, "failed to start manager")
					os.Exit(1)
				}
			})
			if !mgr.GetCache().WaitForCacheSync(ctx) {
				defer cancel()
				setup.Logger.Error(err, "failed to create policy provider")
				os.Exit(1)
			}
			vpolEngine = vpolengine.NewMetricWrapper(vpolengine.NewEngine(
				vpolProvider,
				nsResolver,
				matching.NewMatcher(),
			), metrics.AdmissionRequest)

			ivpolEngine = ivpolengine.NewMetricWrapper(ivpolengine.NewEngine(
				ivpolProvider,
				nsResolver,
				matching.NewMatcher(),
				setup.RegistrySecretLister,
				setup.ImageVerifyCacheClient,
				setup.Configuration,
			), metrics.AdmissionRequest)
			mpolEngine = mpolengine.NewMetricWrapper(mpolengine.NewEngine(
				mpolProvider,
				nsResolver,
				matching.NewMatcher(),
				typeConverter,
				contextProvider,
			), metrics.AdmissionRequest)
		}
		if admissionReports {
			ephrCounterFunc := func(c breaker.Counter) func(context.Context) bool {
				return func(context.Context) bool {
					count, isRunning := c.Count()
					if !isRunning {
						return true
					}
					return count > maxAdmissionReports
				}
			}

			ephrs, err := breaker.StartAdmissionReportsCounter(signalCtx, setup.MetadataClient)
			if err != nil {
				go func() {
					for {
						ephrs, err := breaker.StartAdmissionReportsCounter(signalCtx, setup.MetadataClient)
						if err != nil {
							setup.Logger.Error(err, "failed to start admission reports watcher, retrying...")
							time.Sleep(2 * time.Second)
							continue
						}
						breaker.SetReportsBreaker(breaker.NewBreaker("admission reports", ephrCounterFunc(ephrs)))
						return
					}
				}()
				breaker.SetReportsBreaker(breaker.NewBreaker("admission reports", func(context.Context) bool {
					return true
				}))
			} else {
				breaker.SetReportsBreaker(breaker.NewBreaker("admission reports", ephrCounterFunc(ephrs)))
			}
		} else {
			breaker.SetReportsBreaker(breaker.NewBreaker("admission reports", func(context.Context) bool {
				return true
			}))
		}

		resourceHandlers := webhooksresource.NewHandlers(
			engine,
			setup.KyvernoDynamicClient,
			setup.KyvernoClient,
			setup.Configuration,
			setup.MetricsManager,
			policyCache,
			nsLister,
			kyvernoInformer.Kyverno().V2().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
			kyvernoInformer.Kyverno().V1().ClusterPolicies(),
			kyvernoInformer.Kyverno().V1().Policies(),
			urgen,
			eventGenerator,
			admissionReports,
			backgroundServiceAccountName,
			reportsServiceAccountName,
			setup.Jp,
			maxAuditWorkers,
			maxAuditCapacity,
		)
		voplHandlers := vpol.New(
			vpolEngine,
			contextProvider,
			setup.KyvernoClient,
			admissionReports,
			eventGenerator,
		)
		ivpolHandlers := ivpol.New(
			ivpolEngine,
			contextProvider,
			setup.KyvernoClient,
			admissionReports,
			eventGenerator,
		)
		gpolHandlers := gpol.New(urgen, kyvernoInformer.Policies().V1beta1().GeneratingPolicies().Lister(), kyvernoInformer.Policies().V1beta1().NamespacedGeneratingPolicies().Lister(), backgroundServiceAccountName)
		exceptionHandlers := webhooksexception.NewHandlers(exception.ValidationOptions{
			Enabled:   internal.PolicyExceptionEnabled(),
			Namespace: internal.ExceptionNamespace(),
		})
		mpolHandlers := mpol.New(contextProvider, mpolEngine, setup.KyvernoClient, setup.ReportingConfiguration, urgen, backgroundServiceAccountName, eventGenerator)
		celExceptionHandlers := webhookscelexception.NewHandlers(exception.ValidationOptions{
			Enabled: internal.PolicyExceptionEnabled(),
		})
		globalContextHandlers := webhooksglobalcontext.NewHandlers()
		server := webhooks.NewServer(
			signalCtx,
			webhooks.PolicyHandlers{
				Mutation:   webhooks.HandlerFunc(policyHandlers.Mutate),
				Validation: webhooks.HandlerFunc(policyHandlers.Validate),
			},
			webhooks.ResourceHandlers{
				Mutation:                          webhooks.HandlerFunc(resourceHandlers.Mutate),
				ImageVerificationPoliciesMutation: webhooks.HandlerFunc(ivpolHandlers.MutateClustered),
				NamespacedImageVerificationPoliciesMutation: webhooks.HandlerFunc(ivpolHandlers.MutateNamespaced),
				MutatingPolicies:                    webhooks.HandlerFunc(mpolHandlers.MutateClustered),
				NamespacedMutatingPolicies:          webhooks.HandlerFunc(mpolHandlers.MutateNamespaced),
				Validation:                          webhooks.HandlerFunc(resourceHandlers.Validate),
				ValidatingPolicies:                  webhooks.HandlerFunc(voplHandlers.ValidateClustered),
				NamespacedValidatingPolicies:        webhooks.HandlerFunc(voplHandlers.ValidateNamespaced),
				ImageVerificationPolicies:           webhooks.HandlerFunc(ivpolHandlers.ValidateClustered),
				NamespacedImageVerificationPolicies: webhooks.HandlerFunc(ivpolHandlers.ValidateNamespaced),
				GeneratingPolicies:                  webhooks.HandlerFunc(gpolHandlers.Generate),
				NamespacedGeneratingPolicies:        webhooks.HandlerFunc(gpolHandlers.GenerateNamespaced),
			},
			webhooks.ExceptionHandlers{
				Validation: webhooks.HandlerFunc(exceptionHandlers.Validate),
			},
			webhooks.CELExceptionHandlers{
				Validation: webhooks.HandlerFunc(celExceptionHandlers.Validate),
			},
			webhooks.GlobalContextHandlers{
				Validation: webhooks.HandlerFunc(globalContextHandlers.Validate),
			},
			setup.Configuration,
			setup.MetricsManager,
			webhooks.DebugModeOptions{
				DumpPayload: dumpPayload,
			},
			func() ([]byte, []byte, error) {
				secret, err := tlsSecret.Lister().Secrets(config.KyvernoNamespace()).Get(tlsSecretName)
				if err != nil {
					return nil, nil, err
				}
				return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
			},
			setup.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
			setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
			setup.KubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
			runtime,
			kubeInformer.Rbac().V1().RoleBindings().Lister(),
			kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
			setup.KyvernoDynamicClient.Discovery(),
			webhookServerHost,
			int32(webhookServerPort), //nolint:gosec
		)
		if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		server.Run()
		defer server.Stop()
		eventController.Run(signalCtx, setup.Logger, &wg)
		gceController.Run(signalCtx, setup.Logger, &wg)
		if polexController != nil {
			polexController.Run(signalCtx, setup.Logger, &wg)
		}
		for _, controller := range nonLeaderControllers {
			controller.Run(signalCtx, setup.Logger.WithName("controllers"), &wg)
		}
		le.Run(signalCtx)
	}()
	wg.Wait()
}