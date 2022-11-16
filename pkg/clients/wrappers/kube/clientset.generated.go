package client

import (
	context "context"

	metrics "github.com/kyverno/kyverno/pkg/metrics"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_fields "k8s.io/apimachinery/pkg/fields"
	k8s_io_apimachinery_pkg_runtime "k8s.io/apimachinery/pkg/runtime"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	watch "k8s.io/apimachinery/pkg/watch"
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
	discovery "k8s.io/client-go/discovery"
	versioned "k8s.io/client-go/kubernetes"
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
	restclient "k8s.io/client-go/rest"
)

type clientset struct {
	inner                        versioned.Interface
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

func (c *clientset) Discovery() discovery.DiscoveryInterface {
	return c.inner.Discovery()
}

func Wrap(inner versioned.Interface, m metrics.MetricsConfigManager) versioned.Interface {
	return &clientset{
		inner:                        inner,
		admissionregistrationv1:      wrapAdmissionregistrationV1Interface(inner.AdmissionregistrationV1(), m),
		admissionregistrationv1beta1: wrapAdmissionregistrationV1beta1Interface(inner.AdmissionregistrationV1beta1(), m),
		appsv1:                       wrapAppsV1Interface(inner.AppsV1(), m),
		appsv1beta1:                  wrapAppsV1beta1Interface(inner.AppsV1beta1(), m),
		appsv1beta2:                  wrapAppsV1beta2Interface(inner.AppsV1beta2(), m),
		authenticationv1:             wrapAuthenticationV1Interface(inner.AuthenticationV1(), m),
		authenticationv1beta1:        wrapAuthenticationV1beta1Interface(inner.AuthenticationV1beta1(), m),
		authorizationv1:              wrapAuthorizationV1Interface(inner.AuthorizationV1(), m),
		authorizationv1beta1:         wrapAuthorizationV1beta1Interface(inner.AuthorizationV1beta1(), m),
		autoscalingv1:                wrapAutoscalingV1Interface(inner.AutoscalingV1(), m),
		autoscalingv2:                wrapAutoscalingV2Interface(inner.AutoscalingV2(), m),
		autoscalingv2beta1:           wrapAutoscalingV2beta1Interface(inner.AutoscalingV2beta1(), m),
		autoscalingv2beta2:           wrapAutoscalingV2beta2Interface(inner.AutoscalingV2beta2(), m),
		batchv1:                      wrapBatchV1Interface(inner.BatchV1(), m),
		batchv1beta1:                 wrapBatchV1beta1Interface(inner.BatchV1beta1(), m),
		certificatesv1:               wrapCertificatesV1Interface(inner.CertificatesV1(), m),
		certificatesv1beta1:          wrapCertificatesV1beta1Interface(inner.CertificatesV1beta1(), m),
		coordinationv1:               wrapCoordinationV1Interface(inner.CoordinationV1(), m),
		coordinationv1beta1:          wrapCoordinationV1beta1Interface(inner.CoordinationV1beta1(), m),
		corev1:                       wrapCoreV1Interface(inner.CoreV1(), m),
		discoveryv1:                  wrapDiscoveryV1Interface(inner.DiscoveryV1(), m),
		discoveryv1beta1:             wrapDiscoveryV1beta1Interface(inner.DiscoveryV1beta1(), m),
		eventsv1:                     wrapEventsV1Interface(inner.EventsV1(), m),
		eventsv1beta1:                wrapEventsV1beta1Interface(inner.EventsV1beta1(), m),
		extensionsv1beta1:            wrapExtensionsV1beta1Interface(inner.ExtensionsV1beta1(), m),
		flowcontrolv1alpha1:          wrapFlowcontrolV1alpha1Interface(inner.FlowcontrolV1alpha1(), m),
		flowcontrolv1beta1:           wrapFlowcontrolV1beta1Interface(inner.FlowcontrolV1beta1(), m),
		flowcontrolv1beta2:           wrapFlowcontrolV1beta2Interface(inner.FlowcontrolV1beta2(), m),
		internalv1alpha1:             wrapInternalV1alpha1Interface(inner.InternalV1alpha1(), m),
		networkingv1:                 wrapNetworkingV1Interface(inner.NetworkingV1(), m),
		networkingv1alpha1:           wrapNetworkingV1alpha1Interface(inner.NetworkingV1alpha1(), m),
		networkingv1beta1:            wrapNetworkingV1beta1Interface(inner.NetworkingV1beta1(), m),
		nodev1:                       wrapNodeV1Interface(inner.NodeV1(), m),
		nodev1alpha1:                 wrapNodeV1alpha1Interface(inner.NodeV1alpha1(), m),
		nodev1beta1:                  wrapNodeV1beta1Interface(inner.NodeV1beta1(), m),
		policyv1:                     wrapPolicyV1Interface(inner.PolicyV1(), m),
		policyv1beta1:                wrapPolicyV1beta1Interface(inner.PolicyV1beta1(), m),
		rbacv1:                       wrapRbacV1Interface(inner.RbacV1(), m),
		rbacv1alpha1:                 wrapRbacV1alpha1Interface(inner.RbacV1alpha1(), m),
		rbacv1beta1:                  wrapRbacV1beta1Interface(inner.RbacV1beta1(), m),
		schedulingv1:                 wrapSchedulingV1Interface(inner.SchedulingV1(), m),
		schedulingv1alpha1:           wrapSchedulingV1alpha1Interface(inner.SchedulingV1alpha1(), m),
		schedulingv1beta1:            wrapSchedulingV1beta1Interface(inner.SchedulingV1beta1(), m),
		storagev1:                    wrapStorageV1Interface(inner.StorageV1(), m),
		storagev1alpha1:              wrapStorageV1alpha1Interface(inner.StorageV1alpha1(), m),
		storagev1beta1:               wrapStorageV1beta1Interface(inner.StorageV1beta1(), m),
	}
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

type wrappedAdmissionregistrationV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAdmissionregistrationV1Interface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return &wrappedAdmissionregistrationV1Interface{inner, metrics}
}

type wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface
	recorder metrics.Recorder
}

func wrapAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface{inner, recorder}
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1.MutatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 metav1.CreateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAdmissionregistrationV1Interface) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingWebhookConfiguration", metrics.KubeClient)
	return wrapAdmissionregistrationV1InterfaceMutatingWebhookConfigurationInterface(c.inner.MutatingWebhookConfigurations(), recorder)
}

type wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface
	recorder metrics.Recorder
}

func wrapAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface{inner, recorder}
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1.ValidatingWebhookConfigurationApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, arg2 metav1.UpdateOptions) (*k8s_io_api_admissionregistration_v1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAdmissionregistrationV1Interface) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingWebhookConfiguration", metrics.KubeClient)
	return wrapAdmissionregistrationV1InterfaceValidatingWebhookConfigurationInterface(c.inner.ValidatingWebhookConfigurations(), recorder)
}
func (c *wrappedAdmissionregistrationV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAdmissionregistrationV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAdmissionregistrationV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface {
	return &wrappedAdmissionregistrationV1beta1Interface{inner, metrics}
}

type wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface
	recorder metrics.Recorder
}

func wrapAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface{inner, recorder}
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1beta1.MutatingWebhookConfigurationApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.MutatingWebhookConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAdmissionregistrationV1beta1Interface) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.MutatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingWebhookConfiguration", metrics.KubeClient)
	return wrapAdmissionregistrationV1beta1InterfaceMutatingWebhookConfigurationInterface(c.inner.MutatingWebhookConfigurations(), recorder)
}

type wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface
	recorder metrics.Recorder
}

func wrapAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface {
	return &wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface{inner, recorder}
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1beta1.ValidatingWebhookConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, arg2 metav1.UpdateOptions) (*k8s_io_api_admissionregistration_v1beta1.ValidatingWebhookConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAdmissionregistrationV1beta1Interface) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.ValidatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingWebhookConfiguration", metrics.KubeClient)
	return wrapAdmissionregistrationV1beta1InterfaceValidatingWebhookConfigurationInterface(c.inner.ValidatingWebhookConfigurations(), recorder)
}
func (c *wrappedAdmissionregistrationV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAppsV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAppsV1Interface(inner k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface {
	return &wrappedAppsV1Interface{inner, metrics}
}

type wrappedAppsV1InterfaceControllerRevisionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface
	recorder metrics.Recorder
}

func wrapAppsV1InterfaceControllerRevisionInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	return &wrappedAppsV1InterfaceControllerRevisionInterface{inner, recorder}
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ControllerRevisionApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ControllerRevisionList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ControllerRevision, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceControllerRevisionInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1Interface) ControllerRevisions(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ControllerRevision", metrics.KubeClient)
	return wrapAppsV1InterfaceControllerRevisionInterface(c.inner.ControllerRevisions(arg0), recorder)
}

type wrappedAppsV1InterfaceDaemonSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1InterfaceDaemonSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	return &wrappedAppsV1InterfaceDaemonSetInterface{inner, recorder}
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DaemonSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.DaemonSet, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1.DaemonSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDaemonSetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1Interface) DaemonSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "DaemonSet", metrics.KubeClient)
	return wrapAppsV1InterfaceDaemonSetInterface(c.inner.DaemonSets(arg0), recorder)
}

type wrappedAppsV1InterfaceDeploymentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface
	recorder metrics.Recorder
}

func wrapAppsV1InterfaceDeploymentInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	return &wrappedAppsV1InterfaceDeploymentInterface{inner, recorder}
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.DeploymentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.Deployment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceDeploymentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1Interface) Deployments(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Deployment", metrics.KubeClient)
	return wrapAppsV1InterfaceDeploymentInterface(c.inner.Deployments(arg0), recorder)
}

type wrappedAppsV1InterfaceReplicaSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1InterfaceReplicaSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return &wrappedAppsV1InterfaceReplicaSetInterface{inner, recorder}
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ReplicaSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceReplicaSetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1Interface) ReplicaSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ReplicaSet", metrics.KubeClient)
	return wrapAppsV1InterfaceReplicaSetInterface(c.inner.ReplicaSets(arg0), recorder)
}

type wrappedAppsV1InterfaceStatefulSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1InterfaceStatefulSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	return &wrappedAppsV1InterfaceStatefulSetInterface{inner, recorder}
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.StatefulSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.StatefulSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) GetScale(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.StatefulSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 metav1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.StatefulSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1InterfaceStatefulSetInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1Interface) StatefulSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "StatefulSet", metrics.KubeClient)
	return wrapAppsV1InterfaceStatefulSetInterface(c.inner.StatefulSets(arg0), recorder)
}
func (c *wrappedAppsV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAppsV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAppsV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface {
	return &wrappedAppsV1beta1Interface{inner, metrics}
}

type wrappedAppsV1beta1InterfaceControllerRevisionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta1InterfaceControllerRevisionInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface {
	return &wrappedAppsV1beta1InterfaceControllerRevisionInterface{inner, recorder}
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.ControllerRevisionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.ControllerRevisionList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.ControllerRevision, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceControllerRevisionInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta1Interface) ControllerRevisions(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.ControllerRevisionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ControllerRevision", metrics.KubeClient)
	return wrapAppsV1beta1InterfaceControllerRevisionInterface(c.inner.ControllerRevisions(arg0), recorder)
}

type wrappedAppsV1beta1InterfaceDeploymentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta1InterfaceDeploymentInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface {
	return &wrappedAppsV1beta1InterfaceDeploymentInterface{inner, recorder}
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.DeploymentApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.DeploymentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.Deployment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceDeploymentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta1Interface) Deployments(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Deployment", metrics.KubeClient)
	return wrapAppsV1beta1InterfaceDeploymentInterface(c.inner.Deployments(arg0), recorder)
}

type wrappedAppsV1beta1InterfaceStatefulSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta1InterfaceStatefulSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface {
	return &wrappedAppsV1beta1InterfaceStatefulSetInterface{inner, recorder}
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta1.StatefulSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta1.StatefulSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta1.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta1.StatefulSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta1InterfaceStatefulSetInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta1Interface) StatefulSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta1.StatefulSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "StatefulSet", metrics.KubeClient)
	return wrapAppsV1beta1InterfaceStatefulSetInterface(c.inner.StatefulSets(arg0), recorder)
}
func (c *wrappedAppsV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAppsV1beta2Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface
	metrics metrics.MetricsConfigManager
}

func wrapAppsV1beta2Interface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface {
	return &wrappedAppsV1beta2Interface{inner, metrics}
}

type wrappedAppsV1beta2InterfaceControllerRevisionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta2InterfaceControllerRevisionInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface {
	return &wrappedAppsV1beta2InterfaceControllerRevisionInterface{inner, recorder}
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ControllerRevisionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_apps_v1beta2.ControllerRevisionList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ControllerRevision, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ControllerRevision, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceControllerRevisionInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta2Interface) ControllerRevisions(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ControllerRevisionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ControllerRevision", metrics.KubeClient)
	return wrapAppsV1beta2InterfaceControllerRevisionInterface(c.inner.ControllerRevisions(arg0), recorder)
}

type wrappedAppsV1beta2InterfaceDaemonSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta2InterfaceDaemonSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface {
	return &wrappedAppsV1beta2InterfaceDaemonSetInterface{inner, recorder}
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DaemonSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DaemonSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DaemonSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.DaemonSet, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta2.DaemonSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDaemonSetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta2Interface) DaemonSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DaemonSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "DaemonSet", metrics.KubeClient)
	return wrapAppsV1beta2InterfaceDaemonSetInterface(c.inner.DaemonSets(arg0), recorder)
}

type wrappedAppsV1beta2InterfaceDeploymentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta2InterfaceDeploymentInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface {
	return &wrappedAppsV1beta2InterfaceDeploymentInterface{inner, recorder}
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.DeploymentApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 metav1.CreateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.DeploymentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.Deployment, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Deployment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceDeploymentInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta2Interface) Deployments(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Deployment", metrics.KubeClient)
	return wrapAppsV1beta2InterfaceDeploymentInterface(c.inner.Deployments(arg0), recorder)
}

type wrappedAppsV1beta2InterfaceReplicaSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta2InterfaceReplicaSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface {
	return &wrappedAppsV1beta2InterfaceReplicaSetInterface{inner, recorder}
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 metav1.CreateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1beta2.ReplicaSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.ReplicaSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceReplicaSetInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta2Interface) ReplicaSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.ReplicaSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ReplicaSet", metrics.KubeClient)
	return wrapAppsV1beta2InterfaceReplicaSetInterface(c.inner.ReplicaSets(arg0), recorder)
}

type wrappedAppsV1beta2InterfaceStatefulSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface
	recorder metrics.Recorder
}

func wrapAppsV1beta2InterfaceStatefulSetInterface(inner k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface {
	return &wrappedAppsV1beta2InterfaceStatefulSetInterface{inner, recorder}
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_apps_v1beta2.ScaleApplyConfiguration, arg3 metav1.ApplyOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1beta2.StatefulSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 metav1.CreateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) GetScale(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_apps_v1beta2.StatefulSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_apps_v1beta2.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1beta2.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1beta2.StatefulSet, arg2 metav1.UpdateOptions) (*k8s_io_api_apps_v1beta2.StatefulSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAppsV1beta2InterfaceStatefulSetInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAppsV1beta2Interface) StatefulSets(arg0 string) k8s_io_client_go_kubernetes_typed_apps_v1beta2.StatefulSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "StatefulSet", metrics.KubeClient)
	return wrapAppsV1beta2InterfaceStatefulSetInterface(c.inner.StatefulSets(arg0), recorder)
}
func (c *wrappedAppsV1beta2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAuthenticationV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAuthenticationV1Interface(inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return &wrappedAuthenticationV1Interface{inner, metrics}
}

type wrappedAuthenticationV1InterfaceTokenReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface
	recorder metrics.Recorder
}

func wrapAuthenticationV1InterfaceTokenReviewInterface(inner k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return &wrappedAuthenticationV1InterfaceTokenReviewInterface{inner, recorder}
}
func (c *wrappedAuthenticationV1InterfaceTokenReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1.TokenReview, arg2 metav1.CreateOptions) (*k8s_io_api_authentication_v1.TokenReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthenticationV1Interface) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "TokenReview", metrics.KubeClient)
	return wrapAuthenticationV1InterfaceTokenReviewInterface(c.inner.TokenReviews(), recorder)
}
func (c *wrappedAuthenticationV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAuthenticationV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAuthenticationV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface {
	return &wrappedAuthenticationV1beta1Interface{inner, metrics}
}

type wrappedAuthenticationV1beta1InterfaceTokenReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface
	recorder metrics.Recorder
}

func wrapAuthenticationV1beta1InterfaceTokenReviewInterface(inner k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface {
	return &wrappedAuthenticationV1beta1InterfaceTokenReviewInterface{inner, recorder}
}
func (c *wrappedAuthenticationV1beta1InterfaceTokenReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1beta1.TokenReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1beta1.TokenReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthenticationV1beta1Interface) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1beta1.TokenReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "TokenReview", metrics.KubeClient)
	return wrapAuthenticationV1beta1InterfaceTokenReviewInterface(c.inner.TokenReviews(), recorder)
}
func (c *wrappedAuthenticationV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAuthorizationV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAuthorizationV1Interface(inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return &wrappedAuthorizationV1Interface{inner, metrics}
}

type wrappedAuthorizationV1InterfaceLocalSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1InterfaceLocalSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1InterfaceLocalSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1InterfaceLocalSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.LocalSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.LocalSubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1Interface) LocalSubjectAccessReviews(arg0 string) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "LocalSubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1InterfaceLocalSubjectAccessReviewInterface(c.inner.LocalSubjectAccessReviews(arg0), recorder)
}

type wrappedAuthorizationV1InterfaceSelfSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1InterfaceSelfSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1InterfaceSelfSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1InterfaceSelfSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SelfSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.SelfSubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1Interface) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1InterfaceSelfSubjectAccessReviewInterface(c.inner.SelfSubjectAccessReviews(), recorder)
}

type wrappedAuthorizationV1InterfaceSelfSubjectRulesReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1InterfaceSelfSubjectRulesReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	return &wrappedAuthorizationV1InterfaceSelfSubjectRulesReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1InterfaceSelfSubjectRulesReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SelfSubjectRulesReview, arg2 metav1.CreateOptions) (*k8s_io_api_authorization_v1.SelfSubjectRulesReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1Interface) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectRulesReview", metrics.KubeClient)
	return wrapAuthorizationV1InterfaceSelfSubjectRulesReviewInterface(c.inner.SelfSubjectRulesReviews(), recorder)
}

type wrappedAuthorizationV1InterfaceSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1InterfaceSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	return &wrappedAuthorizationV1InterfaceSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1InterfaceSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1.SubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1.SubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1Interface) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1InterfaceSubjectAccessReviewInterface(c.inner.SubjectAccessReviews(), recorder)
}
func (c *wrappedAuthorizationV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAuthorizationV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAuthorizationV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface {
	return &wrappedAuthorizationV1beta1Interface{inner, metrics}
}

type wrappedAuthorizationV1beta1InterfaceLocalSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1beta1InterfaceLocalSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1InterfaceLocalSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1beta1InterfaceLocalSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.LocalSubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.LocalSubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1beta1Interface) LocalSubjectAccessReviews(arg0 string) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.LocalSubjectAccessReviewInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "LocalSubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1beta1InterfaceLocalSubjectAccessReviewInterface(c.inner.LocalSubjectAccessReviews(arg0), recorder)
}

type wrappedAuthorizationV1beta1InterfaceSelfSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1beta1InterfaceSelfSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1InterfaceSelfSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1beta1InterfaceSelfSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectAccessReview, arg2 metav1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1beta1Interface) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1beta1InterfaceSelfSubjectAccessReviewInterface(c.inner.SelfSubjectAccessReviews(), recorder)
}

type wrappedAuthorizationV1beta1InterfaceSelfSubjectRulesReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1beta1InterfaceSelfSubjectRulesReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return &wrappedAuthorizationV1beta1InterfaceSelfSubjectRulesReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1beta1InterfaceSelfSubjectRulesReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1beta1Interface) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectRulesReview", metrics.KubeClient)
	return wrapAuthorizationV1beta1InterfaceSelfSubjectRulesReviewInterface(c.inner.SelfSubjectRulesReviews(), recorder)
}

type wrappedAuthorizationV1beta1InterfaceSubjectAccessReviewInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface
	recorder metrics.Recorder
}

func wrapAuthorizationV1beta1InterfaceSubjectAccessReviewInterface(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface {
	return &wrappedAuthorizationV1beta1InterfaceSubjectAccessReviewInterface{inner, recorder}
}
func (c *wrappedAuthorizationV1beta1InterfaceSubjectAccessReviewInterface) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SubjectAccessReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SubjectAccessReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

func (c *wrappedAuthorizationV1beta1Interface) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SubjectAccessReview", metrics.KubeClient)
	return wrapAuthorizationV1beta1InterfaceSubjectAccessReviewInterface(c.inner.SubjectAccessReviews(), recorder)
}
func (c *wrappedAuthorizationV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAutoscalingV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAutoscalingV1Interface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface {
	return &wrappedAutoscalingV1Interface{inner, metrics}
}

type wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface
	recorder metrics.Recorder
}

func wrapAutoscalingV1InterfaceHorizontalPodAutoscalerInterface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface{inner, recorder}
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscalerList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 metav1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV1InterfaceHorizontalPodAutoscalerInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAutoscalingV1Interface) HorizontalPodAutoscalers(arg0 string) k8s_io_client_go_kubernetes_typed_autoscaling_v1.HorizontalPodAutoscalerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "HorizontalPodAutoscaler", metrics.KubeClient)
	return wrapAutoscalingV1InterfaceHorizontalPodAutoscalerInterface(c.inner.HorizontalPodAutoscalers(arg0), recorder)
}
func (c *wrappedAutoscalingV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAutoscalingV2Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
	metrics metrics.MetricsConfigManager
}

func wrapAutoscalingV2Interface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface {
	return &wrappedAutoscalingV2Interface{inner, metrics}
}

type wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface
	recorder metrics.Recorder
}

func wrapAutoscalingV2InterfaceHorizontalPodAutoscalerInterface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface{inner, recorder}
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscalerList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2InterfaceHorizontalPodAutoscalerInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAutoscalingV2Interface) HorizontalPodAutoscalers(arg0 string) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "HorizontalPodAutoscaler", metrics.KubeClient)
	return wrapAutoscalingV2InterfaceHorizontalPodAutoscalerInterface(c.inner.HorizontalPodAutoscalers(arg0), recorder)
}
func (c *wrappedAutoscalingV2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAutoscalingV2beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapAutoscalingV2beta1Interface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface {
	return &wrappedAutoscalingV2beta1Interface{inner, metrics}
}

type wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface
	recorder metrics.Recorder
}

func wrapAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface{inner, recorder}
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta1.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscalerList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 metav1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta1.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAutoscalingV2beta1Interface) HorizontalPodAutoscalers(arg0 string) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.HorizontalPodAutoscalerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "HorizontalPodAutoscaler", metrics.KubeClient)
	return wrapAutoscalingV2beta1InterfaceHorizontalPodAutoscalerInterface(c.inner.HorizontalPodAutoscalers(arg0), recorder)
}
func (c *wrappedAutoscalingV2beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedAutoscalingV2beta2Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface
	metrics metrics.MetricsConfigManager
}

func wrapAutoscalingV2beta2Interface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface {
	return &wrappedAutoscalingV2beta2Interface{inner, metrics}
}

type wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface
	recorder metrics.Recorder
}

func wrapAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface {
	return &wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface{inner, recorder}
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_autoscaling_v2beta2.HorizontalPodAutoscalerApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Create(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscalerList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Update(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 metav1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v2beta2.HorizontalPodAutoscaler, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedAutoscalingV2beta2Interface) HorizontalPodAutoscalers(arg0 string) k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.HorizontalPodAutoscalerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "HorizontalPodAutoscaler", metrics.KubeClient)
	return wrapAutoscalingV2beta2InterfaceHorizontalPodAutoscalerInterface(c.inner.HorizontalPodAutoscalers(arg0), recorder)
}
func (c *wrappedAutoscalingV2beta2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedBatchV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapBatchV1Interface(inner k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface {
	return &wrappedBatchV1Interface{inner, metrics}
}

type wrappedBatchV1InterfaceCronJobInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface
	recorder metrics.Recorder
}

func wrapBatchV1InterfaceCronJobInterface(inner k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	return &wrappedBatchV1InterfaceCronJobInterface{inner, recorder}
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.CronJobApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.CronJobList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.CronJob, arg2 metav1.UpdateOptions) (*k8s_io_api_batch_v1.CronJob, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceCronJobInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedBatchV1Interface) CronJobs(arg0 string) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CronJob", metrics.KubeClient)
	return wrapBatchV1InterfaceCronJobInterface(c.inner.CronJobs(arg0), recorder)
}

type wrappedBatchV1InterfaceJobInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface
	recorder metrics.Recorder
}

func wrapBatchV1InterfaceJobInterface(inner k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	return &wrappedBatchV1InterfaceJobInterface{inner, recorder}
}
func (c *wrappedBatchV1InterfaceJobInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.JobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1.JobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 metav1.CreateOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_batch_v1.JobList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1InterfaceJobInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1InterfaceJobInterface) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1.Job, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1.Job, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1InterfaceJobInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedBatchV1Interface) Jobs(arg0 string) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Job", metrics.KubeClient)
	return wrapBatchV1InterfaceJobInterface(c.inner.Jobs(arg0), recorder)
}
func (c *wrappedBatchV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedBatchV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapBatchV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return &wrappedBatchV1beta1Interface{inner, metrics}
}

type wrappedBatchV1beta1InterfaceCronJobInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface
	recorder metrics.Recorder
}

func wrapBatchV1beta1InterfaceCronJobInterface(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	return &wrappedBatchV1beta1InterfaceCronJobInterface{inner, recorder}
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1beta1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_batch_v1beta1.CronJobApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Create(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_batch_v1beta1.CronJobList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Update(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 metav1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_batch_v1beta1.CronJob, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_batch_v1beta1.CronJob, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedBatchV1beta1InterfaceCronJobInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedBatchV1beta1Interface) CronJobs(arg0 string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CronJob", metrics.KubeClient)
	return wrapBatchV1beta1InterfaceCronJobInterface(c.inner.CronJobs(arg0), recorder)
}
func (c *wrappedBatchV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedCertificatesV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapCertificatesV1Interface(inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface {
	return &wrappedCertificatesV1Interface{inner, metrics}
}

type wrappedCertificatesV1InterfaceCertificateSigningRequestInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface
	recorder metrics.Recorder
}

func wrapCertificatesV1InterfaceCertificateSigningRequestInterface(inner k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface {
	return &wrappedCertificatesV1InterfaceCertificateSigningRequestInterface{inner, recorder}
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1.CertificateSigningRequestApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 metav1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) UpdateApproval(arg0 context.Context, arg1 string, arg2 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update_approval")
	return c.inner.UpdateApproval(arg0, arg1, arg2, arg3)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1.CertificateSigningRequest, arg2 metav1.UpdateOptions) (*k8s_io_api_certificates_v1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1InterfaceCertificateSigningRequestInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCertificatesV1Interface) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1.CertificateSigningRequestInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CertificateSigningRequest", metrics.KubeClient)
	return wrapCertificatesV1InterfaceCertificateSigningRequestInterface(c.inner.CertificateSigningRequests(), recorder)
}
func (c *wrappedCertificatesV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedCertificatesV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapCertificatesV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return &wrappedCertificatesV1beta1Interface{inner, metrics}
}

type wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface
	recorder metrics.Recorder
}

func wrapCertificatesV1beta1InterfaceCertificateSigningRequestInterface(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	return &wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface{inner, recorder}
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1beta1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_certificates_v1beta1.CertificateSigningRequestApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Create(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Update(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) UpdateApproval(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update_approval")
	return c.inner.UpdateApproval(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_certificates_v1beta1.CertificateSigningRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_certificates_v1beta1.CertificateSigningRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCertificatesV1beta1InterfaceCertificateSigningRequestInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCertificatesV1beta1Interface) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CertificateSigningRequest", metrics.KubeClient)
	return wrapCertificatesV1beta1InterfaceCertificateSigningRequestInterface(c.inner.CertificateSigningRequests(), recorder)
}
func (c *wrappedCertificatesV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedCoordinationV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapCoordinationV1Interface(inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return &wrappedCoordinationV1Interface{inner, metrics}
}

type wrappedCoordinationV1InterfaceLeaseInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface
	recorder metrics.Recorder
}

func wrapCoordinationV1InterfaceLeaseInterface(inner k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	return &wrappedCoordinationV1InterfaceLeaseInterface{inner, recorder}
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_coordination_v1.LeaseApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_coordination_v1.LeaseList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1.Lease, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1.Lease, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1InterfaceLeaseInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoordinationV1Interface) Leases(arg0 string) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Lease", metrics.KubeClient)
	return wrapCoordinationV1InterfaceLeaseInterface(c.inner.Leases(arg0), recorder)
}
func (c *wrappedCoordinationV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedCoordinationV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapCoordinationV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface {
	return &wrappedCoordinationV1beta1Interface{inner, metrics}
}

type wrappedCoordinationV1beta1InterfaceLeaseInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface
	recorder metrics.Recorder
}

func wrapCoordinationV1beta1InterfaceLeaseInterface(inner k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface {
	return &wrappedCoordinationV1beta1InterfaceLeaseInterface{inner, recorder}
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_coordination_v1beta1.LeaseApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Create(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_coordination_v1beta1.LeaseList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Update(arg0 context.Context, arg1 *k8s_io_api_coordination_v1beta1.Lease, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_coordination_v1beta1.Lease, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoordinationV1beta1InterfaceLeaseInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoordinationV1beta1Interface) Leases(arg0 string) k8s_io_client_go_kubernetes_typed_coordination_v1beta1.LeaseInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Lease", metrics.KubeClient)
	return wrapCoordinationV1beta1InterfaceLeaseInterface(c.inner.Leases(arg0), recorder)
}
func (c *wrappedCoordinationV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedCoreV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapCoreV1Interface(inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return &wrappedCoreV1Interface{inner, metrics}
}

type wrappedCoreV1InterfaceComponentStatusInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceComponentStatusInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	return &wrappedCoreV1InterfaceComponentStatusInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ComponentStatusApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ComponentStatusList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ComponentStatus, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ComponentStatus, arg2 metav1.UpdateOptions) (*k8s_io_api_core_v1.ComponentStatus, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceComponentStatusInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) ComponentStatuses() k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ComponentStatus", metrics.KubeClient)
	return wrapCoreV1InterfaceComponentStatusInterface(c.inner.ComponentStatuses(), recorder)
}

type wrappedCoreV1InterfaceConfigMapInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceConfigMapInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	return &wrappedCoreV1InterfaceConfigMapInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ConfigMapApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ConfigMapList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ConfigMap, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ConfigMap, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ConfigMap, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceConfigMapInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) ConfigMaps(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ConfigMap", metrics.KubeClient)
	return wrapCoreV1InterfaceConfigMapInterface(c.inner.ConfigMaps(arg0), recorder)
}

type wrappedCoreV1InterfaceEndpointsInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceEndpointsInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	return &wrappedCoreV1InterfaceEndpointsInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EndpointsApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EndpointsList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Endpoints, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Endpoints, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Endpoints, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEndpointsInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Endpoints(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Endpoints", metrics.KubeClient)
	return wrapCoreV1InterfaceEndpointsInterface(c.inner.Endpoints(arg0), recorder)
}

type wrappedCoreV1InterfaceEventInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.EventInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceEventInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return &wrappedCoreV1InterfaceEventInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceEventInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EventApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("create_with_event_namespace")
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1InterfaceEventInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	defer c.recorder.Record("get_field_selector")
	return c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1InterfaceEventInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceEventInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceEventInterface) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("patch_with_event_namespace")
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceEventInterface) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	defer c.recorder.Record("search")
	return c.inner.Search(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceEventInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 metav1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceEventInterface) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("update_with_event_namespace")
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedCoreV1InterfaceEventInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Events(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Event", metrics.KubeClient)
	return wrapCoreV1InterfaceEventInterface(c.inner.Events(arg0), recorder)
}

type wrappedCoreV1InterfaceLimitRangeInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceLimitRangeInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	return &wrappedCoreV1InterfaceLimitRangeInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.LimitRangeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 metav1.CreateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_core_v1.LimitRangeList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.LimitRange, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.LimitRange, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.LimitRange, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceLimitRangeInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) LimitRanges(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "LimitRange", metrics.KubeClient)
	return wrapCoreV1InterfaceLimitRangeInterface(c.inner.LimitRanges(arg0), recorder)
}

type wrappedCoreV1InterfaceNamespaceInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceNamespaceInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	return &wrappedCoreV1InterfaceNamespaceInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NamespaceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NamespaceApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Finalize(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("finalize")
	return c.inner.Finalize(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.NamespaceList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Namespace, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Namespace, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNamespaceInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Namespaces() k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "Namespace", metrics.KubeClient)
	return wrapCoreV1InterfaceNamespaceInterface(c.inner.Namespaces(), recorder)
}

type wrappedCoreV1InterfaceNodeInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceNodeInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	return &wrappedCoreV1InterfaceNodeInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceNodeInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.NodeApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_core_v1.NodeList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceNodeInterface) PatchStatus(arg0 context.Context, arg1 string, arg2 []uint8) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("patch_status")
	return c.inner.PatchStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Node, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Node, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceNodeInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Nodes() k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "Node", metrics.KubeClient)
	return wrapCoreV1InterfaceNodeInterface(c.inner.Nodes(), recorder)
}

type wrappedCoreV1InterfacePersistentVolumeClaimInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfacePersistentVolumeClaimInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	return &wrappedCoreV1InterfacePersistentVolumeClaimInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeClaimApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeClaimApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeClaimList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolumeClaim, arg2 metav1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolumeClaim, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeClaimInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) PersistentVolumeClaims(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "PersistentVolumeClaim", metrics.KubeClient)
	return wrapCoreV1InterfacePersistentVolumeClaimInterface(c.inner.PersistentVolumeClaims(arg0), recorder)
}

type wrappedCoreV1InterfacePersistentVolumeInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfacePersistentVolumeInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	return &wrappedCoreV1InterfacePersistentVolumeInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PersistentVolumeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PersistentVolumeList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.PersistentVolume, arg2 metav1.UpdateOptions) (*k8s_io_api_core_v1.PersistentVolume, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePersistentVolumeInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) PersistentVolumes() k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PersistentVolume", metrics.KubeClient)
	return wrapCoreV1InterfacePersistentVolumeInterface(c.inner.PersistentVolumes(), recorder)
}

type wrappedCoreV1InterfacePodTemplateInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfacePodTemplateInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	return &wrappedCoreV1InterfacePodTemplateInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodTemplateApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 metav1.CreateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodTemplateList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.PodTemplate, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.PodTemplate, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.PodTemplate, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodTemplateInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) PodTemplates(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "PodTemplate", metrics.KubeClient)
	return wrapCoreV1InterfacePodTemplateInterface(c.inner.PodTemplates(arg0), recorder)
}

type wrappedCoreV1InterfacePodInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.PodInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfacePodInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.PodInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	return &wrappedCoreV1InterfacePodInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfacePodInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.PodApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) Bind(arg0 context.Context, arg1 *k8s_io_api_core_v1.Binding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	defer c.recorder.Record("bind")
	return c.inner.Bind(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	defer c.recorder.Record("evict")
	return c.inner.Evict(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodInterface) EvictV1(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	defer c.recorder.Record("evict_v1")
	return c.inner.EvictV1(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodInterface) EvictV1beta1(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	defer c.recorder.Record("evict_v1beta1")
	return c.inner.EvictV1beta1(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) GetLogs(arg0 string, arg1 *k8s_io_api_core_v1.PodLogOptions) *k8s_io_client_go_rest.Request {
	defer c.recorder.Record("get_logs")
	return c.inner.GetLogs(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.PodList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfacePodInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfacePodInterface) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	defer c.recorder.Record("proxy_get")
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1InterfacePodInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) UpdateEphemeralContainers(arg0 context.Context, arg1 string, arg2 *k8s_io_api_core_v1.Pod, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("update_ephemeral_containers")
	return c.inner.UpdateEphemeralContainers(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1InterfacePodInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Pod, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Pod, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfacePodInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Pods(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Pod", metrics.KubeClient)
	return wrapCoreV1InterfacePodInterface(c.inner.Pods(arg0), recorder)
}

type wrappedCoreV1InterfaceReplicationControllerInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceReplicationControllerInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	return &wrappedCoreV1InterfaceReplicationControllerInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ReplicationControllerApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ReplicationControllerApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ReplicationControllerList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 metav1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ReplicationController, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ReplicationController, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceReplicationControllerInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) ReplicationControllers(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ReplicationController", metrics.KubeClient)
	return wrapCoreV1InterfaceReplicationControllerInterface(c.inner.ReplicationControllers(arg0), recorder)
}

type wrappedCoreV1InterfaceResourceQuotaInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceResourceQuotaInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	return &wrappedCoreV1InterfaceResourceQuotaInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ResourceQuotaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ResourceQuotaApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ResourceQuotaList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.ResourceQuota, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ResourceQuota, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceResourceQuotaInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) ResourceQuotas(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ResourceQuota", metrics.KubeClient)
	return wrapCoreV1InterfaceResourceQuotaInterface(c.inner.ResourceQuotas(arg0), recorder)
}

type wrappedCoreV1InterfaceSecretInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceSecretInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	return &wrappedCoreV1InterfaceSecretInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceSecretInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.SecretApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Secret, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Secret, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Secret, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.SecretList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Secret, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Secret, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Secret, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceSecretInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Secrets(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Secret", metrics.KubeClient)
	return wrapCoreV1InterfaceSecretInterface(c.inner.Secrets(arg0), recorder)
}

type wrappedCoreV1InterfaceServiceAccountInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceServiceAccountInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	return &wrappedCoreV1InterfaceServiceAccountInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceAccountApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) CreateToken(arg0 context.Context, arg1 string, arg2 *k8s_io_api_authentication_v1.TokenRequest, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenRequest, error) {
	defer c.recorder.Record("create_token")
	return c.inner.CreateToken(arg0, arg1, arg2, arg3)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceAccountList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.ServiceAccount, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.ServiceAccount, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.ServiceAccount, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceAccountInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) ServiceAccounts(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ServiceAccount", metrics.KubeClient)
	return wrapCoreV1InterfaceServiceAccountInterface(c.inner.ServiceAccounts(arg0), recorder)
}

type wrappedCoreV1InterfaceServiceInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface
	recorder metrics.Recorder
}

func wrapCoreV1InterfaceServiceInterface(inner k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	return &wrappedCoreV1InterfaceServiceInterface{inner, recorder}
}
func (c *wrappedCoreV1InterfaceServiceInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.ServiceApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.ServiceList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedCoreV1InterfaceServiceInterface) ProxyGet(arg0 string, arg1 string, arg2 string, arg3 string, arg4 map[string]string) k8s_io_client_go_rest.ResponseWrapper {
	defer c.recorder.Record("proxy_get")
	return c.inner.ProxyGet(arg0, arg1, arg2, arg3, arg4)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 metav1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_core_v1.Service, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Service, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedCoreV1InterfaceServiceInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedCoreV1Interface) Services(arg0 string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Service", metrics.KubeClient)
	return wrapCoreV1InterfaceServiceInterface(c.inner.Services(arg0), recorder)
}
func (c *wrappedCoreV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedDiscoveryV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapDiscoveryV1Interface(inner k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface {
	return &wrappedDiscoveryV1Interface{inner, metrics}
}

type wrappedDiscoveryV1InterfaceEndpointSliceInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface
	recorder metrics.Recorder
}

func wrapDiscoveryV1InterfaceEndpointSliceInterface(inner k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface {
	return &wrappedDiscoveryV1InterfaceEndpointSliceInterface{inner, recorder}
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_discovery_v1.EndpointSliceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 metav1.CreateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_discovery_v1.EndpointSliceList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1.EndpointSlice, arg2 metav1.UpdateOptions) (*k8s_io_api_discovery_v1.EndpointSlice, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1InterfaceEndpointSliceInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedDiscoveryV1Interface) EndpointSlices(arg0 string) k8s_io_client_go_kubernetes_typed_discovery_v1.EndpointSliceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "EndpointSlice", metrics.KubeClient)
	return wrapDiscoveryV1InterfaceEndpointSliceInterface(c.inner.EndpointSlices(arg0), recorder)
}
func (c *wrappedDiscoveryV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedDiscoveryV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapDiscoveryV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return &wrappedDiscoveryV1beta1Interface{inner, metrics}
}

type wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface
	recorder metrics.Recorder
}

func wrapDiscoveryV1beta1InterfaceEndpointSliceInterface(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	return &wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface{inner, recorder}
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_discovery_v1beta1.EndpointSliceApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Create(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_discovery_v1beta1.EndpointSliceList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Update(arg0 context.Context, arg1 *k8s_io_api_discovery_v1beta1.EndpointSlice, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_discovery_v1beta1.EndpointSlice, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedDiscoveryV1beta1InterfaceEndpointSliceInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedDiscoveryV1beta1Interface) EndpointSlices(arg0 string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "EndpointSlice", metrics.KubeClient)
	return wrapDiscoveryV1beta1InterfaceEndpointSliceInterface(c.inner.EndpointSlices(arg0), recorder)
}
func (c *wrappedDiscoveryV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedEventsV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapEventsV1Interface(inner k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface {
	return &wrappedEventsV1Interface{inner, metrics}
}

type wrappedEventsV1InterfaceEventInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_events_v1.EventInterface
	recorder metrics.Recorder
}

func wrapEventsV1InterfaceEventInterface(inner k8s_io_client_go_kubernetes_typed_events_v1.EventInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_events_v1.EventInterface {
	return &wrappedEventsV1InterfaceEventInterface{inner, recorder}
}
func (c *wrappedEventsV1InterfaceEventInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_events_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_events_v1.Event, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1.Event, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1.Event, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_events_v1.EventList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1InterfaceEventInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1.Event, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1InterfaceEventInterface) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1.Event, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1InterfaceEventInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedEventsV1Interface) Events(arg0 string) k8s_io_client_go_kubernetes_typed_events_v1.EventInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Event", metrics.KubeClient)
	return wrapEventsV1InterfaceEventInterface(c.inner.Events(arg0), recorder)
}
func (c *wrappedEventsV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedEventsV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapEventsV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return &wrappedEventsV1beta1Interface{inner, metrics}
}

type wrappedEventsV1beta1InterfaceEventInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface
	recorder metrics.Recorder
}

func wrapEventsV1beta1InterfaceEventInterface(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	return &wrappedEventsV1beta1InterfaceEventInterface{inner, recorder}
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_events_v1beta1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Create(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) CreateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("create_with_event_namespace")
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_events_v1beta1.EventList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) PatchWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event, arg1 []uint8) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("patch_with_event_namespace")
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Update(arg0 context.Context, arg1 *k8s_io_api_events_v1beta1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) UpdateWithEventNamespace(arg0 *k8s_io_api_events_v1beta1.Event) (*k8s_io_api_events_v1beta1.Event, error) {
	defer c.recorder.Record("update_with_event_namespace")
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *wrappedEventsV1beta1InterfaceEventInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedEventsV1beta1Interface) Events(arg0 string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Event", metrics.KubeClient)
	return wrapEventsV1beta1InterfaceEventInterface(c.inner.Events(arg0), recorder)
}
func (c *wrappedEventsV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedExtensionsV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapExtensionsV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return &wrappedExtensionsV1beta1Interface{inner, metrics}
}

type wrappedExtensionsV1beta1InterfaceDaemonSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfaceDaemonSetInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	return &wrappedExtensionsV1beta1InterfaceDaemonSetInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DaemonSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DaemonSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_extensions_v1beta1.DaemonSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DaemonSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.DaemonSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDaemonSetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) DaemonSets(arg0 string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "DaemonSet", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfaceDaemonSetInterface(c.inner.DaemonSets(arg0), recorder)
}

type wrappedExtensionsV1beta1InterfaceDeploymentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfaceDeploymentInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return &wrappedExtensionsV1beta1InterfaceDeploymentInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) GetScale(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 metav1.CreateOptions) error {
	defer c.recorder.Record("rollback")
	return c.inner.Rollback(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 metav1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceDeploymentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) Deployments(arg0 string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Deployment", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfaceDeploymentInterface(c.inner.Deployments(arg0), recorder)
}

type wrappedExtensionsV1beta1InterfaceIngressInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfaceIngressInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	return &wrappedExtensionsV1beta1InterfaceIngressInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.IngressApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.IngressList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Ingress, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceIngressInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) Ingresses(arg0 string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Ingress", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfaceIngressInterface(c.inner.Ingresses(arg0), recorder)
}

type wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfaceNetworkPolicyInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	return &wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.NetworkPolicyApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.NetworkPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceNetworkPolicyInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) NetworkPolicies(arg0 string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "NetworkPolicy", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfaceNetworkPolicyInterface(c.inner.NetworkPolicies(arg0), recorder)
}

type wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfacePodSecurityPolicyInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	return &wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.PodSecurityPolicyApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfacePodSecurityPolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PodSecurityPolicy", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfacePodSecurityPolicyInterface(c.inner.PodSecurityPolicies(), recorder)
}

type wrappedExtensionsV1beta1InterfaceReplicaSetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface
	recorder metrics.Recorder
}

func wrapExtensionsV1beta1InterfaceReplicaSetInterface(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	return &wrappedExtensionsV1beta1InterfaceReplicaSetInterface{inner, recorder}
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ReplicaSetApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.ReplicaSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedExtensionsV1beta1InterfaceReplicaSetInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedExtensionsV1beta1Interface) ReplicaSets(arg0 string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "ReplicaSet", metrics.KubeClient)
	return wrapExtensionsV1beta1InterfaceReplicaSetInterface(c.inner.ReplicaSets(arg0), recorder)
}
func (c *wrappedExtensionsV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedFlowcontrolV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapFlowcontrolV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface {
	return &wrappedFlowcontrolV1alpha1Interface{inner, metrics}
}

type wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1alpha1InterfaceFlowSchemaInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	return &wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchemaList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 metav1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.FlowSchema, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfaceFlowSchemaInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1alpha1Interface) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "FlowSchema", metrics.KubeClient)
	return wrapFlowcontrolV1alpha1InterfaceFlowSchemaInterface(c.inner.FlowSchemas(), recorder)
}

type wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1alpha1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 metav1.CreateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1alpha1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1alpha1Interface) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityLevelConfiguration", metrics.KubeClient)
	return wrapFlowcontrolV1alpha1InterfacePriorityLevelConfigurationInterface(c.inner.PriorityLevelConfigurations(), recorder)
}
func (c *wrappedFlowcontrolV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedFlowcontrolV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapFlowcontrolV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return &wrappedFlowcontrolV1beta1Interface{inner, metrics}
}

type wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1beta1InterfaceFlowSchemaInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	return &wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.FlowSchemaApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchemaList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.FlowSchema, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfaceFlowSchemaInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1beta1Interface) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "FlowSchema", metrics.KubeClient)
	return wrapFlowcontrolV1beta1InterfaceFlowSchemaInterface(c.inner.FlowSchemas(), recorder)
}

type wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta1.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta1.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1beta1Interface) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityLevelConfiguration", metrics.KubeClient)
	return wrapFlowcontrolV1beta1InterfacePriorityLevelConfigurationInterface(c.inner.PriorityLevelConfigurations(), recorder)
}
func (c *wrappedFlowcontrolV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedFlowcontrolV1beta2Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface
	metrics metrics.MetricsConfigManager
}

func wrapFlowcontrolV1beta2Interface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface {
	return &wrappedFlowcontrolV1beta2Interface{inner, metrics}
}

type wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1beta2InterfaceFlowSchemaInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface {
	return &wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.FlowSchemaApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.FlowSchemaApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchemaList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 metav1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.FlowSchema, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.FlowSchema, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfaceFlowSchemaInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1beta2Interface) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowSchemaInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "FlowSchema", metrics.KubeClient)
	return wrapFlowcontrolV1beta2InterfaceFlowSchemaInterface(c.inner.FlowSchemas(), recorder)
}

type wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface
	recorder metrics.Recorder
}

func wrapFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface {
	return &wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface{inner, recorder}
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_flowcontrol_v1beta2.PriorityLevelConfigurationApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Create(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfigurationList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Update(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_flowcontrol_v1beta2.PriorityLevelConfiguration, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedFlowcontrolV1beta2Interface) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.PriorityLevelConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityLevelConfiguration", metrics.KubeClient)
	return wrapFlowcontrolV1beta2InterfacePriorityLevelConfigurationInterface(c.inner.PriorityLevelConfigurations(), recorder)
}
func (c *wrappedFlowcontrolV1beta2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedInternalV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapInternalV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return &wrappedInternalV1alpha1Interface{inner, metrics}
}

type wrappedInternalV1alpha1InterfaceStorageVersionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface
	recorder metrics.Recorder
}

func wrapInternalV1alpha1InterfaceStorageVersionInterface(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	return &wrappedInternalV1alpha1InterfaceStorageVersionInterface{inner, recorder}
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1.StorageVersionApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apiserverinternal_v1alpha1.StorageVersionApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Create(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 metav1.CreateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersionList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Update(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apiserverinternal_v1alpha1.StorageVersion, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedInternalV1alpha1InterfaceStorageVersionInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedInternalV1alpha1Interface) StorageVersions() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageVersion", metrics.KubeClient)
	return wrapInternalV1alpha1InterfaceStorageVersionInterface(c.inner.StorageVersions(), recorder)
}
func (c *wrappedInternalV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNetworkingV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNetworkingV1Interface(inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface {
	return &wrappedNetworkingV1Interface{inner, metrics}
}

type wrappedNetworkingV1InterfaceIngressClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1InterfaceIngressClassInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface {
	return &wrappedNetworkingV1InterfaceIngressClassInterface{inner, recorder}
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_networking_v1.IngressClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.IngressClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.IngressClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.IngressClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1Interface) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1.IngressClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IngressClass", metrics.KubeClient)
	return wrapNetworkingV1InterfaceIngressClassInterface(c.inner.IngressClasses(), recorder)
}

type wrappedNetworkingV1InterfaceIngressInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1InterfaceIngressInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface {
	return &wrappedNetworkingV1InterfaceIngressInterface{inner, recorder}
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.IngressApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.IngressList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.Ingress, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceIngressInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1Interface) Ingresses(arg0 string) k8s_io_client_go_kubernetes_typed_networking_v1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Ingress", metrics.KubeClient)
	return wrapNetworkingV1InterfaceIngressInterface(c.inner.Ingresses(arg0), recorder)
}

type wrappedNetworkingV1InterfaceNetworkPolicyInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1InterfaceNetworkPolicyInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface {
	return &wrappedNetworkingV1InterfaceNetworkPolicyInterface{inner, recorder}
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1.NetworkPolicyApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1.NetworkPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1.NetworkPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1.NetworkPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1InterfaceNetworkPolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1Interface) NetworkPolicies(arg0 string) k8s_io_client_go_kubernetes_typed_networking_v1.NetworkPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "NetworkPolicy", metrics.KubeClient)
	return wrapNetworkingV1InterfaceNetworkPolicyInterface(c.inner.NetworkPolicies(arg0), recorder)
}
func (c *wrappedNetworkingV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNetworkingV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNetworkingV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return &wrappedNetworkingV1alpha1Interface{inner, metrics}
}

type wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1alpha1InterfaceClusterCIDRInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	return &wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface{inner, recorder}
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1alpha1.ClusterCIDRApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDRList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1alpha1.ClusterCIDR, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1alpha1.ClusterCIDR, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1alpha1InterfaceClusterCIDRInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1alpha1Interface) ClusterCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCIDR", metrics.KubeClient)
	return wrapNetworkingV1alpha1InterfaceClusterCIDRInterface(c.inner.ClusterCIDRs(), recorder)
}
func (c *wrappedNetworkingV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNetworkingV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNetworkingV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return &wrappedNetworkingV1beta1Interface{inner, metrics}
}

type wrappedNetworkingV1beta1InterfaceIngressClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1beta1InterfaceIngressClassInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return &wrappedNetworkingV1beta1InterfaceIngressClassInterface{inner, recorder}
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 metav1.CreateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.IngressClass, arg2 metav1.UpdateOptions) (*k8s_io_api_networking_v1beta1.IngressClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1beta1Interface) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IngressClass", metrics.KubeClient)
	return wrapNetworkingV1beta1InterfaceIngressClassInterface(c.inner.IngressClasses(), recorder)
}

type wrappedNetworkingV1beta1InterfaceIngressInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface
	recorder metrics.Recorder
}

func wrapNetworkingV1beta1InterfaceIngressInterface(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return &wrappedNetworkingV1beta1InterfaceIngressInterface{inner, recorder}
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_networking_v1beta1.IngressApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Create(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_networking_v1beta1.IngressList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Update(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_networking_v1beta1.Ingress, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_networking_v1beta1.Ingress, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedNetworkingV1beta1InterfaceIngressInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNetworkingV1beta1Interface) Ingresses(arg0 string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Ingress", metrics.KubeClient)
	return wrapNetworkingV1beta1InterfaceIngressInterface(c.inner.Ingresses(arg0), recorder)
}
func (c *wrappedNetworkingV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNodeV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNodeV1Interface(inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return &wrappedNodeV1Interface{inner, metrics}
}

type wrappedNodeV1InterfaceRuntimeClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface
	recorder metrics.Recorder
}

func wrapNodeV1InterfaceRuntimeClassInterface(inner k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	return &wrappedNodeV1InterfaceRuntimeClassInterface{inner, recorder}
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1.RuntimeClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 metav1.CreateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1.RuntimeClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1.RuntimeClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1.RuntimeClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1InterfaceRuntimeClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNodeV1Interface) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "RuntimeClass", metrics.KubeClient)
	return wrapNodeV1InterfaceRuntimeClassInterface(c.inner.RuntimeClasses(), recorder)
}
func (c *wrappedNodeV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNodeV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNodeV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface {
	return &wrappedNodeV1alpha1Interface{inner, metrics}
}

type wrappedNodeV1alpha1InterfaceRuntimeClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface
	recorder metrics.Recorder
}

func wrapNodeV1alpha1InterfaceRuntimeClassInterface(inner k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface {
	return &wrappedNodeV1alpha1InterfaceRuntimeClassInterface{inner, recorder}
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1alpha1.RuntimeClassApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1alpha1.RuntimeClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1alpha1.RuntimeClass, arg2 metav1.UpdateOptions) (*k8s_io_api_node_v1alpha1.RuntimeClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1alpha1InterfaceRuntimeClassInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNodeV1alpha1Interface) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1alpha1.RuntimeClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "RuntimeClass", metrics.KubeClient)
	return wrapNodeV1alpha1InterfaceRuntimeClassInterface(c.inner.RuntimeClasses(), recorder)
}
func (c *wrappedNodeV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedNodeV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapNodeV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface {
	return &wrappedNodeV1beta1Interface{inner, metrics}
}

type wrappedNodeV1beta1InterfaceRuntimeClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface
	recorder metrics.Recorder
}

func wrapNodeV1beta1InterfaceRuntimeClassInterface(inner k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface {
	return &wrappedNodeV1beta1InterfaceRuntimeClassInterface{inner, recorder}
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_node_v1beta1.RuntimeClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_node_v1beta1.RuntimeClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_node_v1beta1.RuntimeClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_node_v1beta1.RuntimeClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedNodeV1beta1InterfaceRuntimeClassInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedNodeV1beta1Interface) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1beta1.RuntimeClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "RuntimeClass", metrics.KubeClient)
	return wrapNodeV1beta1InterfaceRuntimeClassInterface(c.inner.RuntimeClasses(), recorder)
}
func (c *wrappedNodeV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedPolicyV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapPolicyV1Interface(inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return &wrappedPolicyV1Interface{inner, metrics}
}

type wrappedPolicyV1InterfaceEvictionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface
	recorder metrics.Recorder
}

func wrapPolicyV1InterfaceEvictionInterface(inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return &wrappedPolicyV1InterfaceEvictionInterface{inner, recorder}
}
func (c *wrappedPolicyV1InterfaceEvictionInterface) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	defer c.recorder.Record("evict")
	return c.inner.Evict(arg0, arg1)
}

func (c *wrappedPolicyV1Interface) Evictions(arg0 string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Eviction", metrics.KubeClient)
	return wrapPolicyV1InterfaceEvictionInterface(c.inner.Evictions(arg0), recorder)
}

type wrappedPolicyV1InterfacePodDisruptionBudgetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface
	recorder metrics.Recorder
}

func wrapPolicyV1InterfacePodDisruptionBudgetInterface(inner k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	return &wrappedPolicyV1InterfacePodDisruptionBudgetInterface{inner, recorder}
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1.PodDisruptionBudgetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1.PodDisruptionBudget, arg2 metav1.UpdateOptions) (*k8s_io_api_policy_v1.PodDisruptionBudget, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1InterfacePodDisruptionBudgetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedPolicyV1Interface) PodDisruptionBudgets(arg0 string) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "PodDisruptionBudget", metrics.KubeClient)
	return wrapPolicyV1InterfacePodDisruptionBudgetInterface(c.inner.PodDisruptionBudgets(arg0), recorder)
}
func (c *wrappedPolicyV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedPolicyV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapPolicyV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return &wrappedPolicyV1beta1Interface{inner, metrics}
}

type wrappedPolicyV1beta1InterfaceEvictionInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface
	recorder metrics.Recorder
}

func wrapPolicyV1beta1InterfaceEvictionInterface(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return &wrappedPolicyV1beta1InterfaceEvictionInterface{inner, recorder}
}
func (c *wrappedPolicyV1beta1InterfaceEvictionInterface) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	defer c.recorder.Record("evict")
	return c.inner.Evict(arg0, arg1)
}

func (c *wrappedPolicyV1beta1Interface) Evictions(arg0 string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Eviction", metrics.KubeClient)
	return wrapPolicyV1beta1InterfaceEvictionInterface(c.inner.Evictions(arg0), recorder)
}

type wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface
	recorder metrics.Recorder
}

func wrapPolicyV1beta1InterfacePodDisruptionBudgetInterface(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	return &wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface{inner, recorder}
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodDisruptionBudgetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudgetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodDisruptionBudget, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodDisruptionBudget, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodDisruptionBudgetInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedPolicyV1beta1Interface) PodDisruptionBudgets(arg0 string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "PodDisruptionBudget", metrics.KubeClient)
	return wrapPolicyV1beta1InterfacePodDisruptionBudgetInterface(c.inner.PodDisruptionBudgets(arg0), recorder)
}

type wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface
	recorder metrics.Recorder
}

func wrapPolicyV1beta1InterfacePodSecurityPolicyInterface(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	return &wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface{inner, recorder}
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_policy_v1beta1.PodSecurityPolicyApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Create(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 metav1.CreateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Update(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.PodSecurityPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_policy_v1beta1.PodSecurityPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedPolicyV1beta1InterfacePodSecurityPolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedPolicyV1beta1Interface) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PodSecurityPolicy", metrics.KubeClient)
	return wrapPolicyV1beta1InterfacePodSecurityPolicyInterface(c.inner.PodSecurityPolicies(), recorder)
}
func (c *wrappedPolicyV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedRbacV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapRbacV1Interface(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface {
	return &wrappedRbacV1Interface{inner, metrics}
}

type wrappedRbacV1InterfaceClusterRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1InterfaceClusterRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	return &wrappedRbacV1InterfaceClusterRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleBindingInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1Interface) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRoleBinding", metrics.KubeClient)
	return wrapRbacV1InterfaceClusterRoleBindingInterface(c.inner.ClusterRoleBindings(), recorder)
}

type wrappedRbacV1InterfaceClusterRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1InterfaceClusterRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	return &wrappedRbacV1InterfaceClusterRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 metav1.CreateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.ClusterRoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.ClusterRole, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceClusterRoleInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1Interface) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRole", metrics.KubeClient)
	return wrapRbacV1InterfaceClusterRoleInterface(c.inner.ClusterRoles(), recorder)
}

type wrappedRbacV1InterfaceRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1InterfaceRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	return &wrappedRbacV1InterfaceRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.RoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1.RoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.RoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleBindingInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1Interface) RoleBindings(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "RoleBinding", metrics.KubeClient)
	return wrapRbacV1InterfaceRoleBindingInterface(c.inner.RoleBindings(arg0), recorder)
}

type wrappedRbacV1InterfaceRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1InterfaceRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	return &wrappedRbacV1InterfaceRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1InterfaceRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1.RoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1.Role, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_rbac_v1.Role, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_rbac_v1.RoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1.Role, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1.Role, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1InterfaceRoleInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1Interface) Roles(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Role", metrics.KubeClient)
	return wrapRbacV1InterfaceRoleInterface(c.inner.Roles(arg0), recorder)
}
func (c *wrappedRbacV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedRbacV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapRbacV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface {
	return &wrappedRbacV1alpha1Interface{inner, metrics}
}

type wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1alpha1InterfaceClusterRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface {
	return &wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleBindingInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1alpha1Interface) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRoleBinding", metrics.KubeClient)
	return wrapRbacV1alpha1InterfaceClusterRoleBindingInterface(c.inner.ClusterRoleBindings(), recorder)
}

type wrappedRbacV1alpha1InterfaceClusterRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1alpha1InterfaceClusterRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface {
	return &wrappedRbacV1alpha1InterfaceClusterRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 metav1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.ClusterRole, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceClusterRoleInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1alpha1Interface) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.ClusterRoleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRole", metrics.KubeClient)
	return wrapRbacV1alpha1InterfaceClusterRoleInterface(c.inner.ClusterRoles(), recorder)
}

type wrappedRbacV1alpha1InterfaceRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1alpha1InterfaceRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface {
	return &wrappedRbacV1alpha1InterfaceRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.RoleBindingApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.RoleBinding, arg2 metav1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.RoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleBindingInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1alpha1Interface) RoleBindings(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleBindingInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "RoleBinding", metrics.KubeClient)
	return wrapRbacV1alpha1InterfaceRoleBindingInterface(c.inner.RoleBindings(arg0), recorder)
}

type wrappedRbacV1alpha1InterfaceRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1alpha1InterfaceRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface {
	return &wrappedRbacV1alpha1InterfaceRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1alpha1.RoleApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1alpha1.RoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1alpha1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1alpha1.Role, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1alpha1InterfaceRoleInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1alpha1Interface) Roles(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RoleInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Role", metrics.KubeClient)
	return wrapRbacV1alpha1InterfaceRoleInterface(c.inner.Roles(arg0), recorder)
}
func (c *wrappedRbacV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedRbacV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapRbacV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface {
	return &wrappedRbacV1beta1Interface{inner, metrics}
}

type wrappedRbacV1beta1InterfaceClusterRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1beta1InterfaceClusterRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface {
	return &wrappedRbacV1beta1InterfaceClusterRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.ClusterRoleBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRoleBinding, arg2 metav1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleBindingInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1beta1Interface) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRoleBinding", metrics.KubeClient)
	return wrapRbacV1beta1InterfaceClusterRoleBindingInterface(c.inner.ClusterRoleBindings(), recorder)
}

type wrappedRbacV1beta1InterfaceClusterRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1beta1InterfaceClusterRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface {
	return &wrappedRbacV1beta1InterfaceClusterRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.ClusterRoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 metav1.CreateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.ClusterRoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.ClusterRole, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.ClusterRole, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceClusterRoleInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1beta1Interface) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1beta1.ClusterRoleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRole", metrics.KubeClient)
	return wrapRbacV1beta1InterfaceClusterRoleInterface(c.inner.ClusterRoles(), recorder)
}

type wrappedRbacV1beta1InterfaceRoleBindingInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface
	recorder metrics.Recorder
}

func wrapRbacV1beta1InterfaceRoleBindingInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface {
	return &wrappedRbacV1beta1InterfaceRoleBindingInterface{inner, recorder}
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.RoleBindingApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 metav1.CreateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleBindingList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.RoleBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.RoleBinding, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleBindingInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1beta1Interface) RoleBindings(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleBindingInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "RoleBinding", metrics.KubeClient)
	return wrapRbacV1beta1InterfaceRoleBindingInterface(c.inner.RoleBindings(arg0), recorder)
}

type wrappedRbacV1beta1InterfaceRoleInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface
	recorder metrics.Recorder
}

func wrapRbacV1beta1InterfaceRoleInterface(inner k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface {
	return &wrappedRbacV1beta1InterfaceRoleInterface{inner, recorder}
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_rbac_v1beta1.RoleApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Create(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_rbac_v1beta1.RoleList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_rbac_v1beta1.Role, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Update(arg0 context.Context, arg1 *k8s_io_api_rbac_v1beta1.Role, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_rbac_v1beta1.Role, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedRbacV1beta1InterfaceRoleInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedRbacV1beta1Interface) Roles(arg0 string) k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RoleInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Role", metrics.KubeClient)
	return wrapRbacV1beta1InterfaceRoleInterface(c.inner.Roles(arg0), recorder)
}
func (c *wrappedRbacV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedSchedulingV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapSchedulingV1Interface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface {
	return &wrappedSchedulingV1Interface{inner, metrics}
}

type wrappedSchedulingV1InterfacePriorityClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface
	recorder metrics.Recorder
}

func wrapSchedulingV1InterfacePriorityClassInterface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface {
	return &wrappedSchedulingV1InterfacePriorityClassInterface{inner, recorder}
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1.PriorityClassApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1.PriorityClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1.PriorityClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1InterfacePriorityClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedSchedulingV1Interface) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1.PriorityClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityClass", metrics.KubeClient)
	return wrapSchedulingV1InterfacePriorityClassInterface(c.inner.PriorityClasses(), recorder)
}
func (c *wrappedSchedulingV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedSchedulingV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapSchedulingV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface {
	return &wrappedSchedulingV1alpha1Interface{inner, metrics}
}

type wrappedSchedulingV1alpha1InterfacePriorityClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface
	recorder metrics.Recorder
}

func wrapSchedulingV1alpha1InterfacePriorityClassInterface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	return &wrappedSchedulingV1alpha1InterfacePriorityClassInterface{inner, recorder}
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1alpha1.PriorityClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1alpha1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1alpha1.PriorityClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1alpha1InterfacePriorityClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedSchedulingV1alpha1Interface) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityClass", metrics.KubeClient)
	return wrapSchedulingV1alpha1InterfacePriorityClassInterface(c.inner.PriorityClasses(), recorder)
}
func (c *wrappedSchedulingV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedSchedulingV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapSchedulingV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return &wrappedSchedulingV1beta1Interface{inner, metrics}
}

type wrappedSchedulingV1beta1InterfacePriorityClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface
	recorder metrics.Recorder
}

func wrapSchedulingV1beta1InterfacePriorityClassInterface(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	return &wrappedSchedulingV1beta1InterfacePriorityClassInterface{inner, recorder}
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_scheduling_v1beta1.PriorityClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_scheduling_v1beta1.PriorityClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_scheduling_v1beta1.PriorityClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedSchedulingV1beta1InterfacePriorityClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedSchedulingV1beta1Interface) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityClass", metrics.KubeClient)
	return wrapSchedulingV1beta1InterfacePriorityClassInterface(c.inner.PriorityClasses(), recorder)
}
func (c *wrappedSchedulingV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedStorageV1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface
	metrics metrics.MetricsConfigManager
}

func wrapStorageV1Interface(inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface {
	return &wrappedStorageV1Interface{inner, metrics}
}

type wrappedStorageV1InterfaceCSIDriverInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface
	recorder metrics.Recorder
}

func wrapStorageV1InterfaceCSIDriverInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface {
	return &wrappedStorageV1InterfaceCSIDriverInterface{inner, recorder}
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSIDriverApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIDriverList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIDriver, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIDriver, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1.CSIDriver, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIDriverInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1Interface) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1.CSIDriverInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSIDriver", metrics.KubeClient)
	return wrapStorageV1InterfaceCSIDriverInterface(c.inner.CSIDrivers(), recorder)
}

type wrappedStorageV1InterfaceCSINodeInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface
	recorder metrics.Recorder
}

func wrapStorageV1InterfaceCSINodeInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface {
	return &wrappedStorageV1InterfaceCSINodeInterface{inner, recorder}
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSINodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_storage_v1.CSINodeList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSINode, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSINode, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.CSINode, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSINodeInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1Interface) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1.CSINodeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSINode", metrics.KubeClient)
	return wrapStorageV1InterfaceCSINodeInterface(c.inner.CSINodes(), recorder)
}

type wrappedStorageV1InterfaceCSIStorageCapacityInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface
	recorder metrics.Recorder
}

func wrapStorageV1InterfaceCSIStorageCapacityInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface {
	return &wrappedStorageV1InterfaceCSIStorageCapacityInterface{inner, recorder}
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.CSIStorageCapacityApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.CSIStorageCapacityList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.CSIStorageCapacity, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1.CSIStorageCapacity, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceCSIStorageCapacityInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1Interface) CSIStorageCapacities(arg0 string) k8s_io_client_go_kubernetes_typed_storage_v1.CSIStorageCapacityInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CSIStorageCapacity", metrics.KubeClient)
	return wrapStorageV1InterfaceCSIStorageCapacityInterface(c.inner.CSIStorageCapacities(arg0), recorder)
}

type wrappedStorageV1InterfaceStorageClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface
	recorder metrics.Recorder
}

func wrapStorageV1InterfaceStorageClassInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface {
	return &wrappedStorageV1InterfaceStorageClassInterface{inner, recorder}
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.StorageClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 metav1.CreateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_storage_v1.StorageClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.StorageClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.StorageClass, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1.StorageClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceStorageClassInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1Interface) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1.StorageClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageClass", metrics.KubeClient)
	return wrapStorageV1InterfaceStorageClassInterface(c.inner.StorageClasses(), recorder)
}

type wrappedStorageV1InterfaceVolumeAttachmentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface
	recorder metrics.Recorder
}

func wrapStorageV1InterfaceVolumeAttachmentInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface {
	return &wrappedStorageV1InterfaceVolumeAttachmentInterface{inner, recorder}
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1.VolumeAttachmentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1.VolumeAttachment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1InterfaceVolumeAttachmentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1Interface) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1.VolumeAttachmentInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "VolumeAttachment", metrics.KubeClient)
	return wrapStorageV1InterfaceVolumeAttachmentInterface(c.inner.VolumeAttachments(), recorder)
}
func (c *wrappedStorageV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedStorageV1alpha1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func wrapStorageV1alpha1Interface(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return &wrappedStorageV1alpha1Interface{inner, metrics}
}

type wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface
	recorder metrics.Recorder
}

func wrapStorageV1alpha1InterfaceCSIStorageCapacityInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	return &wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface{inner, recorder}
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.CSIStorageCapacityApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) DeleteCollection(arg0 context.Context, arg1 metav1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacityList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.CSIStorageCapacity, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceCSIStorageCapacityInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1alpha1Interface) CSIStorageCapacities(arg0 string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CSIStorageCapacity", metrics.KubeClient)
	return wrapStorageV1alpha1InterfaceCSIStorageCapacityInterface(c.inner.CSIStorageCapacities(arg0), recorder)
}

type wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface
	recorder metrics.Recorder
}

func wrapStorageV1alpha1InterfaceVolumeAttachmentInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	return &wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface{inner, recorder}
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1alpha1.VolumeAttachmentApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachmentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1alpha1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1alpha1.VolumeAttachment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1alpha1InterfaceVolumeAttachmentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1alpha1Interface) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "VolumeAttachment", metrics.KubeClient)
	return wrapStorageV1alpha1InterfaceVolumeAttachmentInterface(c.inner.VolumeAttachments(), recorder)
}
func (c *wrappedStorageV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedStorageV1beta1Interface struct {
	inner   k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func wrapStorageV1beta1Interface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface, metrics metrics.MetricsConfigManager) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return &wrappedStorageV1beta1Interface{inner, metrics}
}

type wrappedStorageV1beta1InterfaceCSIDriverInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface
	recorder metrics.Recorder
}

func wrapStorageV1beta1InterfaceCSIDriverInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	return &wrappedStorageV1beta1InterfaceCSIDriverInterface{inner, recorder}
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSIDriverApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIDriverList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIDriver, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIDriver, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIDriverInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1beta1Interface) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSIDriver", metrics.KubeClient)
	return wrapStorageV1beta1InterfaceCSIDriverInterface(c.inner.CSIDrivers(), recorder)
}

type wrappedStorageV1beta1InterfaceCSINodeInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface
	recorder metrics.Recorder
}

func wrapStorageV1beta1InterfaceCSINodeInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	return &wrappedStorageV1beta1InterfaceCSINodeInterface{inner, recorder}
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSINodeApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 metav1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) List(arg0 context.Context, arg1 metav1.ListOptions) (*k8s_io_api_storage_v1beta1.CSINodeList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSINode, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSINode, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSINodeInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1beta1Interface) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSINode", metrics.KubeClient)
	return wrapStorageV1beta1InterfaceCSINodeInterface(c.inner.CSINodes(), recorder)
}

type wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface
	recorder metrics.Recorder
}

func wrapStorageV1beta1InterfaceCSIStorageCapacityInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	return &wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface{inner, recorder}
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.CSIStorageCapacityApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Delete(arg0 context.Context, arg1 string, arg2 metav1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacityList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.CSIStorageCapacity, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1beta1.CSIStorageCapacity, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceCSIStorageCapacityInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1beta1Interface) CSIStorageCapacities(arg0 string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CSIStorageCapacity", metrics.KubeClient)
	return wrapStorageV1beta1InterfaceCSIStorageCapacityInterface(c.inner.CSIStorageCapacities(arg0), recorder)
}

type wrappedStorageV1beta1InterfaceStorageClassInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface
	recorder metrics.Recorder
}

func wrapStorageV1beta1InterfaceStorageClassInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	return &wrappedStorageV1beta1InterfaceStorageClassInterface{inner, recorder}
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.StorageClassApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.StorageClassList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.StorageClass, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.StorageClass, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceStorageClassInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1beta1Interface) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageClass", metrics.KubeClient)
	return wrapStorageV1beta1InterfaceStorageClassInterface(c.inner.StorageClasses(), recorder)
}

type wrappedStorageV1beta1InterfaceVolumeAttachmentInterface struct {
	inner    k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface
	recorder metrics.Recorder
}

func wrapStorageV1beta1InterfaceVolumeAttachmentInterface(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	return &wrappedStorageV1beta1InterfaceVolumeAttachmentInterface{inner, recorder}
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.VolumeAttachmentApplyConfiguration, arg2 metav1.ApplyOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_storage_v1beta1.VolumeAttachmentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Create(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachmentList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Update(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 metav1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_storage_v1beta1.VolumeAttachment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_storage_v1beta1.VolumeAttachment, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedStorageV1beta1InterfaceVolumeAttachmentInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedStorageV1beta1Interface) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "VolumeAttachment", metrics.KubeClient)
	return wrapStorageV1beta1InterfaceVolumeAttachmentInterface(c.inner.VolumeAttachments(), recorder)
}
func (c *wrappedStorageV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}
