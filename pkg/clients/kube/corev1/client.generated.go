package client

import (
	"github.com/go-logr/logr"
	componentstatuses "github.com/kyverno/kyverno/pkg/clients/kube/corev1/componentstatuses"
	configmaps "github.com/kyverno/kyverno/pkg/clients/kube/corev1/configmaps"
	endpoints "github.com/kyverno/kyverno/pkg/clients/kube/corev1/endpoints"
	events "github.com/kyverno/kyverno/pkg/clients/kube/corev1/events"
	limitranges "github.com/kyverno/kyverno/pkg/clients/kube/corev1/limitranges"
	namespaces "github.com/kyverno/kyverno/pkg/clients/kube/corev1/namespaces"
	nodes "github.com/kyverno/kyverno/pkg/clients/kube/corev1/nodes"
	persistentvolumeclaims "github.com/kyverno/kyverno/pkg/clients/kube/corev1/persistentvolumeclaims"
	persistentvolumes "github.com/kyverno/kyverno/pkg/clients/kube/corev1/persistentvolumes"
	pods "github.com/kyverno/kyverno/pkg/clients/kube/corev1/pods"
	podtemplates "github.com/kyverno/kyverno/pkg/clients/kube/corev1/podtemplates"
	replicationcontrollers "github.com/kyverno/kyverno/pkg/clients/kube/corev1/replicationcontrollers"
	resourcequotas "github.com/kyverno/kyverno/pkg/clients/kube/corev1/resourcequotas"
	secrets "github.com/kyverno/kyverno/pkg/clients/kube/corev1/secrets"
	serviceaccounts "github.com/kyverno/kyverno/pkg/clients/kube/corev1/serviceaccounts"
	services "github.com/kyverno/kyverno/pkg/clients/kube/corev1/services"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface, client string) k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ComponentStatuses() k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ComponentStatus", c.clientType)
	return componentstatuses.WithMetrics(c.inner.ComponentStatuses(), recorder)
}
func (c *withMetrics) ConfigMaps(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ConfigMap", c.clientType)
	return configmaps.WithMetrics(c.inner.ConfigMaps(namespace), recorder)
}
func (c *withMetrics) Endpoints(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Endpoints", c.clientType)
	return endpoints.WithMetrics(c.inner.Endpoints(namespace), recorder)
}
func (c *withMetrics) Events(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Event", c.clientType)
	return events.WithMetrics(c.inner.Events(namespace), recorder)
}
func (c *withMetrics) LimitRanges(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "LimitRange", c.clientType)
	return limitranges.WithMetrics(c.inner.LimitRanges(namespace), recorder)
}
func (c *withMetrics) Namespaces() k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "Namespace", c.clientType)
	return namespaces.WithMetrics(c.inner.Namespaces(), recorder)
}
func (c *withMetrics) Nodes() k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "Node", c.clientType)
	return nodes.WithMetrics(c.inner.Nodes(), recorder)
}
func (c *withMetrics) PersistentVolumeClaims(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PersistentVolumeClaim", c.clientType)
	return persistentvolumeclaims.WithMetrics(c.inner.PersistentVolumeClaims(namespace), recorder)
}
func (c *withMetrics) PersistentVolumes() k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PersistentVolume", c.clientType)
	return persistentvolumes.WithMetrics(c.inner.PersistentVolumes(), recorder)
}
func (c *withMetrics) PodTemplates(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodTemplate", c.clientType)
	return podtemplates.WithMetrics(c.inner.PodTemplates(namespace), recorder)
}
func (c *withMetrics) Pods(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Pod", c.clientType)
	return pods.WithMetrics(c.inner.Pods(namespace), recorder)
}
func (c *withMetrics) ReplicationControllers(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ReplicationController", c.clientType)
	return replicationcontrollers.WithMetrics(c.inner.ReplicationControllers(namespace), recorder)
}
func (c *withMetrics) ResourceQuotas(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ResourceQuota", c.clientType)
	return resourcequotas.WithMetrics(c.inner.ResourceQuotas(namespace), recorder)
}
func (c *withMetrics) Secrets(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Secret", c.clientType)
	return secrets.WithMetrics(c.inner.Secrets(namespace), recorder)
}
func (c *withMetrics) ServiceAccounts(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ServiceAccount", c.clientType)
	return serviceaccounts.WithMetrics(c.inner.ServiceAccounts(namespace), recorder)
}
func (c *withMetrics) Services(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Service", c.clientType)
	return services.WithMetrics(c.inner.Services(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ComponentStatuses() k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	return componentstatuses.WithTracing(c.inner.ComponentStatuses(), c.client, "ComponentStatus")
}
func (c *withTracing) ConfigMaps(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	return configmaps.WithTracing(c.inner.ConfigMaps(namespace), c.client, "ConfigMap")
}
func (c *withTracing) Endpoints(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	return endpoints.WithTracing(c.inner.Endpoints(namespace), c.client, "Endpoints")
}
func (c *withTracing) Events(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return events.WithTracing(c.inner.Events(namespace), c.client, "Event")
}
func (c *withTracing) LimitRanges(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	return limitranges.WithTracing(c.inner.LimitRanges(namespace), c.client, "LimitRange")
}
func (c *withTracing) Namespaces() k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	return namespaces.WithTracing(c.inner.Namespaces(), c.client, "Namespace")
}
func (c *withTracing) Nodes() k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	return nodes.WithTracing(c.inner.Nodes(), c.client, "Node")
}
func (c *withTracing) PersistentVolumeClaims(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	return persistentvolumeclaims.WithTracing(c.inner.PersistentVolumeClaims(namespace), c.client, "PersistentVolumeClaim")
}
func (c *withTracing) PersistentVolumes() k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	return persistentvolumes.WithTracing(c.inner.PersistentVolumes(), c.client, "PersistentVolume")
}
func (c *withTracing) PodTemplates(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	return podtemplates.WithTracing(c.inner.PodTemplates(namespace), c.client, "PodTemplate")
}
func (c *withTracing) Pods(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	return pods.WithTracing(c.inner.Pods(namespace), c.client, "Pod")
}
func (c *withTracing) ReplicationControllers(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	return replicationcontrollers.WithTracing(c.inner.ReplicationControllers(namespace), c.client, "ReplicationController")
}
func (c *withTracing) ResourceQuotas(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	return resourcequotas.WithTracing(c.inner.ResourceQuotas(namespace), c.client, "ResourceQuota")
}
func (c *withTracing) Secrets(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	return secrets.WithTracing(c.inner.Secrets(namespace), c.client, "Secret")
}
func (c *withTracing) ServiceAccounts(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	return serviceaccounts.WithTracing(c.inner.ServiceAccounts(namespace), c.client, "ServiceAccount")
}
func (c *withTracing) Services(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	return services.WithTracing(c.inner.Services(namespace), c.client, "Service")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_core_v1.CoreV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ComponentStatuses() k8s_io_client_go_kubernetes_typed_core_v1.ComponentStatusInterface {
	return componentstatuses.WithLogging(c.inner.ComponentStatuses(), c.logger.WithValues("resource", "ComponentStatuses"))
}
func (c *withLogging) ConfigMaps(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ConfigMapInterface {
	return configmaps.WithLogging(c.inner.ConfigMaps(namespace), c.logger.WithValues("resource", "ConfigMaps").WithValues("namespace", namespace))
}
func (c *withLogging) Endpoints(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EndpointsInterface {
	return endpoints.WithLogging(c.inner.Endpoints(namespace), c.logger.WithValues("resource", "Endpoints").WithValues("namespace", namespace))
}
func (c *withLogging) Events(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return events.WithLogging(c.inner.Events(namespace), c.logger.WithValues("resource", "Events").WithValues("namespace", namespace))
}
func (c *withLogging) LimitRanges(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.LimitRangeInterface {
	return limitranges.WithLogging(c.inner.LimitRanges(namespace), c.logger.WithValues("resource", "LimitRanges").WithValues("namespace", namespace))
}
func (c *withLogging) Namespaces() k8s_io_client_go_kubernetes_typed_core_v1.NamespaceInterface {
	return namespaces.WithLogging(c.inner.Namespaces(), c.logger.WithValues("resource", "Namespaces"))
}
func (c *withLogging) Nodes() k8s_io_client_go_kubernetes_typed_core_v1.NodeInterface {
	return nodes.WithLogging(c.inner.Nodes(), c.logger.WithValues("resource", "Nodes"))
}
func (c *withLogging) PersistentVolumeClaims(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeClaimInterface {
	return persistentvolumeclaims.WithLogging(c.inner.PersistentVolumeClaims(namespace), c.logger.WithValues("resource", "PersistentVolumeClaims").WithValues("namespace", namespace))
}
func (c *withLogging) PersistentVolumes() k8s_io_client_go_kubernetes_typed_core_v1.PersistentVolumeInterface {
	return persistentvolumes.WithLogging(c.inner.PersistentVolumes(), c.logger.WithValues("resource", "PersistentVolumes"))
}
func (c *withLogging) PodTemplates(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodTemplateInterface {
	return podtemplates.WithLogging(c.inner.PodTemplates(namespace), c.logger.WithValues("resource", "PodTemplates").WithValues("namespace", namespace))
}
func (c *withLogging) Pods(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.PodInterface {
	return pods.WithLogging(c.inner.Pods(namespace), c.logger.WithValues("resource", "Pods").WithValues("namespace", namespace))
}
func (c *withLogging) ReplicationControllers(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ReplicationControllerInterface {
	return replicationcontrollers.WithLogging(c.inner.ReplicationControllers(namespace), c.logger.WithValues("resource", "ReplicationControllers").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceQuotas(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ResourceQuotaInterface {
	return resourcequotas.WithLogging(c.inner.ResourceQuotas(namespace), c.logger.WithValues("resource", "ResourceQuotas").WithValues("namespace", namespace))
}
func (c *withLogging) Secrets(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.SecretInterface {
	return secrets.WithLogging(c.inner.Secrets(namespace), c.logger.WithValues("resource", "Secrets").WithValues("namespace", namespace))
}
func (c *withLogging) ServiceAccounts(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceAccountInterface {
	return serviceaccounts.WithLogging(c.inner.ServiceAccounts(namespace), c.logger.WithValues("resource", "ServiceAccounts").WithValues("namespace", namespace))
}
func (c *withLogging) Services(namespace string) k8s_io_client_go_kubernetes_typed_core_v1.ServiceInterface {
	return services.WithLogging(c.inner.Services(namespace), c.logger.WithValues("resource", "Services").WithValues("namespace", namespace))
}
