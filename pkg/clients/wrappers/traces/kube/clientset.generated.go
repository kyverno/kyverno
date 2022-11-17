package client

import (
	context "context"

	github_com_kyverno_kyverno_pkg_tracing "github.com/kyverno/kyverno/pkg/tracing"
	go_opentelemetry_io_otel_attribute "go.opentelemetry.io/otel/attribute"
	k8s_io_api_admissionregistration_v1 "k8s.io/api/admissionregistration/v1"
	k8s_io_api_admissionregistration_v1beta1 "k8s.io/api/admissionregistration/v1beta1"
	k8s_io_api_apiserverinternal_v1alpha1 "k8s.io/api/apiserverinternal/v1alpha1"
	k8s_io_api_apps_v1 "k8s.io/api/apps/v1"
	k8s_io_api_apps_v1beta1 "k8s.io/api/apps/v1beta1"
	k8s_io_api_apps_v1beta2 "k8s.io/api/apps/v1beta2"
	k8s_io_api_authentication_v1 "k8s.io/api/authentication/v1"
	k8s_io_api_authentication_v1beta1 "k8s.io/api/authentication/v1beta1"
	k8s_io_api_authorization_v1 "k8s.io/api/authorization/v1"
	k8s_io_api_authorization_v1beta1 "k8s.io/api/authorization/v1beta1"
	k8s_io_api_autoscaling_v1 "k8s.io/api/autoscaling/v1"
	k8s_io_api_autoscaling_v2 "k8s.io/api/autoscaling/v2"
	k8s_io_api_autoscaling_v2beta1 "k8s.io/api/autoscaling/v2beta1"
	k8s_io_api_autoscaling_v2beta2 "k8s.io/api/autoscaling/v2beta2"
	k8s_io_api_batch_v1 "k8s.io/api/batch/v1"
	k8s_io_api_batch_v1beta1 "k8s.io/api/batch/v1beta1"
	k8s_io_api_certificates_v1 "k8s.io/api/certificates/v1"
	k8s_io_api_certificates_v1beta1 "k8s.io/api/certificates/v1beta1"
	k8s_io_api_coordination_v1 "k8s.io/api/coordination/v1"
	k8s_io_api_coordination_v1beta1 "k8s.io/api/coordination/v1beta1"
	k8s_io_api_core_v1 "k8s.io/api/core/v1"
	k8s_io_api_discovery_v1 "k8s.io/api/discovery/v1"
	k8s_io_api_discovery_v1beta1 "k8s.io/api/discovery/v1beta1"
	k8s_io_api_events_v1 "k8s.io/api/events/v1"
	k8s_io_api_events_v1beta1 "k8s.io/api/events/v1beta1"
	k8s_io_api_extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	k8s_io_api_flowcontrol_v1alpha1 "k8s.io/api/flowcontrol/v1alpha1"
	k8s_io_api_flowcontrol_v1beta1 "k8s.io/api/flowcontrol/v1beta1"
	k8s_io_api_flowcontrol_v1beta2 "k8s.io/api/flowcontrol/v1beta2"
	k8s_io_api_networking_v1 "k8s.io/api/networking/v1"
	k8s_io_api_networking_v1alpha1 "k8s.io/api/networking/v1alpha1"
	k8s_io_api_networking_v1beta1 "k8s.io/api/networking/v1beta1"
	k8s_io_api_node_v1 "k8s.io/api/node/v1"
	k8s_io_api_node_v1alpha1 "k8s.io/api/node/v1alpha1"
	k8s_io_api_node_v1beta1 "k8s.io/api/node/v1beta1"
	k8s_io_api_policy_v1 "k8s.io/api/policy/v1"
	k8s_io_api_policy_v1beta1 "k8s.io/api/policy/v1beta1"
	k8s_io_api_rbac_v1 "k8s.io/api/rbac/v1"
	k8s_io_api_rbac_v1alpha1 "k8s.io/api/rbac/v1alpha1"
	k8s_io_api_rbac_v1beta1 "k8s.io/api/rbac/v1beta1"
	k8s_io_api_scheduling_v1 "k8s.io/api/scheduling/v1"
	k8s_io_api_scheduling_v1alpha1 "k8s.io/api/scheduling/v1alpha1"
	k8s_io_api_scheduling_v1beta1 "k8s.io/api/scheduling/v1beta1"
	k8s_io_api_storage_v1 "k8s.io/api/storage/v1"
	k8s_io_api_storage_v1alpha1 "k8s.io/api/storage/v1alpha1"
	k8s_io_api_storage_v1beta1 "k8s.io/api/storage/v1beta1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_fields "k8s.io/apimachinery/pkg/fields"
	k8s_io_apimachinery_pkg_runtime "k8s.io/apimachinery/pkg/runtime"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_applyconfigurations_admissionregistration_v1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1"
	k8s_io_client_go_applyconfigurations_admissionregistration_v1beta1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1beta1"
	k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1 "k8s.io/client-go/applyconfigurations/apiserverinternal/v1alpha1"
	k8s_io_client_go_applyconfigurations_apps_v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	k8s_io_client_go_applyconfigurations_apps_v1beta1 "k8s.io/client-go/applyconfigurations/apps/v1beta1"
	k8s_io_client_go_applyconfigurations_apps_v1beta2 "k8s.io/client-go/applyconfigurations/apps/v1beta2"
	k8s_io_client_go_applyconfigurations_autoscaling_v1 "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	k8s_io_client_go_applyconfigurations_autoscaling_v2 "k8s.io/client-go/applyconfigurations/autoscaling/v2"
	k8s_io_client_go_applyconfigurations_autoscaling_v2beta1 "k8s.io/client-go/applyconfigurations/autoscaling/v2beta1"
	k8s_io_client_go_applyconfigurations_autoscaling_v2beta2 "k8s.io/client-go/applyconfigurations/autoscaling/v2beta2"
	k8s_io_client_go_applyconfigurations_batch_v1 "k8s.io/client-go/applyconfigurations/batch/v1"
	k8s_io_client_go_applyconfigurations_batch_v1beta1 "k8s.io/client-go/applyconfigurations/batch/v1beta1"
	k8s_io_client_go_applyconfigurations_certificates_v1 "k8s.io/client-go/applyconfigurations/certificates/v1"
	k8s_io_client_go_applyconfigurations_certificates_v1beta1 "k8s.io/client-go/applyconfigurations/certificates/v1beta1"
	k8s_io_client_go_applyconfigurations_coordination_v1 "k8s.io/client-go/applyconfigurations/coordination/v1"
	k8s_io_client_go_applyconfigurations_coordination_v1beta1 "k8s.io/client-go/applyconfigurations/coordination/v1beta1"
	k8s_io_client_go_applyconfigurations_core_v1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s_io_client_go_applyconfigurations_discovery_v1 "k8s.io/client-go/applyconfigurations/discovery/v1"
	k8s_io_client_go_applyconfigurations_discovery_v1beta1 "k8s.io/client-go/applyconfigurations/discovery/v1beta1"
	k8s_io_client_go_applyconfigurations_events_v1 "k8s.io/client-go/applyconfigurations/events/v1"
	k8s_io_client_go_applyconfigurations_events_v1beta1 "k8s.io/client-go/applyconfigurations/events/v1beta1"
	k8s_io_client_go_applyconfigurations_extensions_v1beta1 "k8s.io/client-go/applyconfigurations/extensions/v1beta1"
	k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1 "k8s.io/client-go/applyconfigurations/flowcontrol/v1alpha1"
	k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1 "k8s.io/client-go/applyconfigurations/flowcontrol/v1beta1"
	k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2 "k8s.io/client-go/applyconfigurations/flowcontrol/v1beta2"
	k8s_io_client_go_applyconfigurations_networking_v1 "k8s.io/client-go/applyconfigurations/networking/v1"
	k8s_io_client_go_applyconfigurations_networking_v1alpha1 "k8s.io/client-go/applyconfigurations/networking/v1alpha1"
	k8s_io_client_go_applyconfigurations_networking_v1beta1 "k8s.io/client-go/applyconfigurations/networking/v1beta1"
	k8s_io_client_go_applyconfigurations_node_v1 "k8s.io/client-go/applyconfigurations/node/v1"
	k8s_io_client_go_applyconfigurations_node_v1alpha1 "k8s.io/client-go/applyconfigurations/node/v1alpha1"
	k8s_io_client_go_applyconfigurations_node_v1beta1 "k8s.io/client-go/applyconfigurations/node/v1beta1"
	k8s_io_client_go_applyconfigurations_policy_v1 "k8s.io/client-go/applyconfigurations/policy/v1"
	k8s_io_client_go_applyconfigurations_policy_v1beta1 "k8s.io/client-go/applyconfigurations/policy/v1beta1"
	k8s_io_client_go_applyconfigurations_rbac_v1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	k8s_io_client_go_applyconfigurations_rbac_v1alpha1 "k8s.io/client-go/applyconfigurations/rbac/v1alpha1"
	k8s_io_client_go_applyconfigurations_rbac_v1beta1 "k8s.io/client-go/applyconfigurations/rbac/v1beta1"
	k8s_io_client_go_applyconfigurations_scheduling_v1 "k8s.io/client-go/applyconfigurations/scheduling/v1"
	k8s_io_client_go_applyconfigurations_scheduling_v1alpha1 "k8s.io/client-go/applyconfigurations/scheduling/v1alpha1"
	k8s_io_client_go_applyconfigurations_scheduling_v1beta1 "k8s.io/client-go/applyconfigurations/scheduling/v1beta1"
	k8s_io_client_go_applyconfigurations_storage_v1 "k8s.io/client-go/applyconfigurations/storage/v1"
	k8s_io_client_go_applyconfigurations_storage_v1alpha1 "k8s.io/client-go/applyconfigurations/storage/v1alpha1"
	k8s_io_client_go_applyconfigurations_storage_v1beta1 "k8s.io/client-go/applyconfigurations/storage/v1beta1"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_kubernetes "k8s.io/client-go/kubernetes"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1 "k8s.io/client-go/kubernetes/typed/apiserverinternal/v1alpha1"
	k8s_io_client_go_kubernetes_typed_apps_v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	k8s_io_client_go_kubernetes_typed_apps_v1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	k8s_io_client_go_kubernetes_typed_apps_v1beta2 "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	k8s_io_client_go_kubernetes_typed_authentication_v1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	k8s_io_client_go_kubernetes_typed_authentication_v1beta1 "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	k8s_io_client_go_kubernetes_typed_authorization_v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	k8s_io_client_go_kubernetes_typed_authorization_v1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	k8s_io_client_go_kubernetes_typed_autoscaling_v1 "k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	k8s_io_client_go_kubernetes_typed_autoscaling_v2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2"
	k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta1"
	k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta2"
	k8s_io_client_go_kubernetes_typed_batch_v1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	k8s_io_client_go_kubernetes_typed_batch_v1beta1 "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	k8s_io_client_go_kubernetes_typed_certificates_v1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	k8s_io_client_go_kubernetes_typed_certificates_v1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	k8s_io_client_go_kubernetes_typed_coordination_v1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	k8s_io_client_go_kubernetes_typed_coordination_v1beta1 "k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	k8s_io_client_go_kubernetes_typed_core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8s_io_client_go_kubernetes_typed_discovery_v1 "k8s.io/client-go/kubernetes/typed/discovery/v1"
	k8s_io_client_go_kubernetes_typed_discovery_v1beta1 "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	k8s_io_client_go_kubernetes_typed_events_v1 "k8s.io/client-go/kubernetes/typed/events/v1"
	k8s_io_client_go_kubernetes_typed_events_v1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	k8s_io_client_go_kubernetes_typed_extensions_v1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1alpha1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta2"
	k8s_io_client_go_kubernetes_typed_networking_v1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	k8s_io_client_go_kubernetes_typed_networking_v1alpha1 "k8s.io/client-go/kubernetes/typed/networking/v1alpha1"
	k8s_io_client_go_kubernetes_typed_networking_v1beta1 "k8s.io/client-go/kubernetes/typed/networking/v1beta1"
	k8s_io_client_go_kubernetes_typed_node_v1 "k8s.io/client-go/kubernetes/typed/node/v1"
	k8s_io_client_go_kubernetes_typed_node_v1alpha1 "k8s.io/client-go/kubernetes/typed/node/v1alpha1"
	k8s_io_client_go_kubernetes_typed_node_v1beta1 "k8s.io/client-go/kubernetes/typed/node/v1beta1"
	k8s_io_client_go_kubernetes_typed_policy_v1 "k8s.io/client-go/kubernetes/typed/policy/v1"
	k8s_io_client_go_kubernetes_typed_policy_v1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	k8s_io_client_go_kubernetes_typed_rbac_v1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	k8s_io_client_go_kubernetes_typed_rbac_v1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	k8s_io_client_go_kubernetes_typed_rbac_v1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	k8s_io_client_go_kubernetes_typed_scheduling_v1 "k8s.io/client-go/kubernetes/typed/scheduling/v1"
	k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	k8s_io_client_go_kubernetes_typed_scheduling_v1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	k8s_io_client_go_kubernetes_typed_storage_v1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	k8s_io_client_go_kubernetes_typed_storage_v1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	k8s_io_client_go_kubernetes_typed_storage_v1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

// Wrap
func Wrap(inner k8s_io_client_go_kubernetes.Interface) k8s_io_client_go_kubernetes.Interface {
	return &clientset{
		inner:                        inner,
		admissionregistrationv1:      newAdmissionregistrationV1(inner.AdmissionregistrationV1()),
		admissionregistrationv1beta1: newAdmissionregistrationV1beta1(inner.AdmissionregistrationV1beta1()),
		appsv1:                       newAppsV1(inner.AppsV1()),
		appsv1beta1:                  newAppsV1beta1(inner.AppsV1beta1()),
		appsv1beta2:                  newAppsV1beta2(inner.AppsV1beta2()),
		authenticationv1:             newAuthenticationV1(inner.AuthenticationV1()),
		authenticationv1beta1:        newAuthenticationV1beta1(inner.AuthenticationV1beta1()),
		authorizationv1:              newAuthorizationV1(inner.AuthorizationV1()),
		authorizationv1beta1:         newAuthorizationV1beta1(inner.AuthorizationV1beta1()),
		autoscalingv1:                newAutoscalingV1(inner.AutoscalingV1()),
		autoscalingv2:                newAutoscalingV2(inner.AutoscalingV2()),
		autoscalingv2beta1:           newAutoscalingV2beta1(inner.AutoscalingV2beta1()),
		autoscalingv2beta2:           newAutoscalingV2beta2(inner.AutoscalingV2beta2()),
		batchv1:                      newBatchV1(inner.BatchV1()),
		batchv1beta1:                 newBatchV1beta1(inner.BatchV1beta1()),
		certificatesv1:               newCertificatesV1(inner.CertificatesV1()),
		certificatesv1beta1:          newCertificatesV1beta1(inner.CertificatesV1beta1()),
		coordinationv1:               newCoordinationV1(inner.CoordinationV1()),
		coordinationv1beta1:          newCoordinationV1beta1(inner.CoordinationV1beta1()),
		corev1:                       newCoreV1(inner.CoreV1()),
		discoveryv1:                  newDiscoveryV1(inner.DiscoveryV1()),
		discoveryv1beta1:             newDiscoveryV1beta1(inner.DiscoveryV1beta1()),
		eventsv1:                     newEventsV1(inner.EventsV1()),
		eventsv1beta1:                newEventsV1beta1(inner.EventsV1beta1()),
		extensionsv1beta1:            newExtensionsV1beta1(inner.ExtensionsV1beta1()),
		flowcontrolv1alpha1:          newFlowcontrolV1alpha1(inner.FlowcontrolV1alpha1()),
		flowcontrolv1beta1:           newFlowcontrolV1beta1(inner.FlowcontrolV1beta1()),
		flowcontrolv1beta2:           newFlowcontrolV1beta2(inner.FlowcontrolV1beta2()),
		internalv1alpha1:             newInternalV1alpha1(inner.InternalV1alpha1()),
		networkingv1:                 newNetworkingV1(inner.NetworkingV1()),
		networkingv1alpha1:           newNetworkingV1alpha1(inner.NetworkingV1alpha1()),
		networkingv1beta1:            newNetworkingV1beta1(inner.NetworkingV1beta1()),
		nodev1:                       newNodeV1(inner.NodeV1()),
		nodev1alpha1:                 newNodeV1alpha1(inner.NodeV1alpha1()),
		nodev1beta1:                  newNodeV1beta1(inner.NodeV1beta1()),
		policyv1:                     newPolicyV1(inner.PolicyV1()),
		policyv1beta1:                newPolicyV1beta1(inner.PolicyV1beta1()),
		rbacv1:                       newRbacV1(inner.RbacV1()),
		rbacv1alpha1:                 newRbacV1alpha1(inner.RbacV1alpha1()),
		rbacv1beta1:                  newRbacV1beta1(inner.RbacV1beta1()),
		schedulingv1:                 newSchedulingV1(inner.SchedulingV1()),
		schedulingv1alpha1:           newSchedulingV1alpha1(inner.SchedulingV1alpha1()),
		schedulingv1beta1:            newSchedulingV1beta1(inner.SchedulingV1beta1()),
		storagev1:                    newStorageV1(inner.StorageV1()),
		storagev1alpha1:              newStorageV1alpha1(inner.StorageV1alpha1()),
		storagev1beta1:               newStorageV1beta1(inner.StorageV1beta1()),
	}
}

// NewForConfig
func NewForConfig(c *k8s_io_client_go_rest.Config) (k8s_io_client_go_kubernetes.Interface, error) {
	inner, err := k8s_io_client_go_kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return Wrap(inner), nil
}

// clientset wrapper
type clientset struct {
	inner                        k8s_io_client_go_kubernetes.Interface
	admissionregistrationv1      k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	admissionregistrationv1beta1 k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface
	appsv1                       k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
	appsv1beta1                  k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface
	appsv1beta2                  k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface
	authenticationv1             k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	authenticationv1beta1        k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface
	authorizationv1              k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	authorizationv1beta1         k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface
	autoscalingv1                k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface
	autoscalingv2                k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
	autoscalingv2beta1           k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface
	autoscalingv2beta2           k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface
	batchv1                      k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
	batchv1beta1                 k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	certificatesv1               k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface
	certificatesv1beta1          k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	coordinationv1               k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	coordinationv1beta1          k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface
	corev1                       k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	discoveryv1                  k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface
	discoveryv1beta1             k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	eventsv1                     k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface
	eventsv1beta1                k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	extensionsv1beta1            k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	flowcontrolv1alpha1          k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface
	flowcontrolv1beta1           k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	flowcontrolv1beta2           k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface
	internalv1alpha1             k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	networkingv1                 k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface
	networkingv1alpha1           k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	networkingv1beta1            k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	nodev1                       k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	nodev1alpha1                 k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface
	nodev1beta1                  k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface
	policyv1                     k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	policyv1beta1                k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	rbacv1                       k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
	rbacv1alpha1                 k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface
	rbacv1beta1                  k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface
	schedulingv1                 k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface
	schedulingv1alpha1           k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
	schedulingv1beta1            k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	storagev1                    k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface
	storagev1alpha1              k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	storagev1beta1               k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
}

// Discovery is NOT instrumented
func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.inner.Discovery()
}
func (c *clientset) AdmissionregistrationV1() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return c.admissionregistrationv1
}
func (c *clientset) AdmissionregistrationV1beta1() k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface {
	return c.admissionregistrationv1beta1
}
func (c *clientset) AppsV1() k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface {
	return c.appsv1
}
func (c *clientset) AppsV1beta1() k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface {
	return c.appsv1beta1
}
func (c *clientset) AppsV1beta2() k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface {
	return c.appsv1beta2
}
func (c *clientset) AuthenticationV1() k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return c.authenticationv1
}
func (c *clientset) AuthenticationV1beta1() k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface {
	return c.authenticationv1beta1
}
func (c *clientset) AuthorizationV1() k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return c.authorizationv1
}
func (c *clientset) AuthorizationV1beta1() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface {
	return c.authorizationv1beta1
}
func (c *clientset) AutoscalingV1() k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface {
	return c.autoscalingv1
}
func (c *clientset) AutoscalingV2() k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface {
	return c.autoscalingv2
}
func (c *clientset) AutoscalingV2beta1() k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface {
	return c.autoscalingv2beta1
}
func (c *clientset) AutoscalingV2beta2() k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface {
	return c.autoscalingv2beta2
}
func (c *clientset) BatchV1() k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface {
	return c.batchv1
}
func (c *clientset) BatchV1beta1() k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return c.batchv1beta1
}
func (c *clientset) CertificatesV1() k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface {
	return c.certificatesv1
}
func (c *clientset) CertificatesV1beta1() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return c.certificatesv1beta1
}
func (c *clientset) CoordinationV1() k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return c.coordinationv1
}
func (c *clientset) CoordinationV1beta1() k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface {
	return c.coordinationv1beta1
}
func (c *clientset) CoreV1() k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return c.corev1
}
func (c *clientset) DiscoveryV1() k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface {
	return c.discoveryv1
}
func (c *clientset) DiscoveryV1beta1() k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return c.discoveryv1beta1
}
func (c *clientset) EventsV1() k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface {
	return c.eventsv1
}
func (c *clientset) EventsV1beta1() k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return c.eventsv1beta1
}
func (c *clientset) ExtensionsV1beta1() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return c.extensionsv1beta1
}
func (c *clientset) FlowcontrolV1alpha1() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface {
	return c.flowcontrolv1alpha1
}
func (c *clientset) FlowcontrolV1beta1() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return c.flowcontrolv1beta1
}
func (c *clientset) FlowcontrolV1beta2() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface {
	return c.flowcontrolv1beta2
}
func (c *clientset) InternalV1alpha1() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return c.internalv1alpha1
}
func (c *clientset) NetworkingV1() k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface {
	return c.networkingv1
}
func (c *clientset) NetworkingV1alpha1() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return c.networkingv1alpha1
}
func (c *clientset) NetworkingV1beta1() k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return c.networkingv1beta1
}
func (c *clientset) NodeV1() k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return c.nodev1
}
func (c *clientset) NodeV1alpha1() k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface {
	return c.nodev1alpha1
}
func (c *clientset) NodeV1beta1() k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface {
	return c.nodev1beta1
}
func (c *clientset) PolicyV1() k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return c.policyv1
}
func (c *clientset) PolicyV1beta1() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return c.policyv1beta1
}
func (c *clientset) RbacV1() k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface {
	return c.rbacv1
}
func (c *clientset) RbacV1alpha1() k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface {
	return c.rbacv1alpha1
}
func (c *clientset) RbacV1beta1() k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface {
	return c.rbacv1beta1
}
func (c *clientset) SchedulingV1() k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface {
	return c.schedulingv1
}
func (c *clientset) SchedulingV1alpha1() k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface {
	return c.schedulingv1alpha1
}
func (c *clientset) SchedulingV1beta1() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return c.schedulingv1beta1
}
func (c *clientset) StorageV1() k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface {
	return c.storagev1
}
func (c *clientset) StorageV1alpha1() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return c.storagev1alpha1
}
func (c *clientset) StorageV1beta1() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return c.storagev1beta1
}

// wrappedAdmissionregistrationV1 wrapper
type wrappedAdmissionregistrationV1 struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
}

func newAdmissionregistrationV1(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return &wrappedAdmissionregistrationV1{inner}
}
func (c *wrappedAdmissionregistrationV1) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	return newAdmissionregistrationV1MutatingWebhookConfigurations(c.inner.MutatingWebhookConfigurations())
}
func (c *wrappedAdmissionregistrationV1) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	return newAdmissionregistrationV1ValidatingWebhookConfigurations(c.inner.ValidatingWebhookConfigurations())
}

// RESTClient is NOT instrumented
func (c *wrappedAdmissionregistrationV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAdmissionregistrationV1beta1 wrapper
type wrappedAdmissionregistrationV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface
}

func newAdmissionregistrationV1beta1(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface {
	return &wrappedAdmissionregistrationV1beta1{inner}
}
func (c *wrappedAdmissionregistrationV1beta1) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface {
	return newAdmissionregistrationV1beta1MutatingWebhookConfigurations(c.inner.MutatingWebhookConfigurations())
}
func (c *wrappedAdmissionregistrationV1beta1) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface {
	return newAdmissionregistrationV1beta1ValidatingWebhookConfigurations(c.inner.ValidatingWebhookConfigurations())
}

// RESTClient is NOT instrumented
func (c *wrappedAdmissionregistrationV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAppsV1 wrapper
type wrappedAppsV1 struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
}

func newAppsV1(inner k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface) k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface {
	return &wrappedAppsV1{inner}
}
func (c *wrappedAppsV1) ControllerRevisions(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	return newAppsV1ControllerRevisions(c.inner.ControllerRevisions(namespace))
}
func (c *wrappedAppsV1) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	return newAppsV1DaemonSets(c.inner.DaemonSets(namespace))
}
func (c *wrappedAppsV1) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	return newAppsV1Deployments(c.inner.Deployments(namespace))
}
func (c *wrappedAppsV1) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return newAppsV1ReplicaSets(c.inner.ReplicaSets(namespace))
}
func (c *wrappedAppsV1) StatefulSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	return newAppsV1StatefulSets(c.inner.StatefulSets(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAppsV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAppsV1beta1 wrapper
type wrappedAppsV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface
}

func newAppsV1beta1(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface) k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface {
	return &wrappedAppsV1beta1{inner}
}
func (c *wrappedAppsV1beta1) ControllerRevisions(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface {
	return newAppsV1beta1ControllerRevisions(c.inner.ControllerRevisions(namespace))
}
func (c *wrappedAppsV1beta1) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface {
	return newAppsV1beta1Deployments(c.inner.Deployments(namespace))
}
func (c *wrappedAppsV1beta1) StatefulSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface {
	return newAppsV1beta1StatefulSets(c.inner.StatefulSets(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAppsV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAppsV1beta2 wrapper
type wrappedAppsV1beta2 struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface
}

func newAppsV1beta2(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface {
	return &wrappedAppsV1beta2{inner}
}
func (c *wrappedAppsV1beta2) ControllerRevisions(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface {
	return newAppsV1beta2ControllerRevisions(c.inner.ControllerRevisions(namespace))
}
func (c *wrappedAppsV1beta2) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface {
	return newAppsV1beta2DaemonSets(c.inner.DaemonSets(namespace))
}
func (c *wrappedAppsV1beta2) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface {
	return newAppsV1beta2Deployments(c.inner.Deployments(namespace))
}
func (c *wrappedAppsV1beta2) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface {
	return newAppsV1beta2ReplicaSets(c.inner.ReplicaSets(namespace))
}
func (c *wrappedAppsV1beta2) StatefulSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface {
	return newAppsV1beta2StatefulSets(c.inner.StatefulSets(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAppsV1beta2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAuthenticationV1 wrapper
type wrappedAuthenticationV1 struct {
	inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
}

func newAuthenticationV1(inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface) k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return &wrappedAuthenticationV1{inner}
}
func (c *wrappedAuthenticationV1) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return newAuthenticationV1TokenReviews(c.inner.TokenReviews())
}

// RESTClient is NOT instrumented
func (c *wrappedAuthenticationV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAuthenticationV1beta1 wrapper
type wrappedAuthenticationV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface
}

func newAuthenticationV1beta1(inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface) k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface {
	return &wrappedAuthenticationV1beta1{inner}
}
func (c *wrappedAuthenticationV1beta1) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface {
	return newAuthenticationV1beta1TokenReviews(c.inner.TokenReviews())
}

// RESTClient is NOT instrumented
func (c *wrappedAuthenticationV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAuthorizationV1 wrapper
type wrappedAuthorizationV1 struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
}

func newAuthorizationV1(inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface) k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return &wrappedAuthorizationV1{inner}
}
func (c *wrappedAuthorizationV1) LocalSubjectAccessReviews(namespace string) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	return newAuthorizationV1LocalSubjectAccessReviews(c.inner.LocalSubjectAccessReviews(namespace))
}
func (c *wrappedAuthorizationV1) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	return newAuthorizationV1SelfSubjectAccessReviews(c.inner.SelfSubjectAccessReviews())
}
func (c *wrappedAuthorizationV1) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	return newAuthorizationV1SelfSubjectRulesReviews(c.inner.SelfSubjectRulesReviews())
}
func (c *wrappedAuthorizationV1) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	return newAuthorizationV1SubjectAccessReviews(c.inner.SubjectAccessReviews())
}

// RESTClient is NOT instrumented
func (c *wrappedAuthorizationV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAuthorizationV1beta1 wrapper
type wrappedAuthorizationV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface
}

func newAuthorizationV1beta1(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface {
	return &wrappedAuthorizationV1beta1{inner}
}
func (c *wrappedAuthorizationV1beta1) LocalSubjectAccessReviews(namespace string) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface {
	return newAuthorizationV1beta1LocalSubjectAccessReviews(c.inner.LocalSubjectAccessReviews(namespace))
}
func (c *wrappedAuthorizationV1beta1) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface {
	return newAuthorizationV1beta1SelfSubjectAccessReviews(c.inner.SelfSubjectAccessReviews())
}
func (c *wrappedAuthorizationV1beta1) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return newAuthorizationV1beta1SelfSubjectRulesReviews(c.inner.SelfSubjectRulesReviews())
}
func (c *wrappedAuthorizationV1beta1) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface {
	return newAuthorizationV1beta1SubjectAccessReviews(c.inner.SubjectAccessReviews())
}

// RESTClient is NOT instrumented
func (c *wrappedAuthorizationV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAutoscalingV1 wrapper
type wrappedAutoscalingV1 struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface
}

func newAutoscalingV1(inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface) k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface {
	return &wrappedAutoscalingV1{inner}
}
func (c *wrappedAutoscalingV1) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface {
	return newAutoscalingV1HorizontalPodAutoscalers(c.inner.HorizontalPodAutoscalers(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAutoscalingV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAutoscalingV2 wrapper
type wrappedAutoscalingV2 struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
}

func newAutoscalingV2(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface) k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface {
	return &wrappedAutoscalingV2{inner}
}
func (c *wrappedAutoscalingV2) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	return newAutoscalingV2HorizontalPodAutoscalers(c.inner.HorizontalPodAutoscalers(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAutoscalingV2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAutoscalingV2beta1 wrapper
type wrappedAutoscalingV2beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface
}

func newAutoscalingV2beta1(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface {
	return &wrappedAutoscalingV2beta1{inner}
}
func (c *wrappedAutoscalingV2beta1) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface {
	return newAutoscalingV2beta1HorizontalPodAutoscalers(c.inner.HorizontalPodAutoscalers(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAutoscalingV2beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAutoscalingV2beta2 wrapper
type wrappedAutoscalingV2beta2 struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface
}

func newAutoscalingV2beta2(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface {
	return &wrappedAutoscalingV2beta2{inner}
}
func (c *wrappedAutoscalingV2beta2) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface {
	return newAutoscalingV2beta2HorizontalPodAutoscalers(c.inner.HorizontalPodAutoscalers(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedAutoscalingV2beta2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedBatchV1 wrapper
type wrappedBatchV1 struct {
	inner k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
}

func newBatchV1(inner k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface) k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface {
	return &wrappedBatchV1{inner}
}
func (c *wrappedBatchV1) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	return newBatchV1CronJobs(c.inner.CronJobs(namespace))
}
func (c *wrappedBatchV1) Jobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	return newBatchV1Jobs(c.inner.Jobs(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedBatchV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedBatchV1beta1 wrapper
type wrappedBatchV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
}

func newBatchV1beta1(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface) k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return &wrappedBatchV1beta1{inner}
}
func (c *wrappedBatchV1beta1) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	return newBatchV1beta1CronJobs(c.inner.CronJobs(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedBatchV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedCertificatesV1 wrapper
type wrappedCertificatesV1 struct {
	inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface
}

func newCertificatesV1(inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface) k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface {
	return &wrappedCertificatesV1{inner}
}
func (c *wrappedCertificatesV1) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface {
	return newCertificatesV1CertificateSigningRequests(c.inner.CertificateSigningRequests())
}

// RESTClient is NOT instrumented
func (c *wrappedCertificatesV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedCertificatesV1beta1 wrapper
type wrappedCertificatesV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
}

func newCertificatesV1beta1(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return &wrappedCertificatesV1beta1{inner}
}
func (c *wrappedCertificatesV1beta1) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	return newCertificatesV1beta1CertificateSigningRequests(c.inner.CertificateSigningRequests())
}

// RESTClient is NOT instrumented
func (c *wrappedCertificatesV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedCoordinationV1 wrapper
type wrappedCoordinationV1 struct {
	inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
}

func newCoordinationV1(inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface) k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return &wrappedCoordinationV1{inner}
}
func (c *wrappedCoordinationV1) Leases(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	return newCoordinationV1Leases(c.inner.Leases(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedCoordinationV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedCoordinationV1beta1 wrapper
type wrappedCoordinationV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface
}

func newCoordinationV1beta1(inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface {
	return &wrappedCoordinationV1beta1{inner}
}
func (c *wrappedCoordinationV1beta1) Leases(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface {
	return newCoordinationV1beta1Leases(c.inner.Leases(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedCoordinationV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedCoreV1 wrapper
type wrappedCoreV1 struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
}

func newCoreV1(inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface) k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return &wrappedCoreV1{inner}
}
func (c *wrappedCoreV1) ComponentStatuses() k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	return newCoreV1ComponentStatuses(c.inner.ComponentStatuses())
}
func (c *wrappedCoreV1) ConfigMaps(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	return newCoreV1ConfigMaps(c.inner.ConfigMaps(namespace))
}
func (c *wrappedCoreV1) Endpoints(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	return newCoreV1Endpoints(c.inner.Endpoints(namespace))
}
func (c *wrappedCoreV1) Events(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return newCoreV1Events(c.inner.Events(namespace))
}
func (c *wrappedCoreV1) LimitRanges(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	return newCoreV1LimitRanges(c.inner.LimitRanges(namespace))
}
func (c *wrappedCoreV1) Namespaces() k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	return newCoreV1Namespaces(c.inner.Namespaces())
}
func (c *wrappedCoreV1) Nodes() k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	return newCoreV1Nodes(c.inner.Nodes())
}
func (c *wrappedCoreV1) PersistentVolumeClaims(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	return newCoreV1PersistentVolumeClaims(c.inner.PersistentVolumeClaims(namespace))
}
func (c *wrappedCoreV1) PersistentVolumes() k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	return newCoreV1PersistentVolumes(c.inner.PersistentVolumes())
}
func (c *wrappedCoreV1) PodTemplates(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	return newCoreV1PodTemplates(c.inner.PodTemplates(namespace))
}
func (c *wrappedCoreV1) Pods(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	return newCoreV1Pods(c.inner.Pods(namespace))
}
func (c *wrappedCoreV1) ReplicationControllers(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	return newCoreV1ReplicationControllers(c.inner.ReplicationControllers(namespace))
}
func (c *wrappedCoreV1) ResourceQuotas(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	return newCoreV1ResourceQuotas(c.inner.ResourceQuotas(namespace))
}
func (c *wrappedCoreV1) Secrets(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	return newCoreV1Secrets(c.inner.Secrets(namespace))
}
func (c *wrappedCoreV1) ServiceAccounts(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	return newCoreV1ServiceAccounts(c.inner.ServiceAccounts(namespace))
}
func (c *wrappedCoreV1) Services(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	return newCoreV1Services(c.inner.Services(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedCoreV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedDiscoveryV1 wrapper
type wrappedDiscoveryV1 struct {
	inner k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface
}

func newDiscoveryV1(inner k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface) k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface {
	return &wrappedDiscoveryV1{inner}
}
func (c *wrappedDiscoveryV1) EndpointSlices(namespace string) k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface {
	return newDiscoveryV1EndpointSlices(c.inner.EndpointSlices(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedDiscoveryV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedDiscoveryV1beta1 wrapper
type wrappedDiscoveryV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
}

func newDiscoveryV1beta1(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return &wrappedDiscoveryV1beta1{inner}
}
func (c *wrappedDiscoveryV1beta1) EndpointSlices(namespace string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	return newDiscoveryV1beta1EndpointSlices(c.inner.EndpointSlices(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedDiscoveryV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedEventsV1 wrapper
type wrappedEventsV1 struct {
	inner k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface
}

func newEventsV1(inner k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface) k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface {
	return &wrappedEventsV1{inner}
}
func (c *wrappedEventsV1) Events(namespace string) k8s_io_client_go_kubernetes_typed_events_v1.EventInterface {
	return newEventsV1Events(c.inner.Events(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedEventsV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedEventsV1beta1 wrapper
type wrappedEventsV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
}

func newEventsV1beta1(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return &wrappedEventsV1beta1{inner}
}
func (c *wrappedEventsV1beta1) Events(namespace string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	return newEventsV1beta1Events(c.inner.Events(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedEventsV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedExtensionsV1beta1 wrapper
type wrappedExtensionsV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
}

func newExtensionsV1beta1(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return &wrappedExtensionsV1beta1{inner}
}
func (c *wrappedExtensionsV1beta1) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	return newExtensionsV1beta1DaemonSets(c.inner.DaemonSets(namespace))
}
func (c *wrappedExtensionsV1beta1) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return newExtensionsV1beta1Deployments(c.inner.Deployments(namespace))
}
func (c *wrappedExtensionsV1beta1) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	return newExtensionsV1beta1Ingresses(c.inner.Ingresses(namespace))
}
func (c *wrappedExtensionsV1beta1) NetworkPolicies(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	return newExtensionsV1beta1NetworkPolicies(c.inner.NetworkPolicies(namespace))
}
func (c *wrappedExtensionsV1beta1) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	return newExtensionsV1beta1PodSecurityPolicies(c.inner.PodSecurityPolicies())
}
func (c *wrappedExtensionsV1beta1) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	return newExtensionsV1beta1ReplicaSets(c.inner.ReplicaSets(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedExtensionsV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedFlowcontrolV1alpha1 wrapper
type wrappedFlowcontrolV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface
}

func newFlowcontrolV1alpha1(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface {
	return &wrappedFlowcontrolV1alpha1{inner}
}
func (c *wrappedFlowcontrolV1alpha1) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	return newFlowcontrolV1alpha1FlowSchemas(c.inner.FlowSchemas())
}
func (c *wrappedFlowcontrolV1alpha1) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	return newFlowcontrolV1alpha1PriorityLevelConfigurations(c.inner.PriorityLevelConfigurations())
}

// RESTClient is NOT instrumented
func (c *wrappedFlowcontrolV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedFlowcontrolV1beta1 wrapper
type wrappedFlowcontrolV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
}

func newFlowcontrolV1beta1(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return &wrappedFlowcontrolV1beta1{inner}
}
func (c *wrappedFlowcontrolV1beta1) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	return newFlowcontrolV1beta1FlowSchemas(c.inner.FlowSchemas())
}
func (c *wrappedFlowcontrolV1beta1) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	return newFlowcontrolV1beta1PriorityLevelConfigurations(c.inner.PriorityLevelConfigurations())
}

// RESTClient is NOT instrumented
func (c *wrappedFlowcontrolV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedFlowcontrolV1beta2 wrapper
type wrappedFlowcontrolV1beta2 struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface
}

func newFlowcontrolV1beta2(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface {
	return &wrappedFlowcontrolV1beta2{inner}
}
func (c *wrappedFlowcontrolV1beta2) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface {
	return newFlowcontrolV1beta2FlowSchemas(c.inner.FlowSchemas())
}
func (c *wrappedFlowcontrolV1beta2) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface {
	return newFlowcontrolV1beta2PriorityLevelConfigurations(c.inner.PriorityLevelConfigurations())
}

// RESTClient is NOT instrumented
func (c *wrappedFlowcontrolV1beta2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedInternalV1alpha1 wrapper
type wrappedInternalV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
}

func newInternalV1alpha1(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return &wrappedInternalV1alpha1{inner}
}
func (c *wrappedInternalV1alpha1) StorageVersions() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	return newInternalV1alpha1StorageVersions(c.inner.StorageVersions())
}

// RESTClient is NOT instrumented
func (c *wrappedInternalV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNetworkingV1 wrapper
type wrappedNetworkingV1 struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface
}

func newNetworkingV1(inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface {
	return &wrappedNetworkingV1{inner}
}
func (c *wrappedNetworkingV1) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface {
	return newNetworkingV1IngressClasses(c.inner.IngressClasses())
}
func (c *wrappedNetworkingV1) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface {
	return newNetworkingV1Ingresses(c.inner.Ingresses(namespace))
}
func (c *wrappedNetworkingV1) NetworkPolicies(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface {
	return newNetworkingV1NetworkPolicies(c.inner.NetworkPolicies(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedNetworkingV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNetworkingV1alpha1 wrapper
type wrappedNetworkingV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
}

func newNetworkingV1alpha1(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return &wrappedNetworkingV1alpha1{inner}
}
func (c *wrappedNetworkingV1alpha1) ClusterCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	return newNetworkingV1alpha1ClusterCIDRs(c.inner.ClusterCIDRs())
}

// RESTClient is NOT instrumented
func (c *wrappedNetworkingV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNetworkingV1beta1 wrapper
type wrappedNetworkingV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
}

func newNetworkingV1beta1(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface) k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return &wrappedNetworkingV1beta1{inner}
}
func (c *wrappedNetworkingV1beta1) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return newNetworkingV1beta1IngressClasses(c.inner.IngressClasses())
}
func (c *wrappedNetworkingV1beta1) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return newNetworkingV1beta1Ingresses(c.inner.Ingresses(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedNetworkingV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNodeV1 wrapper
type wrappedNodeV1 struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
}

func newNodeV1(inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface) k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return &wrappedNodeV1{inner}
}
func (c *wrappedNodeV1) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	return newNodeV1RuntimeClasses(c.inner.RuntimeClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedNodeV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNodeV1alpha1 wrapper
type wrappedNodeV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface
}

func newNodeV1alpha1(inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface) k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface {
	return &wrappedNodeV1alpha1{inner}
}
func (c *wrappedNodeV1alpha1) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface {
	return newNodeV1alpha1RuntimeClasses(c.inner.RuntimeClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedNodeV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedNodeV1beta1 wrapper
type wrappedNodeV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface
}

func newNodeV1beta1(inner k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface) k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface {
	return &wrappedNodeV1beta1{inner}
}
func (c *wrappedNodeV1beta1) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface {
	return newNodeV1beta1RuntimeClasses(c.inner.RuntimeClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedNodeV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedPolicyV1 wrapper
type wrappedPolicyV1 struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
}

func newPolicyV1(inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface) k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return &wrappedPolicyV1{inner}
}
func (c *wrappedPolicyV1) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return newPolicyV1Evictions(c.inner.Evictions(namespace))
}
func (c *wrappedPolicyV1) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	return newPolicyV1PodDisruptionBudgets(c.inner.PodDisruptionBudgets(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedPolicyV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedPolicyV1beta1 wrapper
type wrappedPolicyV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
}

func newPolicyV1beta1(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return &wrappedPolicyV1beta1{inner}
}
func (c *wrappedPolicyV1beta1) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return newPolicyV1beta1Evictions(c.inner.Evictions(namespace))
}
func (c *wrappedPolicyV1beta1) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	return newPolicyV1beta1PodDisruptionBudgets(c.inner.PodDisruptionBudgets(namespace))
}
func (c *wrappedPolicyV1beta1) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	return newPolicyV1beta1PodSecurityPolicies(c.inner.PodSecurityPolicies())
}

// RESTClient is NOT instrumented
func (c *wrappedPolicyV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedRbacV1 wrapper
type wrappedRbacV1 struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
}

func newRbacV1(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface) k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface {
	return &wrappedRbacV1{inner}
}
func (c *wrappedRbacV1) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	return newRbacV1ClusterRoleBindings(c.inner.ClusterRoleBindings())
}
func (c *wrappedRbacV1) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	return newRbacV1ClusterRoles(c.inner.ClusterRoles())
}
func (c *wrappedRbacV1) RoleBindings(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	return newRbacV1RoleBindings(c.inner.RoleBindings(namespace))
}
func (c *wrappedRbacV1) Roles(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	return newRbacV1Roles(c.inner.Roles(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedRbacV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedRbacV1alpha1 wrapper
type wrappedRbacV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface
}

func newRbacV1alpha1(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface {
	return &wrappedRbacV1alpha1{inner}
}
func (c *wrappedRbacV1alpha1) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface {
	return newRbacV1alpha1ClusterRoleBindings(c.inner.ClusterRoleBindings())
}
func (c *wrappedRbacV1alpha1) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface {
	return newRbacV1alpha1ClusterRoles(c.inner.ClusterRoles())
}
func (c *wrappedRbacV1alpha1) RoleBindings(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface {
	return newRbacV1alpha1RoleBindings(c.inner.RoleBindings(namespace))
}
func (c *wrappedRbacV1alpha1) Roles(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface {
	return newRbacV1alpha1Roles(c.inner.Roles(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedRbacV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedRbacV1beta1 wrapper
type wrappedRbacV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface
}

func newRbacV1beta1(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface {
	return &wrappedRbacV1beta1{inner}
}
func (c *wrappedRbacV1beta1) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface {
	return newRbacV1beta1ClusterRoleBindings(c.inner.ClusterRoleBindings())
}
func (c *wrappedRbacV1beta1) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface {
	return newRbacV1beta1ClusterRoles(c.inner.ClusterRoles())
}
func (c *wrappedRbacV1beta1) RoleBindings(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface {
	return newRbacV1beta1RoleBindings(c.inner.RoleBindings(namespace))
}
func (c *wrappedRbacV1beta1) Roles(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface {
	return newRbacV1beta1Roles(c.inner.Roles(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedRbacV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedSchedulingV1 wrapper
type wrappedSchedulingV1 struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface
}

func newSchedulingV1(inner k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface) k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface {
	return &wrappedSchedulingV1{inner}
}
func (c *wrappedSchedulingV1) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface {
	return newSchedulingV1PriorityClasses(c.inner.PriorityClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedSchedulingV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedSchedulingV1alpha1 wrapper
type wrappedSchedulingV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
}

func newSchedulingV1alpha1(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface {
	return &wrappedSchedulingV1alpha1{inner}
}
func (c *wrappedSchedulingV1alpha1) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	return newSchedulingV1alpha1PriorityClasses(c.inner.PriorityClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedSchedulingV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedSchedulingV1beta1 wrapper
type wrappedSchedulingV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
}

func newSchedulingV1beta1(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return &wrappedSchedulingV1beta1{inner}
}
func (c *wrappedSchedulingV1beta1) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	return newSchedulingV1beta1PriorityClasses(c.inner.PriorityClasses())
}

// RESTClient is NOT instrumented
func (c *wrappedSchedulingV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedStorageV1 wrapper
type wrappedStorageV1 struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface
}

func newStorageV1(inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface) k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface {
	return &wrappedStorageV1{inner}
}
func (c *wrappedStorageV1) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface {
	return newStorageV1CSIDrivers(c.inner.CSIDrivers())
}
func (c *wrappedStorageV1) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface {
	return newStorageV1CSINodes(c.inner.CSINodes())
}
func (c *wrappedStorageV1) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface {
	return newStorageV1CSIStorageCapacities(c.inner.CSIStorageCapacities(namespace))
}
func (c *wrappedStorageV1) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface {
	return newStorageV1StorageClasses(c.inner.StorageClasses())
}
func (c *wrappedStorageV1) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface {
	return newStorageV1VolumeAttachments(c.inner.VolumeAttachments())
}

// RESTClient is NOT instrumented
func (c *wrappedStorageV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedStorageV1alpha1 wrapper
type wrappedStorageV1alpha1 struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
}

func newStorageV1alpha1(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return &wrappedStorageV1alpha1{inner}
}
func (c *wrappedStorageV1alpha1) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	return newStorageV1alpha1CSIStorageCapacities(c.inner.CSIStorageCapacities(namespace))
}
func (c *wrappedStorageV1alpha1) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	return newStorageV1alpha1VolumeAttachments(c.inner.VolumeAttachments())
}

// RESTClient is NOT instrumented
func (c *wrappedStorageV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedStorageV1beta1 wrapper
type wrappedStorageV1beta1 struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
}

func newStorageV1beta1(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return &wrappedStorageV1beta1{inner}
}
func (c *wrappedStorageV1beta1) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	return newStorageV1beta1CSIDrivers(c.inner.CSIDrivers())
}
func (c *wrappedStorageV1beta1) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	return newStorageV1beta1CSINodes(c.inner.CSINodes())
}
func (c *wrappedStorageV1beta1) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	return newStorageV1beta1CSIStorageCapacities(c.inner.CSIStorageCapacities(namespace))
}
func (c *wrappedStorageV1beta1) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	return newStorageV1beta1StorageClasses(c.inner.StorageClasses())
}
func (c *wrappedStorageV1beta1) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	return newStorageV1beta1VolumeAttachments(c.inner.VolumeAttachments())
}

// RESTClient is NOT instrumented
func (c *wrappedStorageV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedAdmissionregistrationV1MutatingWebhookConfigurations wrapper
type wrappedAdmissionregistrationV1MutatingWebhookConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface
}

func newAdmissionregistrationV1MutatingWebhookConfigurations(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1MutatingWebhookConfigurations{inner}
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1.MutatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/MutatingWebhookConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAdmissionregistrationV1ValidatingWebhookConfigurations wrapper
type wrappedAdmissionregistrationV1ValidatingWebhookConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface
}

func newAdmissionregistrationV1ValidatingWebhookConfigurations(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1ValidatingWebhookConfigurations{inner}
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1.ValidatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1/ValidatingWebhookConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations wrapper
type wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface
}

func newAdmissionregistrationV1beta1MutatingWebhookConfigurations(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations{inner}
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1beta1.MutatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/MutatingWebhookConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations wrapper
type wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface
}

func newAdmissionregistrationV1beta1ValidatingWebhookConfigurations(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations{inner}
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1beta1.ValidatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AdmissionregistrationV1beta1/ValidatingWebhookConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1ControllerRevisions wrapper
type wrappedAppsV1ControllerRevisions struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface
}

func newAppsV1ControllerRevisions(inner k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	return &wrappedAppsV1ControllerRevisions{inner}
}
func (c *wrappedAppsV1ControllerRevisions) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ControllerRevisionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ControllerRevisions", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1DaemonSets wrapper
type wrappedAppsV1DaemonSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface
}

func newAppsV1DaemonSets(inner k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	return &wrappedAppsV1DaemonSets{inner}
}
func (c *wrappedAppsV1DaemonSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/DaemonSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1Deployments wrapper
type wrappedAppsV1Deployments struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface
}

func newAppsV1Deployments(inner k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	return &wrappedAppsV1Deployments{inner}
}
func (c *wrappedAppsV1Deployments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/Deployments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1ReplicaSets wrapper
type wrappedAppsV1ReplicaSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface
}

func newAppsV1ReplicaSets(inner k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return &wrappedAppsV1ReplicaSets{inner}
}
func (c *wrappedAppsV1ReplicaSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/ReplicaSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1StatefulSets wrapper
type wrappedAppsV1StatefulSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface
}

func newAppsV1StatefulSets(inner k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	return &wrappedAppsV1StatefulSets{inner}
}
func (c *wrappedAppsV1StatefulSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1/StatefulSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta1ControllerRevisions wrapper
type wrappedAppsV1beta1ControllerRevisions struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface
}

func newAppsV1beta1ControllerRevisions(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface {
	return &wrappedAppsV1beta1ControllerRevisions{inner}
}
func (c *wrappedAppsV1beta1ControllerRevisions) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.ControllerRevisionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/ControllerRevisions", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta1Deployments wrapper
type wrappedAppsV1beta1Deployments struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface
}

func newAppsV1beta1Deployments(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface {
	return &wrappedAppsV1beta1Deployments{inner}
}
func (c *wrappedAppsV1beta1Deployments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/Deployments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta1StatefulSets wrapper
type wrappedAppsV1beta1StatefulSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface
}

func newAppsV1beta1StatefulSets(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface {
	return &wrappedAppsV1beta1StatefulSets{inner}
}
func (c *wrappedAppsV1beta1StatefulSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta1/StatefulSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta2ControllerRevisions wrapper
type wrappedAppsV1beta2ControllerRevisions struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface
}

func newAppsV1beta2ControllerRevisions(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface {
	return &wrappedAppsV1beta2ControllerRevisions{inner}
}
func (c *wrappedAppsV1beta2ControllerRevisions) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ControllerRevisionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ControllerRevisions", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta2DaemonSets wrapper
type wrappedAppsV1beta2DaemonSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface
}

func newAppsV1beta2DaemonSets(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface {
	return &wrappedAppsV1beta2DaemonSets{inner}
}
func (c *wrappedAppsV1beta2DaemonSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/DaemonSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta2Deployments wrapper
type wrappedAppsV1beta2Deployments struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface
}

func newAppsV1beta2Deployments(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface {
	return &wrappedAppsV1beta2Deployments{inner}
}
func (c *wrappedAppsV1beta2Deployments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/Deployments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta2ReplicaSets wrapper
type wrappedAppsV1beta2ReplicaSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface
}

func newAppsV1beta2ReplicaSets(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface {
	return &wrappedAppsV1beta2ReplicaSets{inner}
}
func (c *wrappedAppsV1beta2ReplicaSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/ReplicaSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAppsV1beta2StatefulSets wrapper
type wrappedAppsV1beta2StatefulSets struct {
	inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface
}

func newAppsV1beta2StatefulSets(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface) k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface {
	return &wrappedAppsV1beta2StatefulSets{inner}
}
func (c *wrappedAppsV1beta2StatefulSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_apps_v1beta2.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AppsV1beta2/StatefulSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAuthenticationV1TokenReviews wrapper
type wrappedAuthenticationV1TokenReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface
}

func newAuthenticationV1TokenReviews(inner k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface) k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return &wrappedAuthenticationV1TokenReviews{inner}
}
func (c *wrappedAuthenticationV1TokenReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1.TokenReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthenticationV1/TokenReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthenticationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "TokenReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "TokenReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthenticationV1beta1TokenReviews wrapper
type wrappedAuthenticationV1beta1TokenReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface
}

func newAuthenticationV1beta1TokenReviews(inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface) k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface {
	return &wrappedAuthenticationV1beta1TokenReviews{inner}
}
func (c *wrappedAuthenticationV1beta1TokenReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1beta1.TokenReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1beta1.TokenReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthenticationV1beta1/TokenReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthenticationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "TokenReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "TokenReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1LocalSubjectAccessReviews wrapper
type wrappedAuthorizationV1LocalSubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface
}

func newAuthorizationV1LocalSubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1LocalSubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1LocalSubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.LocalSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.LocalSubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1/LocalSubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LocalSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "LocalSubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1SelfSubjectAccessReviews wrapper
type wrappedAuthorizationV1SelfSubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface
}

func newAuthorizationV1SelfSubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1SelfSubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1SelfSubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SelfSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.SelfSubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1/SelfSubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1SelfSubjectRulesReviews wrapper
type wrappedAuthorizationV1SelfSubjectRulesReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface
}

func newAuthorizationV1SelfSubjectRulesReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	return &wrappedAuthorizationV1SelfSubjectRulesReviews{inner}
}
func (c *wrappedAuthorizationV1SelfSubjectRulesReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.SelfSubjectRulesReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1/SelfSubjectRulesReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectRulesReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectRulesReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1SubjectAccessReviews wrapper
type wrappedAuthorizationV1SubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface
}

func newAuthorizationV1SubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	return &wrappedAuthorizationV1SubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1SubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.SubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1/SubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1beta1LocalSubjectAccessReviews wrapper
type wrappedAuthorizationV1beta1LocalSubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface
}

func newAuthorizationV1beta1LocalSubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1LocalSubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1beta1LocalSubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.LocalSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.LocalSubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1beta1/LocalSubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LocalSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "LocalSubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1beta1SelfSubjectAccessReviews wrapper
type wrappedAuthorizationV1beta1SelfSubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface
}

func newAuthorizationV1beta1SelfSubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1SelfSubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1beta1SelfSubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1beta1/SelfSubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1beta1SelfSubjectRulesReviews wrapper
type wrappedAuthorizationV1beta1SelfSubjectRulesReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface
}

func newAuthorizationV1beta1SelfSubjectRulesReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return &wrappedAuthorizationV1beta1SelfSubjectRulesReviews{inner}
}
func (c *wrappedAuthorizationV1beta1SelfSubjectRulesReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1beta1/SelfSubjectRulesReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectRulesReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectRulesReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAuthorizationV1beta1SubjectAccessReviews wrapper
type wrappedAuthorizationV1beta1SubjectAccessReviews struct {
	inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface
}

func newAuthorizationV1beta1SubjectAccessReviews(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1SubjectAccessReviews{inner}
}
func (c *wrappedAuthorizationV1beta1SubjectAccessReviews) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SubjectAccessReview, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AuthorizationV1beta1/SubjectAccessReviews", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SubjectAccessReview"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}

// wrappedAutoscalingV1HorizontalPodAutoscalers wrapper
type wrappedAutoscalingV1HorizontalPodAutoscalers struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface
}

func newAutoscalingV1HorizontalPodAutoscalers(inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface) k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV1HorizontalPodAutoscalers{inner}
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV1/HorizontalPodAutoscalers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAutoscalingV2HorizontalPodAutoscalers wrapper
type wrappedAutoscalingV2HorizontalPodAutoscalers struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface
}

func newAutoscalingV2HorizontalPodAutoscalers(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2HorizontalPodAutoscalers{inner}
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2/HorizontalPodAutoscalers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAutoscalingV2beta1HorizontalPodAutoscalers wrapper
type wrappedAutoscalingV2beta1HorizontalPodAutoscalers struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface
}

func newAutoscalingV2beta1HorizontalPodAutoscalers(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2beta1HorizontalPodAutoscalers{inner}
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta1/HorizontalPodAutoscalers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedAutoscalingV2beta2HorizontalPodAutoscalers wrapper
type wrappedAutoscalingV2beta2HorizontalPodAutoscalers struct {
	inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface
}

func newAutoscalingV2beta2HorizontalPodAutoscalers(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2beta2HorizontalPodAutoscalers{inner}
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "AutoscalingV2beta2/HorizontalPodAutoscalers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedBatchV1CronJobs wrapper
type wrappedBatchV1CronJobs struct {
	inner k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface
}

func newBatchV1CronJobs(inner k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	return &wrappedBatchV1CronJobs{inner}
}
func (c *wrappedBatchV1CronJobs) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.CronJobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1CronJobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1CronJobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/CronJobs", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedBatchV1Jobs wrapper
type wrappedBatchV1Jobs struct {
	inner k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface
}

func newBatchV1Jobs(inner k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	return &wrappedBatchV1Jobs{inner}
}
func (c *wrappedBatchV1Jobs) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.JobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.JobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.JobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1Jobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1Jobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1/Jobs", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedBatchV1beta1CronJobs wrapper
type wrappedBatchV1beta1CronJobs struct {
	inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface
}

func newBatchV1beta1CronJobs(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	return &wrappedBatchV1beta1CronJobs{inner}
}
func (c *wrappedBatchV1beta1CronJobs) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1beta1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1beta1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1beta1.CronJobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1beta1CronJobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1beta1CronJobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "BatchV1beta1/CronJobs", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCertificatesV1CertificateSigningRequests wrapper
type wrappedCertificatesV1CertificateSigningRequests struct {
	inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface
}

func newCertificatesV1CertificateSigningRequests(inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface) k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface {
	return &wrappedCertificatesV1CertificateSigningRequests{inner}
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) UpdateApproval(arg0 context.Context, arg1 string, arg2 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "UpdateApproval", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateApproval(arg0, arg1, arg2, arg3)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1/CertificateSigningRequests", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCertificatesV1beta1CertificateSigningRequests wrapper
type wrappedCertificatesV1beta1CertificateSigningRequests struct {
	inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface
}

func newCertificatesV1beta1CertificateSigningRequests(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	return &wrappedCertificatesV1beta1CertificateSigningRequests{inner}
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1beta1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1beta1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) UpdateApproval(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "UpdateApproval", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateApproval(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CertificatesV1beta1/CertificateSigningRequests", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoordinationV1Leases wrapper
type wrappedCoordinationV1Leases struct {
	inner k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface
}

func newCoordinationV1Leases(inner k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	return &wrappedCoordinationV1Leases{inner}
}
func (c *wrappedCoordinationV1Leases) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_coordination_v1.LeaseApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_coordination_v1.LeaseList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1Leases) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1Leases) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1/Leases", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoordinationV1beta1Leases wrapper
type wrappedCoordinationV1beta1Leases struct {
	inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface
}

func newCoordinationV1beta1Leases(inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface {
	return &wrappedCoordinationV1beta1Leases{inner}
}
func (c *wrappedCoordinationV1beta1Leases) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_coordination_v1beta1.LeaseApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_coordination_v1beta1.LeaseList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1beta1Leases) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1beta1Leases) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoordinationV1beta1/Leases", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1ComponentStatuses wrapper
type wrappedCoreV1ComponentStatuses struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface
}

func newCoreV1ComponentStatuses(inner k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface) k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	return &wrappedCoreV1ComponentStatuses{inner}
}
func (c *wrappedCoreV1ComponentStatuses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ComponentStatusApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ComponentStatusList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ComponentStatuses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ComponentStatuses) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ComponentStatuses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1ConfigMaps wrapper
type wrappedCoreV1ConfigMaps struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface
}

func newCoreV1ConfigMaps(inner k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	return &wrappedCoreV1ConfigMaps{inner}
}
func (c *wrappedCoreV1ConfigMaps) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ConfigMapApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ConfigMapList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ConfigMaps) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ConfigMaps) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ConfigMaps", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Endpoints wrapper
type wrappedCoreV1Endpoints struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface
}

func newCoreV1Endpoints(inner k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	return &wrappedCoreV1Endpoints{inner}
}
func (c *wrappedCoreV1Endpoints) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EndpointsApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EndpointsList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Endpoints) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Endpoints) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Endpoints", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Events wrapper
type wrappedCoreV1Events struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface
}

func newCoreV1Events(inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return &wrappedCoreV1Events{inner}
}
func (c *wrappedCoreV1Events) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	return c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Events) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedCoreV1Events) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	return c.inner.Search(arg0, arg1)
}
func (c *wrappedCoreV1Events) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Events", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1LimitRanges wrapper
type wrappedCoreV1LimitRanges struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface
}

func newCoreV1LimitRanges(inner k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	return &wrappedCoreV1LimitRanges{inner}
}
func (c *wrappedCoreV1LimitRanges) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.LimitRangeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.LimitRangeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1LimitRanges) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1LimitRanges) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/LimitRanges", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Namespaces wrapper
type wrappedCoreV1Namespaces struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface
}

func newCoreV1Namespaces(inner k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface) k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	return &wrappedCoreV1Namespaces{inner}
}
func (c *wrappedCoreV1Namespaces) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NamespaceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NamespaceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Finalize(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Finalize", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Finalize(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.NamespaceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Namespaces) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Namespaces) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Namespaces", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Nodes wrapper
type wrappedCoreV1Nodes struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface
}

func newCoreV1Nodes(inner k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface) k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	return &wrappedCoreV1Nodes{inner}
}
func (c *wrappedCoreV1Nodes) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.NodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Nodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Nodes) PatchStatus(arg0 context.Context, arg1 string, arg2 []uint8) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "PatchStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.PatchStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Nodes", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1PersistentVolumeClaims wrapper
type wrappedCoreV1PersistentVolumeClaims struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface
}

func newCoreV1PersistentVolumeClaims(inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	return &wrappedCoreV1PersistentVolumeClaims{inner}
}
func (c *wrappedCoreV1PersistentVolumeClaims) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeClaimApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeClaimApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeClaimList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumeClaims", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1PersistentVolumes wrapper
type wrappedCoreV1PersistentVolumes struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface
}

func newCoreV1PersistentVolumes(inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	return &wrappedCoreV1PersistentVolumes{inner}
}
func (c *wrappedCoreV1PersistentVolumes) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PersistentVolumes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PersistentVolumes) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PersistentVolumes", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1PodTemplates wrapper
type wrappedCoreV1PodTemplates struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface
}

func newCoreV1PodTemplates(inner k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	return &wrappedCoreV1PodTemplates{inner}
}
func (c *wrappedCoreV1PodTemplates) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodTemplateApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodTemplateList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PodTemplates) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PodTemplates) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/PodTemplates", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Pods wrapper
type wrappedCoreV1Pods struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.PodInterface
}

func newCoreV1Pods(inner k8s_io_client_go_kubernetes_typed_core_v1.PodInterface) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	return &wrappedCoreV1Pods{inner}
}
func (c *wrappedCoreV1Pods) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Bind(arg0 context.Context, arg1 *k8s_io_api_core_v1.Binding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Bind", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Bind(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Evict", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Evict(arg0, arg1)
}
func (c *wrappedCoreV1Pods) EvictV1(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "EvictV1", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.EvictV1(arg0, arg1)
}
func (c *wrappedCoreV1Pods) EvictV1beta1(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "EvictV1beta1", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.EvictV1beta1(arg0, arg1)
}
func (c *wrappedCoreV1Pods) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) GetLogs(arg0 string, arg1 *k8s_io_api_core_v1.PodLogOptions) *k8s_io_client_go_rest.Request {
	return c.inner.GetLogs(arg0, arg1)
}
func (c *wrappedCoreV1Pods) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Pods) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Pods) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1Pods) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) UpdateEphemeralContainers(arg0 context.Context, arg1 string, arg2 *k8s_io_api_core_v1.Pod, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "UpdateEphemeralContainers", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateEphemeralContainers(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1Pods) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Pods", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1ReplicationControllers wrapper
type wrappedCoreV1ReplicationControllers struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface
}

func newCoreV1ReplicationControllers(inner k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	return &wrappedCoreV1ReplicationControllers{inner}
}
func (c *wrappedCoreV1ReplicationControllers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ReplicationControllerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ReplicationControllerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ReplicationControllerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ReplicationControllers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ReplicationControllers) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1ReplicationControllers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ReplicationControllers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1ResourceQuotas wrapper
type wrappedCoreV1ResourceQuotas struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface
}

func newCoreV1ResourceQuotas(inner k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	return &wrappedCoreV1ResourceQuotas{inner}
}
func (c *wrappedCoreV1ResourceQuotas) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ResourceQuotaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ResourceQuotaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ResourceQuotaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ResourceQuotas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ResourceQuotas) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ResourceQuotas", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Secrets wrapper
type wrappedCoreV1Secrets struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface
}

func newCoreV1Secrets(inner k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	return &wrappedCoreV1Secrets{inner}
}
func (c *wrappedCoreV1Secrets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.SecretApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.SecretList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Secrets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Secrets) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Secrets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1ServiceAccounts wrapper
type wrappedCoreV1ServiceAccounts struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface
}

func newCoreV1ServiceAccounts(inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	return &wrappedCoreV1ServiceAccounts{inner}
}
func (c *wrappedCoreV1ServiceAccounts) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceAccountApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) CreateToken(arg0 context.Context, arg1 string, arg2 *k8s_io_api_authentication_v1.TokenRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "CreateToken", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.CreateToken(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1ServiceAccounts) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceAccountList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ServiceAccounts) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ServiceAccounts) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/ServiceAccounts", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedCoreV1Services wrapper
type wrappedCoreV1Services struct {
	inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface
}

func newCoreV1Services(inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	return &wrappedCoreV1Services{inner}
}
func (c *wrappedCoreV1Services) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Services) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Services) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1Services) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "CoreV1/Services", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedDiscoveryV1EndpointSlices wrapper
type wrappedDiscoveryV1EndpointSlices struct {
	inner k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface
}

func newDiscoveryV1EndpointSlices(inner k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface) k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface {
	return &wrappedDiscoveryV1EndpointSlices{inner}
}
func (c *wrappedDiscoveryV1EndpointSlices) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_discovery_v1.EndpointSliceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_discovery_v1.EndpointSliceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1EndpointSlices) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1EndpointSlices) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1/EndpointSlices", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedDiscoveryV1beta1EndpointSlices wrapper
type wrappedDiscoveryV1beta1EndpointSlices struct {
	inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface
}

func newDiscoveryV1beta1EndpointSlices(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	return &wrappedDiscoveryV1beta1EndpointSlices{inner}
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_discovery_v1beta1.EndpointSliceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_discovery_v1beta1.EndpointSliceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "DiscoveryV1beta1/EndpointSlices", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedEventsV1Events wrapper
type wrappedEventsV1Events struct {
	inner k8s_io_client_go_kubernetes_typed_events_v1.EventInterface
}

func newEventsV1Events(inner k8s_io_client_go_kubernetes_typed_events_v1.EventInterface) k8s_io_client_go_kubernetes_typed_events_v1.EventInterface {
	return &wrappedEventsV1Events{inner}
}
func (c *wrappedEventsV1Events) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_events_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_events_v1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1Events) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1/Events", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedEventsV1beta1Events wrapper
type wrappedEventsV1beta1Events struct {
	inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface
}

func newEventsV1beta1Events(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	return &wrappedEventsV1beta1Events{inner}
}
func (c *wrappedEventsV1beta1Events) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_events_v1beta1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) CreateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_events_v1beta1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1beta1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1beta1Events) PatchWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event, arg1 []uint8) (*k8s_io_api_events_v1beta1.Event, error) {
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedEventsV1beta1Events) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) UpdateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "EventsV1beta1/Events", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1DaemonSets wrapper
type wrappedExtensionsV1beta1DaemonSets struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface
}

func newExtensionsV1beta1DaemonSets(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	return &wrappedExtensionsV1beta1DaemonSets{inner}
}
func (c *wrappedExtensionsV1beta1DaemonSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/DaemonSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1Deployments wrapper
type wrappedExtensionsV1beta1Deployments struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface
}

func newExtensionsV1beta1Deployments(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return &wrappedExtensionsV1beta1Deployments{inner}
}
func (c *wrappedExtensionsV1beta1Deployments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1Deployments) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Rollback", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Rollback(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Deployments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1Ingresses wrapper
type wrappedExtensionsV1beta1Ingresses struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface
}

func newExtensionsV1beta1Ingresses(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	return &wrappedExtensionsV1beta1Ingresses{inner}
}
func (c *wrappedExtensionsV1beta1Ingresses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/Ingresses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1NetworkPolicies wrapper
type wrappedExtensionsV1beta1NetworkPolicies struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface
}

func newExtensionsV1beta1NetworkPolicies(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	return &wrappedExtensionsV1beta1NetworkPolicies{inner}
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/NetworkPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1PodSecurityPolicies wrapper
type wrappedExtensionsV1beta1PodSecurityPolicies struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface
}

func newExtensionsV1beta1PodSecurityPolicies(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	return &wrappedExtensionsV1beta1PodSecurityPolicies{inner}
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.PodSecurityPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/PodSecurityPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedExtensionsV1beta1ReplicaSets wrapper
type wrappedExtensionsV1beta1ReplicaSets struct {
	inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface
}

func newExtensionsV1beta1ReplicaSets(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	return &wrappedExtensionsV1beta1ReplicaSets{inner}
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "ApplyScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "GetScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "UpdateScale", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "ExtensionsV1beta1/ReplicaSets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1alpha1FlowSchemas wrapper
type wrappedFlowcontrolV1alpha1FlowSchemas struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface
}

func newFlowcontrolV1alpha1FlowSchemas(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	return &wrappedFlowcontrolV1alpha1FlowSchemas{inner}
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/FlowSchemas", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1alpha1PriorityLevelConfigurations wrapper
type wrappedFlowcontrolV1alpha1PriorityLevelConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface
}

func newFlowcontrolV1alpha1PriorityLevelConfigurations(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1alpha1PriorityLevelConfigurations{inner}
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1alpha1/PriorityLevelConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1beta1FlowSchemas wrapper
type wrappedFlowcontrolV1beta1FlowSchemas struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface
}

func newFlowcontrolV1beta1FlowSchemas(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	return &wrappedFlowcontrolV1beta1FlowSchemas{inner}
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/FlowSchemas", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1beta1PriorityLevelConfigurations wrapper
type wrappedFlowcontrolV1beta1PriorityLevelConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface
}

func newFlowcontrolV1beta1PriorityLevelConfigurations(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1beta1PriorityLevelConfigurations{inner}
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta1/PriorityLevelConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1beta2FlowSchemas wrapper
type wrappedFlowcontrolV1beta2FlowSchemas struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface
}

func newFlowcontrolV1beta2FlowSchemas(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface {
	return &wrappedFlowcontrolV1beta2FlowSchemas{inner}
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/FlowSchemas", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedFlowcontrolV1beta2PriorityLevelConfigurations wrapper
type wrappedFlowcontrolV1beta2PriorityLevelConfigurations struct {
	inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface
}

func newFlowcontrolV1beta2PriorityLevelConfigurations(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1beta2PriorityLevelConfigurations{inner}
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "FlowcontrolV1beta2/PriorityLevelConfigurations", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedInternalV1alpha1StorageVersions wrapper
type wrappedInternalV1alpha1StorageVersions struct {
	inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface
}

func newInternalV1alpha1StorageVersions(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	return &wrappedInternalV1alpha1StorageVersions{inner}
}
func (c *wrappedInternalV1alpha1StorageVersions) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1.StorageVersionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1.StorageVersionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Create(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedInternalV1alpha1StorageVersions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedInternalV1alpha1StorageVersions) Update(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "InternalV1alpha1/StorageVersions", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1IngressClasses wrapper
type wrappedNetworkingV1IngressClasses struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface
}

func newNetworkingV1IngressClasses(inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface) k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface {
	return &wrappedNetworkingV1IngressClasses{inner}
}
func (c *wrappedNetworkingV1IngressClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.IngressClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1IngressClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1IngressClasses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/IngressClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1Ingresses wrapper
type wrappedNetworkingV1Ingresses struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface
}

func newNetworkingV1Ingresses(inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface) k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface {
	return &wrappedNetworkingV1Ingresses{inner}
}
func (c *wrappedNetworkingV1Ingresses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/Ingresses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1NetworkPolicies wrapper
type wrappedNetworkingV1NetworkPolicies struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface
}

func newNetworkingV1NetworkPolicies(inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface {
	return &wrappedNetworkingV1NetworkPolicies{inner}
}
func (c *wrappedNetworkingV1NetworkPolicies) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.NetworkPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1NetworkPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1NetworkPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1/NetworkPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1alpha1ClusterCIDRs wrapper
type wrappedNetworkingV1alpha1ClusterCIDRs struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface
}

func newNetworkingV1alpha1ClusterCIDRs(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	return &wrappedNetworkingV1alpha1ClusterCIDRs{inner}
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1alpha1.ClusterCIDRApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDRList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1alpha1/ClusterCIDRs", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1beta1IngressClasses wrapper
type wrappedNetworkingV1beta1IngressClasses struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface
}

func newNetworkingV1beta1IngressClasses(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return &wrappedNetworkingV1beta1IngressClasses{inner}
}
func (c *wrappedNetworkingV1beta1IngressClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/IngressClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNetworkingV1beta1Ingresses wrapper
type wrappedNetworkingV1beta1Ingresses struct {
	inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface
}

func newNetworkingV1beta1Ingresses(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return &wrappedNetworkingV1beta1Ingresses{inner}
}
func (c *wrappedNetworkingV1beta1Ingresses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NetworkingV1beta1/Ingresses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNodeV1RuntimeClasses wrapper
type wrappedNodeV1RuntimeClasses struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface
}

func newNodeV1RuntimeClasses(inner k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface) k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	return &wrappedNodeV1RuntimeClasses{inner}
}
func (c *wrappedNodeV1RuntimeClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1.RuntimeClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1/RuntimeClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNodeV1alpha1RuntimeClasses wrapper
type wrappedNodeV1alpha1RuntimeClasses struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface
}

func newNodeV1alpha1RuntimeClasses(inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface) k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface {
	return &wrappedNodeV1alpha1RuntimeClasses{inner}
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1alpha1.RuntimeClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1alpha1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1alpha1/RuntimeClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedNodeV1beta1RuntimeClasses wrapper
type wrappedNodeV1beta1RuntimeClasses struct {
	inner k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface
}

func newNodeV1beta1RuntimeClasses(inner k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface) k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface {
	return &wrappedNodeV1beta1RuntimeClasses{inner}
}
func (c *wrappedNodeV1beta1RuntimeClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1beta1.RuntimeClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1beta1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "NodeV1beta1/RuntimeClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedPolicyV1Evictions wrapper
type wrappedPolicyV1Evictions struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface
}

func newPolicyV1Evictions(inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return &wrappedPolicyV1Evictions{inner}
}
func (c *wrappedPolicyV1Evictions) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/Evictions", "Evict", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Evictions"),
		go_opentelemetry_io_otel_attribute.String("kind", "Eviction"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Evict(arg0, arg1)
}

// wrappedPolicyV1PodDisruptionBudgets wrapper
type wrappedPolicyV1PodDisruptionBudgets struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface
}

func newPolicyV1PodDisruptionBudgets(inner k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	return &wrappedPolicyV1PodDisruptionBudgets{inner}
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1.PodDisruptionBudgetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1/PodDisruptionBudgets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedPolicyV1beta1Evictions wrapper
type wrappedPolicyV1beta1Evictions struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface
}

func newPolicyV1beta1Evictions(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return &wrappedPolicyV1beta1Evictions{inner}
}
func (c *wrappedPolicyV1beta1Evictions) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/Evictions", "Evict", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Evictions"),
		go_opentelemetry_io_otel_attribute.String("kind", "Eviction"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Evict(arg0, arg1)
}

// wrappedPolicyV1beta1PodDisruptionBudgets wrapper
type wrappedPolicyV1beta1PodDisruptionBudgets struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface
}

func newPolicyV1beta1PodDisruptionBudgets(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	return &wrappedPolicyV1beta1PodDisruptionBudgets{inner}
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudgetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodDisruptionBudgets", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedPolicyV1beta1PodSecurityPolicies wrapper
type wrappedPolicyV1beta1PodSecurityPolicies struct {
	inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface
}

func newPolicyV1beta1PodSecurityPolicies(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	return &wrappedPolicyV1beta1PodSecurityPolicies{inner}
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodSecurityPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "PolicyV1beta1/PodSecurityPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1ClusterRoleBindings wrapper
type wrappedRbacV1ClusterRoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface
}

func newRbacV1ClusterRoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	return &wrappedRbacV1ClusterRoleBindings{inner}
}
func (c *wrappedRbacV1ClusterRoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1ClusterRoles wrapper
type wrappedRbacV1ClusterRoles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface
}

func newRbacV1ClusterRoles(inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	return &wrappedRbacV1ClusterRoles{inner}
}
func (c *wrappedRbacV1ClusterRoles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/ClusterRoles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1RoleBindings wrapper
type wrappedRbacV1RoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface
}

func newRbacV1RoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	return &wrappedRbacV1RoleBindings{inner}
}
func (c *wrappedRbacV1RoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.RoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/RoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1Roles wrapper
type wrappedRbacV1Roles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface
}

func newRbacV1Roles(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	return &wrappedRbacV1Roles{inner}
}
func (c *wrappedRbacV1Roles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.RoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1/Roles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1alpha1ClusterRoleBindings wrapper
type wrappedRbacV1alpha1ClusterRoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface
}

func newRbacV1alpha1ClusterRoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface {
	return &wrappedRbacV1alpha1ClusterRoleBindings{inner}
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1alpha1ClusterRoles wrapper
type wrappedRbacV1alpha1ClusterRoles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface
}

func newRbacV1alpha1ClusterRoles(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface {
	return &wrappedRbacV1alpha1ClusterRoles{inner}
}
func (c *wrappedRbacV1alpha1ClusterRoles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/ClusterRoles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1alpha1RoleBindings wrapper
type wrappedRbacV1alpha1RoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface
}

func newRbacV1alpha1RoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface {
	return &wrappedRbacV1alpha1RoleBindings{inner}
}
func (c *wrappedRbacV1alpha1RoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.RoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/RoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1alpha1Roles wrapper
type wrappedRbacV1alpha1Roles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface
}

func newRbacV1alpha1Roles(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface {
	return &wrappedRbacV1alpha1Roles{inner}
}
func (c *wrappedRbacV1alpha1Roles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.RoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1alpha1/Roles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1beta1ClusterRoleBindings wrapper
type wrappedRbacV1beta1ClusterRoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface
}

func newRbacV1beta1ClusterRoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface {
	return &wrappedRbacV1beta1ClusterRoleBindings{inner}
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1beta1ClusterRoles wrapper
type wrappedRbacV1beta1ClusterRoles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface
}

func newRbacV1beta1ClusterRoles(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface {
	return &wrappedRbacV1beta1ClusterRoles{inner}
}
func (c *wrappedRbacV1beta1ClusterRoles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/ClusterRoles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1beta1RoleBindings wrapper
type wrappedRbacV1beta1RoleBindings struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface
}

func newRbacV1beta1RoleBindings(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface {
	return &wrappedRbacV1beta1RoleBindings{inner}
}
func (c *wrappedRbacV1beta1RoleBindings) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.RoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/RoleBindings", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedRbacV1beta1Roles wrapper
type wrappedRbacV1beta1Roles struct {
	inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface
}

func newRbacV1beta1Roles(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface {
	return &wrappedRbacV1beta1Roles{inner}
}
func (c *wrappedRbacV1beta1Roles) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.RoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "RbacV1beta1/Roles", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedSchedulingV1PriorityClasses wrapper
type wrappedSchedulingV1PriorityClasses struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface
}

func newSchedulingV1PriorityClasses(inner k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface) k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface {
	return &wrappedSchedulingV1PriorityClasses{inner}
}
func (c *wrappedSchedulingV1PriorityClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1.PriorityClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1/PriorityClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedSchedulingV1alpha1PriorityClasses wrapper
type wrappedSchedulingV1alpha1PriorityClasses struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface
}

func newSchedulingV1alpha1PriorityClasses(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	return &wrappedSchedulingV1alpha1PriorityClasses{inner}
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1alpha1.PriorityClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1alpha1/PriorityClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedSchedulingV1beta1PriorityClasses wrapper
type wrappedSchedulingV1beta1PriorityClasses struct {
	inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface
}

func newSchedulingV1beta1PriorityClasses(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	return &wrappedSchedulingV1beta1PriorityClasses{inner}
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1beta1.PriorityClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "SchedulingV1beta1/PriorityClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1CSIDrivers wrapper
type wrappedStorageV1CSIDrivers struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface
}

func newStorageV1CSIDrivers(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface) k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface {
	return &wrappedStorageV1CSIDrivers{inner}
}
func (c *wrappedStorageV1CSIDrivers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSIDriverApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIDriverList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSIDrivers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSIDrivers) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIDrivers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1CSINodes wrapper
type wrappedStorageV1CSINodes struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface
}

func newStorageV1CSINodes(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface) k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface {
	return &wrappedStorageV1CSINodes{inner}
}
func (c *wrappedStorageV1CSINodes) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSINodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSINodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSINodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSINodes) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSINodes", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1CSIStorageCapacities wrapper
type wrappedStorageV1CSIStorageCapacities struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface
}

func newStorageV1CSIStorageCapacities(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface) k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface {
	return &wrappedStorageV1CSIStorageCapacities{inner}
}
func (c *wrappedStorageV1CSIStorageCapacities) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSIStorageCapacityApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/CSIStorageCapacities", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1StorageClasses wrapper
type wrappedStorageV1StorageClasses struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface
}

func newStorageV1StorageClasses(inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface) k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface {
	return &wrappedStorageV1StorageClasses{inner}
}
func (c *wrappedStorageV1StorageClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.StorageClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.StorageClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1StorageClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1StorageClasses) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/StorageClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1VolumeAttachments wrapper
type wrappedStorageV1VolumeAttachments struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface
}

func newStorageV1VolumeAttachments(inner k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface) k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface {
	return &wrappedStorageV1VolumeAttachments{inner}
}
func (c *wrappedStorageV1VolumeAttachments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1/VolumeAttachments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1alpha1CSIStorageCapacities wrapper
type wrappedStorageV1alpha1CSIStorageCapacities struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface
}

func newStorageV1alpha1CSIStorageCapacities(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	return &wrappedStorageV1alpha1CSIStorageCapacities{inner}
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.CSIStorageCapacityApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/CSIStorageCapacities", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1alpha1VolumeAttachments wrapper
type wrappedStorageV1alpha1VolumeAttachments struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface
}

func newStorageV1alpha1VolumeAttachments(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	return &wrappedStorageV1alpha1VolumeAttachments{inner}
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1alpha1/VolumeAttachments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1beta1CSIDrivers wrapper
type wrappedStorageV1beta1CSIDrivers struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface
}

func newStorageV1beta1CSIDrivers(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	return &wrappedStorageV1beta1CSIDrivers{inner}
}
func (c *wrappedStorageV1beta1CSIDrivers) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSIDriverApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIDriverList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSIDrivers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSIDrivers) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIDrivers", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1beta1CSINodes wrapper
type wrappedStorageV1beta1CSINodes struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface
}

func newStorageV1beta1CSINodes(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	return &wrappedStorageV1beta1CSINodes{inner}
}
func (c *wrappedStorageV1beta1CSINodes) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSINodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSINodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSINodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSINodes) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSINodes", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1beta1CSIStorageCapacities wrapper
type wrappedStorageV1beta1CSIStorageCapacities struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface
}

func newStorageV1beta1CSIStorageCapacities(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	return &wrappedStorageV1beta1CSIStorageCapacities{inner}
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSIStorageCapacityApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/CSIStorageCapacities", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1beta1StorageClasses wrapper
type wrappedStorageV1beta1StorageClasses struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface
}

func newStorageV1beta1StorageClasses(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	return &wrappedStorageV1beta1StorageClasses{inner}
}
func (c *wrappedStorageV1beta1StorageClasses) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.StorageClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.StorageClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1StorageClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1StorageClasses) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/StorageClasses", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedStorageV1beta1VolumeAttachments wrapper
type wrappedStorageV1beta1VolumeAttachments struct {
	inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface
}

func newStorageV1beta1VolumeAttachments(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface) k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	return &wrappedStorageV1beta1VolumeAttachments{inner}
}
func (c *wrappedStorageV1beta1VolumeAttachments) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Apply", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "ApplyStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "StorageV1beta1/VolumeAttachments", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}
