package clientset

import (
	"github.com/go-logr/logr"
	admissionregistrationv1 "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1"
	admissionregistrationv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1alpha1"
	admissionregistrationv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1beta1"
	appsv1 "github.com/kyverno/kyverno/pkg/clients/kube/appsv1"
	appsv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/appsv1beta1"
	appsv1beta2 "github.com/kyverno/kyverno/pkg/clients/kube/appsv1beta2"
	authenticationv1 "github.com/kyverno/kyverno/pkg/clients/kube/authenticationv1"
	authenticationv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/authenticationv1alpha1"
	authenticationv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/authenticationv1beta1"
	authorizationv1 "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1"
	authorizationv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1beta1"
	autoscalingv1 "github.com/kyverno/kyverno/pkg/clients/kube/autoscalingv1"
	autoscalingv2 "github.com/kyverno/kyverno/pkg/clients/kube/autoscalingv2"
	autoscalingv2beta1 "github.com/kyverno/kyverno/pkg/clients/kube/autoscalingv2beta1"
	autoscalingv2beta2 "github.com/kyverno/kyverno/pkg/clients/kube/autoscalingv2beta2"
	batchv1 "github.com/kyverno/kyverno/pkg/clients/kube/batchv1"
	batchv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/batchv1beta1"
	certificatesv1 "github.com/kyverno/kyverno/pkg/clients/kube/certificatesv1"
	certificatesv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/certificatesv1alpha1"
	certificatesv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/certificatesv1beta1"
	coordinationv1 "github.com/kyverno/kyverno/pkg/clients/kube/coordinationv1"
	coordinationv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/coordinationv1alpha1"
	coordinationv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/coordinationv1beta1"
	corev1 "github.com/kyverno/kyverno/pkg/clients/kube/corev1"
	discovery "github.com/kyverno/kyverno/pkg/clients/kube/discovery"
	discoveryv1 "github.com/kyverno/kyverno/pkg/clients/kube/discoveryv1"
	discoveryv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/discoveryv1beta1"
	eventsv1 "github.com/kyverno/kyverno/pkg/clients/kube/eventsv1"
	eventsv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/eventsv1beta1"
	extensionsv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1"
	flowcontrolv1 "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1"
	flowcontrolv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1beta1"
	flowcontrolv1beta2 "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1beta2"
	flowcontrolv1beta3 "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1beta3"
	internalv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/internalv1alpha1"
	networkingv1 "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1"
	networkingv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1alpha1"
	networkingv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1"
	nodev1 "github.com/kyverno/kyverno/pkg/clients/kube/nodev1"
	nodev1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/nodev1alpha1"
	nodev1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/nodev1beta1"
	policyv1 "github.com/kyverno/kyverno/pkg/clients/kube/policyv1"
	policyv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/policyv1beta1"
	rbacv1 "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1"
	rbacv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1alpha1"
	rbacv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1beta1"
	resourcev1alpha3 "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3"
	schedulingv1 "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1"
	schedulingv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha1"
	schedulingv1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1beta1"
	storagemigrationv1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/storagemigrationv1alpha1"
	storagev1 "github.com/kyverno/kyverno/pkg/clients/kube/storagev1"
	storagev1alpha1 "github.com/kyverno/kyverno/pkg/clients/kube/storagev1alpha1"
	storagev1beta1 "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_kubernetes "k8s.io/client-go/kubernetes"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1alpha1"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1 "k8s.io/client-go/kubernetes/typed/apiserverinternal/v1alpha1"
	k8s_io_client_go_kubernetes_typed_apps_v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	k8s_io_client_go_kubernetes_typed_apps_v1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	k8s_io_client_go_kubernetes_typed_apps_v1beta2 "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	k8s_io_client_go_kubernetes_typed_authentication_v1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	k8s_io_client_go_kubernetes_typed_authentication_v1alpha1 "k8s.io/client-go/kubernetes/typed/authentication/v1alpha1"
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
	k8s_io_client_go_kubernetes_typed_certificates_v1alpha1 "k8s.io/client-go/kubernetes/typed/certificates/v1alpha1"
	k8s_io_client_go_kubernetes_typed_certificates_v1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	k8s_io_client_go_kubernetes_typed_coordination_v1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	k8s_io_client_go_kubernetes_typed_coordination_v1alpha1 "k8s.io/client-go/kubernetes/typed/coordination/v1alpha1"
	k8s_io_client_go_kubernetes_typed_coordination_v1beta1 "k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	k8s_io_client_go_kubernetes_typed_core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8s_io_client_go_kubernetes_typed_discovery_v1 "k8s.io/client-go/kubernetes/typed/discovery/v1"
	k8s_io_client_go_kubernetes_typed_discovery_v1beta1 "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	k8s_io_client_go_kubernetes_typed_events_v1 "k8s.io/client-go/kubernetes/typed/events/v1"
	k8s_io_client_go_kubernetes_typed_events_v1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	k8s_io_client_go_kubernetes_typed_extensions_v1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta2"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta3 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta3"
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
	k8s_io_client_go_kubernetes_typed_resource_v1alpha3 "k8s.io/client-go/kubernetes/typed/resource/v1alpha3"
	k8s_io_client_go_kubernetes_typed_scheduling_v1 "k8s.io/client-go/kubernetes/typed/scheduling/v1"
	k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	k8s_io_client_go_kubernetes_typed_scheduling_v1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	k8s_io_client_go_kubernetes_typed_storage_v1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	k8s_io_client_go_kubernetes_typed_storage_v1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	k8s_io_client_go_kubernetes_typed_storage_v1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1 "k8s.io/client-go/kubernetes/typed/storagemigration/v1alpha1"
)

type clientset struct {
	discovery                     k8s_io_client_go_discovery.DiscoveryInterface
	admissionregistrationv1       k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	admissionregistrationv1alpha1 k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface
	admissionregistrationv1beta1  k8s_io_client_go_kubernetes_typed_admissionregistration_v1beta1.AdmissionregistrationV1beta1Interface
	appsv1                        k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
	appsv1beta1                   k8s_io_client_go_kubernetes_typed_apps_v1beta1.AppsV1beta1Interface
	appsv1beta2                   k8s_io_client_go_kubernetes_typed_apps_v1beta2.AppsV1beta2Interface
	authenticationv1              k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	authenticationv1alpha1        k8s_io_client_go_kubernetes_typed_authentication_v1alpha1.AuthenticationV1alpha1Interface
	authenticationv1beta1         k8s_io_client_go_kubernetes_typed_authentication_v1beta1.AuthenticationV1beta1Interface
	authorizationv1               k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	authorizationv1beta1          k8s_io_client_go_kubernetes_typed_authorization_v1beta1.AuthorizationV1beta1Interface
	autoscalingv1                 k8s_io_client_go_kubernetes_typed_autoscaling_v1.AutoscalingV1Interface
	autoscalingv2                 k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
	autoscalingv2beta1            k8s_io_client_go_kubernetes_typed_autoscaling_v2beta1.AutoscalingV2beta1Interface
	autoscalingv2beta2            k8s_io_client_go_kubernetes_typed_autoscaling_v2beta2.AutoscalingV2beta2Interface
	batchv1                       k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
	batchv1beta1                  k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	certificatesv1                k8s_io_client_go_kubernetes_typed_certificates_v1.CertificatesV1Interface
	certificatesv1alpha1          k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface
	certificatesv1beta1           k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	coordinationv1                k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	coordinationv1alpha1          k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface
	coordinationv1beta1           k8s_io_client_go_kubernetes_typed_coordination_v1beta1.CoordinationV1beta1Interface
	corev1                        k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	discoveryv1                   k8s_io_client_go_kubernetes_typed_discovery_v1.DiscoveryV1Interface
	discoveryv1beta1              k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	eventsv1                      k8s_io_client_go_kubernetes_typed_events_v1.EventsV1Interface
	eventsv1beta1                 k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	extensionsv1beta1             k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	flowcontrolv1                 k8s_io_client_go_kubernetes_typed_flowcontrol_v1.FlowcontrolV1Interface
	flowcontrolv1beta1            k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	flowcontrolv1beta2            k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface
	flowcontrolv1beta3            k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta3.FlowcontrolV1beta3Interface
	internalv1alpha1              k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	networkingv1                  k8s_io_client_go_kubernetes_typed_networking_v1.NetworkingV1Interface
	networkingv1alpha1            k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	networkingv1beta1             k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	nodev1                        k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	nodev1alpha1                  k8s_io_client_go_kubernetes_typed_node_v1alpha1.NodeV1alpha1Interface
	nodev1beta1                   k8s_io_client_go_kubernetes_typed_node_v1beta1.NodeV1beta1Interface
	policyv1                      k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	policyv1beta1                 k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	rbacv1                        k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
	rbacv1alpha1                  k8s_io_client_go_kubernetes_typed_rbac_v1alpha1.RbacV1alpha1Interface
	rbacv1beta1                   k8s_io_client_go_kubernetes_typed_rbac_v1beta1.RbacV1beta1Interface
	resourcev1alpha3              k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	schedulingv1                  k8s_io_client_go_kubernetes_typed_scheduling_v1.SchedulingV1Interface
	schedulingv1alpha1            k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
	schedulingv1beta1             k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	storagev1                     k8s_io_client_go_kubernetes_typed_storage_v1.StorageV1Interface
	storagev1alpha1               k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	storagev1beta1                k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
	storagemigrationv1alpha1      k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface
}

func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.discovery
}
func (c *clientset) AdmissionregistrationV1() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return c.admissionregistrationv1
}
func (c *clientset) AdmissionregistrationV1alpha1() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface {
	return c.admissionregistrationv1alpha1
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
func (c *clientset) AuthenticationV1alpha1() k8s_io_client_go_kubernetes_typed_authentication_v1alpha1.AuthenticationV1alpha1Interface {
	return c.authenticationv1alpha1
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
func (c *clientset) CertificatesV1alpha1() k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface {
	return c.certificatesv1alpha1
}
func (c *clientset) CertificatesV1beta1() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return c.certificatesv1beta1
}
func (c *clientset) CoordinationV1() k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return c.coordinationv1
}
func (c *clientset) CoordinationV1alpha1() k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface {
	return c.coordinationv1alpha1
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
func (c *clientset) FlowcontrolV1() k8s_io_client_go_kubernetes_typed_flowcontrol_v1.FlowcontrolV1Interface {
	return c.flowcontrolv1
}
func (c *clientset) FlowcontrolV1beta1() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return c.flowcontrolv1beta1
}
func (c *clientset) FlowcontrolV1beta2() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta2.FlowcontrolV1beta2Interface {
	return c.flowcontrolv1beta2
}
func (c *clientset) FlowcontrolV1beta3() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta3.FlowcontrolV1beta3Interface {
	return c.flowcontrolv1beta3
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
func (c *clientset) ResourceV1alpha3() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return c.resourcev1alpha3
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
func (c *clientset) StoragemigrationV1alpha1() k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface {
	return c.storagemigrationv1alpha1
}

func WrapWithMetrics(inner k8s_io_client_go_kubernetes.Interface, m metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes.Interface {
	return &clientset{
		discovery:                     discovery.WithMetrics(inner.Discovery(), metrics.ClusteredClientQueryRecorder(m, "Discovery", clientType)),
		admissionregistrationv1:       admissionregistrationv1.WithMetrics(inner.AdmissionregistrationV1(), m, clientType),
		admissionregistrationv1alpha1: admissionregistrationv1alpha1.WithMetrics(inner.AdmissionregistrationV1alpha1(), m, clientType),
		admissionregistrationv1beta1:  admissionregistrationv1beta1.WithMetrics(inner.AdmissionregistrationV1beta1(), m, clientType),
		appsv1:                        appsv1.WithMetrics(inner.AppsV1(), m, clientType),
		appsv1beta1:                   appsv1beta1.WithMetrics(inner.AppsV1beta1(), m, clientType),
		appsv1beta2:                   appsv1beta2.WithMetrics(inner.AppsV1beta2(), m, clientType),
		authenticationv1:              authenticationv1.WithMetrics(inner.AuthenticationV1(), m, clientType),
		authenticationv1alpha1:        authenticationv1alpha1.WithMetrics(inner.AuthenticationV1alpha1(), m, clientType),
		authenticationv1beta1:         authenticationv1beta1.WithMetrics(inner.AuthenticationV1beta1(), m, clientType),
		authorizationv1:               authorizationv1.WithMetrics(inner.AuthorizationV1(), m, clientType),
		authorizationv1beta1:          authorizationv1beta1.WithMetrics(inner.AuthorizationV1beta1(), m, clientType),
		autoscalingv1:                 autoscalingv1.WithMetrics(inner.AutoscalingV1(), m, clientType),
		autoscalingv2:                 autoscalingv2.WithMetrics(inner.AutoscalingV2(), m, clientType),
		autoscalingv2beta1:            autoscalingv2beta1.WithMetrics(inner.AutoscalingV2beta1(), m, clientType),
		autoscalingv2beta2:            autoscalingv2beta2.WithMetrics(inner.AutoscalingV2beta2(), m, clientType),
		batchv1:                       batchv1.WithMetrics(inner.BatchV1(), m, clientType),
		batchv1beta1:                  batchv1beta1.WithMetrics(inner.BatchV1beta1(), m, clientType),
		certificatesv1:                certificatesv1.WithMetrics(inner.CertificatesV1(), m, clientType),
		certificatesv1alpha1:          certificatesv1alpha1.WithMetrics(inner.CertificatesV1alpha1(), m, clientType),
		certificatesv1beta1:           certificatesv1beta1.WithMetrics(inner.CertificatesV1beta1(), m, clientType),
		coordinationv1:                coordinationv1.WithMetrics(inner.CoordinationV1(), m, clientType),
		coordinationv1alpha1:          coordinationv1alpha1.WithMetrics(inner.CoordinationV1alpha1(), m, clientType),
		coordinationv1beta1:           coordinationv1beta1.WithMetrics(inner.CoordinationV1beta1(), m, clientType),
		corev1:                        corev1.WithMetrics(inner.CoreV1(), m, clientType),
		discoveryv1:                   discoveryv1.WithMetrics(inner.DiscoveryV1(), m, clientType),
		discoveryv1beta1:              discoveryv1beta1.WithMetrics(inner.DiscoveryV1beta1(), m, clientType),
		eventsv1:                      eventsv1.WithMetrics(inner.EventsV1(), m, clientType),
		eventsv1beta1:                 eventsv1beta1.WithMetrics(inner.EventsV1beta1(), m, clientType),
		extensionsv1beta1:             extensionsv1beta1.WithMetrics(inner.ExtensionsV1beta1(), m, clientType),
		flowcontrolv1:                 flowcontrolv1.WithMetrics(inner.FlowcontrolV1(), m, clientType),
		flowcontrolv1beta1:            flowcontrolv1beta1.WithMetrics(inner.FlowcontrolV1beta1(), m, clientType),
		flowcontrolv1beta2:            flowcontrolv1beta2.WithMetrics(inner.FlowcontrolV1beta2(), m, clientType),
		flowcontrolv1beta3:            flowcontrolv1beta3.WithMetrics(inner.FlowcontrolV1beta3(), m, clientType),
		internalv1alpha1:              internalv1alpha1.WithMetrics(inner.InternalV1alpha1(), m, clientType),
		networkingv1:                  networkingv1.WithMetrics(inner.NetworkingV1(), m, clientType),
		networkingv1alpha1:            networkingv1alpha1.WithMetrics(inner.NetworkingV1alpha1(), m, clientType),
		networkingv1beta1:             networkingv1beta1.WithMetrics(inner.NetworkingV1beta1(), m, clientType),
		nodev1:                        nodev1.WithMetrics(inner.NodeV1(), m, clientType),
		nodev1alpha1:                  nodev1alpha1.WithMetrics(inner.NodeV1alpha1(), m, clientType),
		nodev1beta1:                   nodev1beta1.WithMetrics(inner.NodeV1beta1(), m, clientType),
		policyv1:                      policyv1.WithMetrics(inner.PolicyV1(), m, clientType),
		policyv1beta1:                 policyv1beta1.WithMetrics(inner.PolicyV1beta1(), m, clientType),
		rbacv1:                        rbacv1.WithMetrics(inner.RbacV1(), m, clientType),
		rbacv1alpha1:                  rbacv1alpha1.WithMetrics(inner.RbacV1alpha1(), m, clientType),
		rbacv1beta1:                   rbacv1beta1.WithMetrics(inner.RbacV1beta1(), m, clientType),
		resourcev1alpha3:              resourcev1alpha3.WithMetrics(inner.ResourceV1alpha3(), m, clientType),
		schedulingv1:                  schedulingv1.WithMetrics(inner.SchedulingV1(), m, clientType),
		schedulingv1alpha1:            schedulingv1alpha1.WithMetrics(inner.SchedulingV1alpha1(), m, clientType),
		schedulingv1beta1:             schedulingv1beta1.WithMetrics(inner.SchedulingV1beta1(), m, clientType),
		storagev1:                     storagev1.WithMetrics(inner.StorageV1(), m, clientType),
		storagev1alpha1:               storagev1alpha1.WithMetrics(inner.StorageV1alpha1(), m, clientType),
		storagev1beta1:                storagev1beta1.WithMetrics(inner.StorageV1beta1(), m, clientType),
		storagemigrationv1alpha1:      storagemigrationv1alpha1.WithMetrics(inner.StoragemigrationV1alpha1(), m, clientType),
	}
}

func WrapWithTracing(inner k8s_io_client_go_kubernetes.Interface) k8s_io_client_go_kubernetes.Interface {
	return &clientset{
		discovery:                     discovery.WithTracing(inner.Discovery(), "Discovery", ""),
		admissionregistrationv1:       admissionregistrationv1.WithTracing(inner.AdmissionregistrationV1(), "AdmissionregistrationV1"),
		admissionregistrationv1alpha1: admissionregistrationv1alpha1.WithTracing(inner.AdmissionregistrationV1alpha1(), "AdmissionregistrationV1alpha1"),
		admissionregistrationv1beta1:  admissionregistrationv1beta1.WithTracing(inner.AdmissionregistrationV1beta1(), "AdmissionregistrationV1beta1"),
		appsv1:                        appsv1.WithTracing(inner.AppsV1(), "AppsV1"),
		appsv1beta1:                   appsv1beta1.WithTracing(inner.AppsV1beta1(), "AppsV1beta1"),
		appsv1beta2:                   appsv1beta2.WithTracing(inner.AppsV1beta2(), "AppsV1beta2"),
		authenticationv1:              authenticationv1.WithTracing(inner.AuthenticationV1(), "AuthenticationV1"),
		authenticationv1alpha1:        authenticationv1alpha1.WithTracing(inner.AuthenticationV1alpha1(), "AuthenticationV1alpha1"),
		authenticationv1beta1:         authenticationv1beta1.WithTracing(inner.AuthenticationV1beta1(), "AuthenticationV1beta1"),
		authorizationv1:               authorizationv1.WithTracing(inner.AuthorizationV1(), "AuthorizationV1"),
		authorizationv1beta1:          authorizationv1beta1.WithTracing(inner.AuthorizationV1beta1(), "AuthorizationV1beta1"),
		autoscalingv1:                 autoscalingv1.WithTracing(inner.AutoscalingV1(), "AutoscalingV1"),
		autoscalingv2:                 autoscalingv2.WithTracing(inner.AutoscalingV2(), "AutoscalingV2"),
		autoscalingv2beta1:            autoscalingv2beta1.WithTracing(inner.AutoscalingV2beta1(), "AutoscalingV2beta1"),
		autoscalingv2beta2:            autoscalingv2beta2.WithTracing(inner.AutoscalingV2beta2(), "AutoscalingV2beta2"),
		batchv1:                       batchv1.WithTracing(inner.BatchV1(), "BatchV1"),
		batchv1beta1:                  batchv1beta1.WithTracing(inner.BatchV1beta1(), "BatchV1beta1"),
		certificatesv1:                certificatesv1.WithTracing(inner.CertificatesV1(), "CertificatesV1"),
		certificatesv1alpha1:          certificatesv1alpha1.WithTracing(inner.CertificatesV1alpha1(), "CertificatesV1alpha1"),
		certificatesv1beta1:           certificatesv1beta1.WithTracing(inner.CertificatesV1beta1(), "CertificatesV1beta1"),
		coordinationv1:                coordinationv1.WithTracing(inner.CoordinationV1(), "CoordinationV1"),
		coordinationv1alpha1:          coordinationv1alpha1.WithTracing(inner.CoordinationV1alpha1(), "CoordinationV1alpha1"),
		coordinationv1beta1:           coordinationv1beta1.WithTracing(inner.CoordinationV1beta1(), "CoordinationV1beta1"),
		corev1:                        corev1.WithTracing(inner.CoreV1(), "CoreV1"),
		discoveryv1:                   discoveryv1.WithTracing(inner.DiscoveryV1(), "DiscoveryV1"),
		discoveryv1beta1:              discoveryv1beta1.WithTracing(inner.DiscoveryV1beta1(), "DiscoveryV1beta1"),
		eventsv1:                      eventsv1.WithTracing(inner.EventsV1(), "EventsV1"),
		eventsv1beta1:                 eventsv1beta1.WithTracing(inner.EventsV1beta1(), "EventsV1beta1"),
		extensionsv1beta1:             extensionsv1beta1.WithTracing(inner.ExtensionsV1beta1(), "ExtensionsV1beta1"),
		flowcontrolv1:                 flowcontrolv1.WithTracing(inner.FlowcontrolV1(), "FlowcontrolV1"),
		flowcontrolv1beta1:            flowcontrolv1beta1.WithTracing(inner.FlowcontrolV1beta1(), "FlowcontrolV1beta1"),
		flowcontrolv1beta2:            flowcontrolv1beta2.WithTracing(inner.FlowcontrolV1beta2(), "FlowcontrolV1beta2"),
		flowcontrolv1beta3:            flowcontrolv1beta3.WithTracing(inner.FlowcontrolV1beta3(), "FlowcontrolV1beta3"),
		internalv1alpha1:              internalv1alpha1.WithTracing(inner.InternalV1alpha1(), "InternalV1alpha1"),
		networkingv1:                  networkingv1.WithTracing(inner.NetworkingV1(), "NetworkingV1"),
		networkingv1alpha1:            networkingv1alpha1.WithTracing(inner.NetworkingV1alpha1(), "NetworkingV1alpha1"),
		networkingv1beta1:             networkingv1beta1.WithTracing(inner.NetworkingV1beta1(), "NetworkingV1beta1"),
		nodev1:                        nodev1.WithTracing(inner.NodeV1(), "NodeV1"),
		nodev1alpha1:                  nodev1alpha1.WithTracing(inner.NodeV1alpha1(), "NodeV1alpha1"),
		nodev1beta1:                   nodev1beta1.WithTracing(inner.NodeV1beta1(), "NodeV1beta1"),
		policyv1:                      policyv1.WithTracing(inner.PolicyV1(), "PolicyV1"),
		policyv1beta1:                 policyv1beta1.WithTracing(inner.PolicyV1beta1(), "PolicyV1beta1"),
		rbacv1:                        rbacv1.WithTracing(inner.RbacV1(), "RbacV1"),
		rbacv1alpha1:                  rbacv1alpha1.WithTracing(inner.RbacV1alpha1(), "RbacV1alpha1"),
		rbacv1beta1:                   rbacv1beta1.WithTracing(inner.RbacV1beta1(), "RbacV1beta1"),
		resourcev1alpha3:              resourcev1alpha3.WithTracing(inner.ResourceV1alpha3(), "ResourceV1alpha3"),
		schedulingv1:                  schedulingv1.WithTracing(inner.SchedulingV1(), "SchedulingV1"),
		schedulingv1alpha1:            schedulingv1alpha1.WithTracing(inner.SchedulingV1alpha1(), "SchedulingV1alpha1"),
		schedulingv1beta1:             schedulingv1beta1.WithTracing(inner.SchedulingV1beta1(), "SchedulingV1beta1"),
		storagev1:                     storagev1.WithTracing(inner.StorageV1(), "StorageV1"),
		storagev1alpha1:               storagev1alpha1.WithTracing(inner.StorageV1alpha1(), "StorageV1alpha1"),
		storagev1beta1:                storagev1beta1.WithTracing(inner.StorageV1beta1(), "StorageV1beta1"),
		storagemigrationv1alpha1:      storagemigrationv1alpha1.WithTracing(inner.StoragemigrationV1alpha1(), "StoragemigrationV1alpha1"),
	}
}

func WrapWithLogging(inner k8s_io_client_go_kubernetes.Interface, logger logr.Logger) k8s_io_client_go_kubernetes.Interface {
	return &clientset{
		discovery:                     discovery.WithLogging(inner.Discovery(), logger.WithValues("group", "Discovery")),
		admissionregistrationv1:       admissionregistrationv1.WithLogging(inner.AdmissionregistrationV1(), logger.WithValues("group", "AdmissionregistrationV1")),
		admissionregistrationv1alpha1: admissionregistrationv1alpha1.WithLogging(inner.AdmissionregistrationV1alpha1(), logger.WithValues("group", "AdmissionregistrationV1alpha1")),
		admissionregistrationv1beta1:  admissionregistrationv1beta1.WithLogging(inner.AdmissionregistrationV1beta1(), logger.WithValues("group", "AdmissionregistrationV1beta1")),
		appsv1:                        appsv1.WithLogging(inner.AppsV1(), logger.WithValues("group", "AppsV1")),
		appsv1beta1:                   appsv1beta1.WithLogging(inner.AppsV1beta1(), logger.WithValues("group", "AppsV1beta1")),
		appsv1beta2:                   appsv1beta2.WithLogging(inner.AppsV1beta2(), logger.WithValues("group", "AppsV1beta2")),
		authenticationv1:              authenticationv1.WithLogging(inner.AuthenticationV1(), logger.WithValues("group", "AuthenticationV1")),
		authenticationv1alpha1:        authenticationv1alpha1.WithLogging(inner.AuthenticationV1alpha1(), logger.WithValues("group", "AuthenticationV1alpha1")),
		authenticationv1beta1:         authenticationv1beta1.WithLogging(inner.AuthenticationV1beta1(), logger.WithValues("group", "AuthenticationV1beta1")),
		authorizationv1:               authorizationv1.WithLogging(inner.AuthorizationV1(), logger.WithValues("group", "AuthorizationV1")),
		authorizationv1beta1:          authorizationv1beta1.WithLogging(inner.AuthorizationV1beta1(), logger.WithValues("group", "AuthorizationV1beta1")),
		autoscalingv1:                 autoscalingv1.WithLogging(inner.AutoscalingV1(), logger.WithValues("group", "AutoscalingV1")),
		autoscalingv2:                 autoscalingv2.WithLogging(inner.AutoscalingV2(), logger.WithValues("group", "AutoscalingV2")),
		autoscalingv2beta1:            autoscalingv2beta1.WithLogging(inner.AutoscalingV2beta1(), logger.WithValues("group", "AutoscalingV2beta1")),
		autoscalingv2beta2:            autoscalingv2beta2.WithLogging(inner.AutoscalingV2beta2(), logger.WithValues("group", "AutoscalingV2beta2")),
		batchv1:                       batchv1.WithLogging(inner.BatchV1(), logger.WithValues("group", "BatchV1")),
		batchv1beta1:                  batchv1beta1.WithLogging(inner.BatchV1beta1(), logger.WithValues("group", "BatchV1beta1")),
		certificatesv1:                certificatesv1.WithLogging(inner.CertificatesV1(), logger.WithValues("group", "CertificatesV1")),
		certificatesv1alpha1:          certificatesv1alpha1.WithLogging(inner.CertificatesV1alpha1(), logger.WithValues("group", "CertificatesV1alpha1")),
		certificatesv1beta1:           certificatesv1beta1.WithLogging(inner.CertificatesV1beta1(), logger.WithValues("group", "CertificatesV1beta1")),
		coordinationv1:                coordinationv1.WithLogging(inner.CoordinationV1(), logger.WithValues("group", "CoordinationV1")),
		coordinationv1alpha1:          coordinationv1alpha1.WithLogging(inner.CoordinationV1alpha1(), logger.WithValues("group", "CoordinationV1alpha1")),
		coordinationv1beta1:           coordinationv1beta1.WithLogging(inner.CoordinationV1beta1(), logger.WithValues("group", "CoordinationV1beta1")),
		corev1:                        corev1.WithLogging(inner.CoreV1(), logger.WithValues("group", "CoreV1")),
		discoveryv1:                   discoveryv1.WithLogging(inner.DiscoveryV1(), logger.WithValues("group", "DiscoveryV1")),
		discoveryv1beta1:              discoveryv1beta1.WithLogging(inner.DiscoveryV1beta1(), logger.WithValues("group", "DiscoveryV1beta1")),
		eventsv1:                      eventsv1.WithLogging(inner.EventsV1(), logger.WithValues("group", "EventsV1")),
		eventsv1beta1:                 eventsv1beta1.WithLogging(inner.EventsV1beta1(), logger.WithValues("group", "EventsV1beta1")),
		extensionsv1beta1:             extensionsv1beta1.WithLogging(inner.ExtensionsV1beta1(), logger.WithValues("group", "ExtensionsV1beta1")),
		flowcontrolv1:                 flowcontrolv1.WithLogging(inner.FlowcontrolV1(), logger.WithValues("group", "FlowcontrolV1")),
		flowcontrolv1beta1:            flowcontrolv1beta1.WithLogging(inner.FlowcontrolV1beta1(), logger.WithValues("group", "FlowcontrolV1beta1")),
		flowcontrolv1beta2:            flowcontrolv1beta2.WithLogging(inner.FlowcontrolV1beta2(), logger.WithValues("group", "FlowcontrolV1beta2")),
		flowcontrolv1beta3:            flowcontrolv1beta3.WithLogging(inner.FlowcontrolV1beta3(), logger.WithValues("group", "FlowcontrolV1beta3")),
		internalv1alpha1:              internalv1alpha1.WithLogging(inner.InternalV1alpha1(), logger.WithValues("group", "InternalV1alpha1")),
		networkingv1:                  networkingv1.WithLogging(inner.NetworkingV1(), logger.WithValues("group", "NetworkingV1")),
		networkingv1alpha1:            networkingv1alpha1.WithLogging(inner.NetworkingV1alpha1(), logger.WithValues("group", "NetworkingV1alpha1")),
		networkingv1beta1:             networkingv1beta1.WithLogging(inner.NetworkingV1beta1(), logger.WithValues("group", "NetworkingV1beta1")),
		nodev1:                        nodev1.WithLogging(inner.NodeV1(), logger.WithValues("group", "NodeV1")),
		nodev1alpha1:                  nodev1alpha1.WithLogging(inner.NodeV1alpha1(), logger.WithValues("group", "NodeV1alpha1")),
		nodev1beta1:                   nodev1beta1.WithLogging(inner.NodeV1beta1(), logger.WithValues("group", "NodeV1beta1")),
		policyv1:                      policyv1.WithLogging(inner.PolicyV1(), logger.WithValues("group", "PolicyV1")),
		policyv1beta1:                 policyv1beta1.WithLogging(inner.PolicyV1beta1(), logger.WithValues("group", "PolicyV1beta1")),
		rbacv1:                        rbacv1.WithLogging(inner.RbacV1(), logger.WithValues("group", "RbacV1")),
		rbacv1alpha1:                  rbacv1alpha1.WithLogging(inner.RbacV1alpha1(), logger.WithValues("group", "RbacV1alpha1")),
		rbacv1beta1:                   rbacv1beta1.WithLogging(inner.RbacV1beta1(), logger.WithValues("group", "RbacV1beta1")),
		resourcev1alpha3:              resourcev1alpha3.WithLogging(inner.ResourceV1alpha3(), logger.WithValues("group", "ResourceV1alpha3")),
		schedulingv1:                  schedulingv1.WithLogging(inner.SchedulingV1(), logger.WithValues("group", "SchedulingV1")),
		schedulingv1alpha1:            schedulingv1alpha1.WithLogging(inner.SchedulingV1alpha1(), logger.WithValues("group", "SchedulingV1alpha1")),
		schedulingv1beta1:             schedulingv1beta1.WithLogging(inner.SchedulingV1beta1(), logger.WithValues("group", "SchedulingV1beta1")),
		storagev1:                     storagev1.WithLogging(inner.StorageV1(), logger.WithValues("group", "StorageV1")),
		storagev1alpha1:               storagev1alpha1.WithLogging(inner.StorageV1alpha1(), logger.WithValues("group", "StorageV1alpha1")),
		storagev1beta1:                storagev1beta1.WithLogging(inner.StorageV1beta1(), logger.WithValues("group", "StorageV1beta1")),
		storagemigrationv1alpha1:      storagemigrationv1alpha1.WithLogging(inner.StoragemigrationV1alpha1(), logger.WithValues("group", "StoragemigrationV1alpha1")),
	}
}
