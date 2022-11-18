package client

import (
	context "context"

	github_com_google_gnostic_openapiv2 "github.com/google/gnostic/openapiv2"
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
	k8s_io_apimachinery_pkg_version "k8s.io/apimachinery/pkg/version"
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
	k8s_io_client_go_openapi "k8s.io/client-go/openapi"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

// Wrap
func Wrap(inner k8s_io_client_go_kubernetes.Interface) k8s_io_client_go_kubernetes.Interface {
	return &clientset{
		discovery:                    newDiscoveryInterface(inner.Discovery()),
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
	discovery                    k8s_io_client_go_discovery.DiscoveryInterface
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

func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.discovery
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

// wrappedDiscoveryInterface
type wrappedDiscoveryInterface struct {
	inner k8s_io_client_go_discovery.DiscoveryInterface
}

func newDiscoveryInterface(inner k8s_io_client_go_discovery.DiscoveryInterface) k8s_io_client_go_discovery.DiscoveryInterface {
	return &wrappedDiscoveryInterface{inner}
}
func (c *wrappedDiscoveryInterface) OpenAPISchema() (*github_com_google_gnostic_openapiv2.Document, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/OpenAPISchema",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "OpenAPISchema"),
	)
	defer span.End()
	return c.inner.OpenAPISchema()
}
func (c *wrappedDiscoveryInterface) OpenAPIV3() k8s_io_client_go_openapi.Client {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/OpenAPIV3",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "OpenAPIV3"),
	)
	defer span.End()
	return c.inner.OpenAPIV3()
}
func (c *wrappedDiscoveryInterface) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerGroups",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerGroups"),
	)
	defer span.End()
	return c.inner.ServerGroups()
}
func (c *wrappedDiscoveryInterface) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerGroupsAndResources",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerGroupsAndResources"),
	)
	defer span.End()
	return c.inner.ServerGroupsAndResources()
}
func (c *wrappedDiscoveryInterface) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerPreferredNamespacedResources",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerPreferredNamespacedResources"),
	)
	defer span.End()
	return c.inner.ServerPreferredNamespacedResources()
}
func (c *wrappedDiscoveryInterface) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerPreferredResources",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerPreferredResources"),
	)
	defer span.End()
	return c.inner.ServerPreferredResources()
}
func (c *wrappedDiscoveryInterface) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerResourcesForGroupVersion",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerResourcesForGroupVersion"),
	)
	defer span.End()
	return c.inner.ServerResourcesForGroupVersion(arg0)
}
func (c *wrappedDiscoveryInterface) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE Discovery/ServerVersion",
		go_opentelemetry_io_otel_attribute.String("client", "Discovery"),
		go_opentelemetry_io_otel_attribute.String("operation", "ServerVersion"),
	)
	defer span.End()
	return c.inner.ServerVersion()
}
func (c *wrappedDiscoveryInterface) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1MutatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/MutatingWebhookConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1ValidatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1/ValidatingWebhookConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1MutatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/MutatingWebhookConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "MutatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "MutatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1ValidatingWebhookConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AdmissionregistrationV1beta1/ValidatingWebhookConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AdmissionregistrationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ValidatingWebhookConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "ValidatingWebhookConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ControllerRevision/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/DaemonSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/Deployment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/ReplicaSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1/StatefulSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/ControllerRevision/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/Deployment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta1/StatefulSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.ControllerRevisionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ControllerRevisions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ControllerRevision/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ControllerRevisions"),
		go_opentelemetry_io_otel_attribute.String("kind", "ControllerRevision"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/DaemonSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/Deployment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/ReplicaSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2StatefulSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.StatefulSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2StatefulSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2StatefulSets) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_apps_v1beta2.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2StatefulSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2StatefulSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AppsV1beta2/StatefulSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AppsV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "StatefulSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "StatefulSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthenticationV1/TokenReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthenticationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "TokenReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "TokenReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthenticationV1beta1/TokenReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthenticationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "TokenReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "TokenReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1/LocalSubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LocalSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "LocalSubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1/SelfSubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1/SelfSubjectRulesReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectRulesReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectRulesReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1/SubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1beta1/LocalSubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LocalSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "LocalSubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1beta1/SelfSubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1beta1/SelfSubjectRulesReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SelfSubjectRulesReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SelfSubjectRulesReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AuthorizationV1beta1/SubjectAccessReview/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AuthorizationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "SubjectAccessReviews"),
		go_opentelemetry_io_otel_attribute.String("kind", "SubjectAccessReview"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/List",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV1/HorizontalPodAutoscaler/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/List",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2/HorizontalPodAutoscaler/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/List",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta1/HorizontalPodAutoscaler/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Create",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Get",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscalerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/List",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Update",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2HorizontalPodAutoscalers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE AutoscalingV2beta2/HorizontalPodAutoscaler/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "AutoscalingV2beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "HorizontalPodAutoscalers"),
		go_opentelemetry_io_otel_attribute.String("kind", "HorizontalPodAutoscaler"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Create",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Get",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.CronJobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/List",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1CronJobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1CronJobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Update",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1CronJobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/CronJob/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.JobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Create",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Get",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.JobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/List",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1Jobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1Jobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Update",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1Jobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1/Job/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Jobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "Job"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1beta1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Create",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Get",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1beta1.CronJobList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/List",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1beta1CronJobs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1beta1CronJobs) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Update",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1CronJobs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE BatchV1beta1/CronJob/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "BatchV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CronJobs"),
		go_opentelemetry_io_otel_attribute.String("kind", "CronJob"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/List",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) UpdateApproval(arg0 context.Context, arg1 string, arg2 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/UpdateApproval",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateApproval"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateApproval(arg0, arg1, arg2, arg3)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1CertificateSigningRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1/CertificateSigningRequest/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1beta1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/List",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) UpdateApproval(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/UpdateApproval",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateApproval"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateApproval(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1CertificateSigningRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CertificatesV1beta1/CertificateSigningRequest/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CertificatesV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CertificateSigningRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "CertificateSigningRequest"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_coordination_v1.LeaseList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1Leases) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1Leases) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1Leases) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1/Lease/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_coordination_v1beta1.LeaseList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1beta1Leases) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1beta1Leases) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1Leases) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoordinationV1beta1/Lease/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoordinationV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Leases"),
		go_opentelemetry_io_otel_attribute.String("kind", "Lease"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ComponentStatusList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ComponentStatuses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ComponentStatuses) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ComponentStatuses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ComponentStatus/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ComponentStatuses"),
		go_opentelemetry_io_otel_attribute.String("kind", "ComponentStatus"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ConfigMapList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ConfigMaps) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ConfigMaps) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ConfigMaps) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ConfigMap/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ConfigMaps"),
		go_opentelemetry_io_otel_attribute.String("kind", "ConfigMap"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EndpointsList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Endpoints) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Endpoints) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Endpoints) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Endpoints/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("kind", "Endpoints"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/CreateWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "CreateWithEventNamespace"),
	)
	defer span.End()
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/GetFieldSelector",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetFieldSelector"),
	)
	defer span.End()
	return c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Events) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/PatchWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "PatchWithEventNamespace"),
	)
	defer span.End()
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedCoreV1Events) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Search",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Search"),
	)
	defer span.End()
	return c.inner.Search(arg0, arg1)
}
func (c *wrappedCoreV1Events) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Events) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/UpdateWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateWithEventNamespace"),
	)
	defer span.End()
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Event/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.LimitRangeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1LimitRanges) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1LimitRanges) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1LimitRanges) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/LimitRange/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "LimitRanges"),
		go_opentelemetry_io_otel_attribute.String("kind", "LimitRange"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NamespaceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Finalize(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Finalize",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Finalize"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Finalize(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.NamespaceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Namespaces) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Namespaces) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Namespaces) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Namespace/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Namespaces"),
		go_opentelemetry_io_otel_attribute.String("kind", "Namespace"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.NodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Nodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Nodes) PatchStatus(arg0 context.Context, arg1 string, arg2 []uint8) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/PatchStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "PatchStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.PatchStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Nodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Node/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Nodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "Node"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeClaimApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeClaimList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumeClaims) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolumeClaim/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumeClaims"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolumeClaim"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PersistentVolumes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PersistentVolumes) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PersistentVolumes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PersistentVolume/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PersistentVolumes"),
		go_opentelemetry_io_otel_attribute.String("kind", "PersistentVolume"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodTemplateList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1PodTemplates) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1PodTemplates) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1PodTemplates) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/PodTemplate/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodTemplates"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodTemplate"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Bind(arg0 context.Context, arg1 *k8s_io_api_core_v1.Binding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Bind",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Bind"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Bind(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Evict",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Evict"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Evict(arg0, arg1)
}
func (c *wrappedCoreV1Pods) EvictV1(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/EvictV1",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "EvictV1"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.EvictV1(arg0, arg1)
}
func (c *wrappedCoreV1Pods) EvictV1beta1(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/EvictV1beta1",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "EvictV1beta1"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.EvictV1beta1(arg0, arg1)
}
func (c *wrappedCoreV1Pods) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) GetLogs(arg0 string, arg1 *k8s_io_api_core_v1.PodLogOptions) *k8s_io_client_go_rest.Request {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/GetLogs",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetLogs"),
	)
	defer span.End()
	return c.inner.GetLogs(arg0, arg1)
}
func (c *wrappedCoreV1Pods) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Pods) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Pods) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/ProxyGet",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "ProxyGet"),
	)
	defer span.End()
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1Pods) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) UpdateEphemeralContainers(arg0 context.Context, arg1 string, arg2 *k8s_io_api_core_v1.Pod, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/UpdateEphemeralContainers",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateEphemeralContainers"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateEphemeralContainers(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1Pods) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Pods) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Pod/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Pods"),
		go_opentelemetry_io_otel_attribute.String("kind", "Pod"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ReplicationControllerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ReplicationControllerList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ReplicationControllers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ReplicationControllers) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1ReplicationControllers) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ReplicationControllers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ReplicationController/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicationControllers"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicationController"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ResourceQuotaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ResourceQuotaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ResourceQuotas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ResourceQuotas) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ResourceQuotas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ResourceQuota/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ResourceQuotas"),
		go_opentelemetry_io_otel_attribute.String("kind", "ResourceQuota"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.SecretList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Secrets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Secrets) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Secret, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Secrets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Secret/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Secrets"),
		go_opentelemetry_io_otel_attribute.String("kind", "Secret"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) CreateToken(arg0 context.Context, arg1 string, arg2 *k8s_io_api_authentication_v1.TokenRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/CreateToken",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "CreateToken"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.CreateToken(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1ServiceAccounts) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceAccountList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1ServiceAccounts) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1ServiceAccounts) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1ServiceAccounts) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/ServiceAccount/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ServiceAccounts"),
		go_opentelemetry_io_otel_attribute.String("kind", "ServiceAccount"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Create",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Get",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/List",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1Services) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1Services) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/ProxyGet",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "ProxyGet"),
	)
	defer span.End()
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1Services) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Update",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1Services) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE CoreV1/Service/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "CoreV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Services"),
		go_opentelemetry_io_otel_attribute.String("kind", "Service"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Create",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Get",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_discovery_v1.EndpointSliceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/List",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1EndpointSlices) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1EndpointSlices) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Update",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1EndpointSlices) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1/EndpointSlice/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Create",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Get",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_discovery_v1beta1.EndpointSliceList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/List",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Update",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1EndpointSlices) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE DiscoveryV1beta1/EndpointSlice/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "DiscoveryV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "EndpointSlices"),
		go_opentelemetry_io_otel_attribute.String("kind", "EndpointSlice"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Create",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Get",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_events_v1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/List",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1Events) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Update",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1/Event/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Create",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) CreateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/CreateWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "CreateWithEventNamespace"),
	)
	defer span.End()
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1Events) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Get",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_events_v1beta1.EventList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/List",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1beta1Events) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1beta1Events) PatchWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event, arg1 []uint8) (*k8s_io_api_events_v1beta1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/PatchWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "PatchWithEventNamespace"),
	)
	defer span.End()
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedEventsV1beta1Events) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Update",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1Events) UpdateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	_, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		context.TODO(),
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/UpdateWithEventNamespace",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateWithEventNamespace"),
	)
	defer span.End()
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1Events) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE EventsV1beta1/Event/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "EventsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Events"),
		go_opentelemetry_io_otel_attribute.String("kind", "Event"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DaemonSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1DaemonSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/DaemonSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "DaemonSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "DaemonSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1Deployments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1Deployments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1Deployments) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Rollback",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Rollback"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Rollback(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1Deployments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Deployments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Deployment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Deployments"),
		go_opentelemetry_io_otel_attribute.String("kind", "Deployment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/Ingress/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1NetworkPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/NetworkPolicy/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1PodSecurityPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/PodSecurityPolicy/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/ApplyScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Create",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Get",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/GetScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/List",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Update",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/UpdateScale",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1ReplicaSets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE ExtensionsV1beta1/ReplicaSet/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "ExtensionsV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ReplicaSets"),
		go_opentelemetry_io_otel_attribute.String("kind", "ReplicaSet"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/FlowSchema/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1alpha1/PriorityLevelConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/FlowSchema/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta1/PriorityLevelConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchemaList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2FlowSchemas) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/FlowSchema/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "FlowSchemas"),
		go_opentelemetry_io_otel_attribute.String("kind", "FlowSchema"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Create",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Get",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfigurationList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/List",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Update",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2PriorityLevelConfigurations) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE FlowcontrolV1beta2/PriorityLevelConfiguration/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "FlowcontrolV1beta2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityLevelConfigurations"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityLevelConfiguration"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1.StorageVersionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Create(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Create",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Get",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersionList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/List",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedInternalV1alpha1StorageVersions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedInternalV1alpha1StorageVersions) Update(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Update",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1StorageVersions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE InternalV1alpha1/StorageVersion/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "InternalV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageVersions"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageVersion"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.IngressClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1IngressClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1IngressClasses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1IngressClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/IngressClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/Ingress/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.NetworkPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1NetworkPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1NetworkPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1NetworkPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1/NetworkPolicy/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "NetworkPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "NetworkPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDRList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1ClusterCIDRs) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1alpha1/ClusterCIDR/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCIDRs"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCIDR"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1IngressClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/IngressClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "IngressClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "IngressClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/List",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1Ingresses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1Ingresses) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1Ingresses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NetworkingV1beta1/Ingress/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NetworkingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Ingresses"),
		go_opentelemetry_io_otel_attribute.String("kind", "Ingress"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1/RuntimeClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1alpha1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1alpha1/RuntimeClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1beta1.RuntimeClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1RuntimeClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE NodeV1beta1/RuntimeClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "NodeV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RuntimeClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "RuntimeClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/Eviction/Evict",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Evictions"),
		go_opentelemetry_io_otel_attribute.String("kind", "Eviction"),
		go_opentelemetry_io_otel_attribute.String("operation", "Evict"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Create",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Get",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1.PodDisruptionBudgetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/List",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Update",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1PodDisruptionBudgets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1/PodDisruptionBudget/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/Eviction/Evict",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Evictions"),
		go_opentelemetry_io_otel_attribute.String("kind", "Eviction"),
		go_opentelemetry_io_otel_attribute.String("operation", "Evict"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Create",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Get",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudgetList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/List",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Update",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodDisruptionBudgets) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodDisruptionBudget/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodDisruptionBudgets"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodDisruptionBudget"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Create",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Get",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/List",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Update",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1PodSecurityPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE PolicyV1beta1/PodSecurityPolicy/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "PolicyV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PodSecurityPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "PodSecurityPolicy"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/ClusterRole/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/RoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1/Role/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/ClusterRole/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/RoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1alpha1/Role/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1ClusterRoles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1ClusterRoles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1ClusterRoles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/ClusterRole/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterRoles"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterRole"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleBindingList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1RoleBindings) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1RoleBindings) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1RoleBindings) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/RoleBinding/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "RoleBindings"),
		go_opentelemetry_io_otel_attribute.String("kind", "RoleBinding"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Create",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Get",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/List",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1Roles) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1Roles) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Update",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1Roles) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE RbacV1beta1/Role/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "RbacV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Roles"),
		go_opentelemetry_io_otel_attribute.String("kind", "Role"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1/PriorityClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1alpha1/PriorityClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1PriorityClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE SchedulingV1beta1/PriorityClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "SchedulingV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "PriorityClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "PriorityClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIDriverList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSIDrivers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSIDrivers) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIDrivers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIDriver/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSINodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSINodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSINodes) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSINodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSINode/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/CSIStorageCapacity/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.StorageClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1StorageClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1StorageClasses) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1StorageClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/StorageClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1/VolumeAttachment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/CSIStorageCapacity/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1alpha1/VolumeAttachment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIDriverList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSIDrivers) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSIDrivers) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIDrivers) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIDriver/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIDrivers"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIDriver"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSINodeList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSINodes) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSINodes) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSINodes) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSINode/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSINodes"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSINode"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacityList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1CSIStorageCapacities) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/CSIStorageCapacity/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CSIStorageCapacities"),
		go_opentelemetry_io_otel_attribute.String("kind", "CSIStorageCapacity"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.StorageClassList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1StorageClasses) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1StorageClasses) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1StorageClasses) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/StorageClass/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "StorageClasses"),
		go_opentelemetry_io_otel_attribute.String("kind", "StorageClass"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
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
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Apply",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/ApplyStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Create",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Delete",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/DeleteCollection",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Get",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachmentList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/List",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Patch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Update",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/UpdateStatus",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1VolumeAttachments) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(
		arg0,
		"pkg/clients/wrappers/traces/kube",
		"KUBE StorageV1beta1/VolumeAttachment/Watch",
		go_opentelemetry_io_otel_attribute.String("client", "StorageV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "VolumeAttachments"),
		go_opentelemetry_io_otel_attribute.String("kind", "VolumeAttachment"),
		go_opentelemetry_io_otel_attribute.String("operation", "Watch"),
	)
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}
