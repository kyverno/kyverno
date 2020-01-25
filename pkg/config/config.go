package config

import (
	"flag"

	"github.com/golang/glog"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml

const (
	//KubePolicyNamespace default kyverno namespace
	KubePolicyNamespace = "kyverno"
	//WebhookServiceName default kyverno webhook service name
	WebhookServiceName = "kyverno-svc"

	//MutatingWebhookConfigurationName default resource mutating webhook configuration name
	MutatingWebhookConfigurationName = "kyverno-resource-mutating-webhook-cfg"
	//MutatingWebhookConfigurationDebugName default resource mutating webhook configuration name for debug mode
	MutatingWebhookConfigurationDebugName = "kyverno-resource-mutating-webhook-cfg-debug"
	//MutatingWebhookName default resource mutating webhook name
	MutatingWebhookName = "nirmata.kyverno.resource.mutating-webhook"

	// ValidatingWebhookConfigurationName  = "kyverno-validating-webhook-cfg"
	// ValidatingWebhookConfigurationDebug = "kyverno-validating-webhook-cfg-debug"
	// ValidatingWebhookName               = "nirmata.kyverno.policy-validating-webhook"

	//VerifyMutatingWebhookConfigurationName default verify mutating webhook configuration name
	VerifyMutatingWebhookConfigurationName = "kyverno-verify-mutating-webhook-cfg"
	//VerifyMutatingWebhookConfigurationDebugName default verify mutating webhook configuration name for debug mode
	VerifyMutatingWebhookConfigurationDebugName = "kyverno-verify-mutating-webhook-cfg-debug"
	//VerifyMutatingWebhookName default verify mutating webhook name
	VerifyMutatingWebhookName = "nirmata.kyverno.verify-mutating-webhook"

	//PolicyValidatingWebhookConfigurationName default policy validating webhook configuration name
	PolicyValidatingWebhookConfigurationName = "kyverno-policy-validating-webhook-cfg"
	//PolicyValidatingWebhookConfigurationDebugName default policy validating webhook configuration name for debug mode
	PolicyValidatingWebhookConfigurationDebugName = "kyverno-policy-validating-webhook-cfg-debug"
	//PolicyValidatingWebhookName default policy validating webhook name
	PolicyValidatingWebhookName = "nirmata.kyverno.policy-validating-webhook"

	//PolicyMutatingWebhookConfigurationName default policy mutating webhook configuration name
	PolicyMutatingWebhookConfigurationName = "kyverno-policy-mutating-webhook-cfg"
	//PolicyMutatingWebhookConfigurationDebugName default policy mutating webhook configuration name for debug mode
	PolicyMutatingWebhookConfigurationDebugName = "kyverno-policy-mutating-webhook-cfg-debug"
	//PolicyMutatingWebhookName default policy mutating webhook name
	PolicyMutatingWebhookName = "nirmata.kyverno.policy-mutating-webhook"

	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants

	// DeploymentKind define the default deployment resource kind
	DeploymentKind = "Deployment"

	// DeploymentAPIVersion define the default deployment resource apiVersion
	DeploymentAPIVersion = "extensions/v1beta1"
	// KubePolicyDeploymentName define the default deployment namespace
	KubePolicyDeploymentName = "kyverno"
)

var (
	//MutatingWebhookServicePath is the path for mutation webhook
	MutatingWebhookServicePath = "/mutate"
	//ValidatingWebhookServicePath is the path for validation webhook
	ValidatingWebhookServicePath = "/validate"
	//PolicyValidatingWebhookServicePath is the path for policy validation webhook(used to validate policy resource)
	PolicyValidatingWebhookServicePath = "/policyvalidate"
	//PolicyMutatingWebhookServicePath is the path for policy mutation webhook(used to default)
	PolicyMutatingWebhookServicePath = "/policymutate"
	//VerifyMutatingWebhookServicePath is the path for verify webhook(used to veryfing if admission control is enabled and active)
	VerifyMutatingWebhookServicePath = "/verifymutate"
)

//LogDefaultFlags sets default glog flags
func LogDefaultFlags() {
	var err error
	err = flag.Set("logtostderr", "true")
	if err != nil {
		glog.Fatalf("failed to set flag 'logtostderr' to 'true':%v", err)
	}
	err = flag.Set("stderrthreshold", "WARNING")
	if err != nil {
		glog.Fatalf("failed to set flag 'stderrthreshold' to 'WARNING':%v", err)
	}
	flag.Set("v", "2")
	if err != nil {
		glog.Fatalf("failed to set flag 'v' to '2':%v", err)
	}
}

//CreateClientConfig creates client config
func CreateClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		glog.Info("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	glog.Infof("Using configuration from '%s'", kubeconfig)
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
