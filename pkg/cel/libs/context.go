package libs

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/cel/libs/generator"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Context interface {
	globalcontext.ContextInterface
	imagedata.ContextInterface
	resource.ContextInterface
	generator.ContextInterface

	GetGeneratedResources() []*unstructured.Unstructured
	ClearGeneratedResources()
	SetPolicyName(name string)
	SetTriggerMetadata(name, namespace, uid, apiVersion, group, kind string)
}

type contextProvider struct {
	logger             logr.Logger
	client             dclient.Interface
	imagedata          imagedataloader.Fetcher
	gctxStore          gctxstore.Store
	generatedResources []*unstructured.Unstructured
	policyName         string
	triggerName        string
	triggerNamespace   string
	triggerAPIVersion  string
	triggerGroup       string
	triggerKind        string
	triggerUID         string
}

func NewContextProvider(
	logger logr.Logger,
	client dclient.Interface,
	imageOpts []imagedataloader.Option,
	gctxStore gctxstore.Store,
) (Context, error) {
	idl, err := imagedataloader.New(client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace()), imageOpts...)
	if err != nil {
		return nil, err
	}
	return &contextProvider{
		logger:             logger,
		client:             client,
		imagedata:          idl,
		gctxStore:          gctxStore,
		generatedResources: make([]*unstructured.Unstructured, 0),
	}, nil
}

func (cp *contextProvider) GetGlobalReference(name, projection, jmesPath string) (any, error) {
	ent, ok := cp.gctxStore.Get(name)
	if !ok {
		err := errors.New("global context entry not found")
		cp.logger.Error(err, "GetGlobalReference", "name", name, "projection", projection, "jmesPath", jmesPath)
		return nil, err
	}
	data, err := ent.Get(projection, jmesPath)
	if err != nil {
		cp.logger.Error(err, "GetGlobalReference", "name", name, "projection", projection, "jmesPath", jmesPath)
		return nil, err
	}
	cp.logger.V(4).Info("GetGlobalReference", "name", name, "projection", projection, "jmesPath", jmesPath, "data", data)
	if isLikelyKubernetesObject(data) {
		out, err := kubeutils.ObjToUnstructured(data)
		if err != nil {
			cp.logger.Error(err, "GetGlobalReference", "name", name, "projection", projection, "jmesPath", jmesPath)
			return nil, err
		}
		if out != nil {
			return *out, nil
		} else {
			return nil, errors.New("failed to convert to Unstructured")
		}
	} else {
		return data, nil
	}
}

func (cp *contextProvider) GetImageData(image string) (map[string]any, error) {
	// TODO: get image credentials from image verification policies?
	data, err := cp.imagedata.FetchImageData(context.TODO(), image)
	if err != nil {
		return nil, err
	}
	return utils.GetValue(data.Data())
}

func (cp *contextProvider) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.List(context.TODO(), metav1.ListOptions{})
}

func (cp *contextProvider) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Get(context.TODO(), name, metav1.GetOptions{})
}

func (cp *contextProvider) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Create(context.TODO(), &unstructured.Unstructured{Object: data}, metav1.CreateOptions{})
}

func (cp *contextProvider) GenerateResources(namespace string, dataList []map[string]any) error {
	for _, data := range dataList {
		resource := &unstructured.Unstructured{Object: data}
		if resource.IsList() {
			resourceList, err := resource.ToList()
			if err != nil {
				return err
			}
			for i := range resourceList.Items {
				item := &resourceList.Items[i]
				labels := item.GetLabels()
				if labels == nil {
					labels = make(map[string]string, 9)
				}
				labels[kyverno.LabelAppManagedBy] = kyverno.ValueKyvernoApp
				labels[common.GenerateSourceUIDLabel] = string(item.GetUID())
				labels[common.GeneratePolicyLabel] = cp.policyName
				// add trigger metadata labels
				labels[common.GenerateTriggerNameLabel] = cp.triggerName
				labels[common.GenerateTriggerNSLabel] = cp.triggerNamespace
				labels[common.GenerateTriggerUIDLabel] = cp.triggerUID
				labels[common.GenerateTriggerKindLabel] = cp.triggerKind
				labels[common.GenerateTriggerGroupLabel] = cp.triggerGroup
				labels[common.GenerateTriggerVersionLabel] = cp.triggerAPIVersion

				item.SetLabels(labels)
				item.SetNamespace(namespace)
				item.SetResourceVersion("")
				generatedRes, err := cp.client.CreateResource(context.TODO(), item.GetAPIVersion(), item.GetKind(), namespace, item, false)
				if err != nil {
					return err
				}
				cp.generatedResources = append(cp.generatedResources, generatedRes)
			}
		} else {
			labels := resource.GetLabels()
			if labels == nil {
				labels = make(map[string]string, 8)
			}
			labels[kyverno.LabelAppManagedBy] = kyverno.ValueKyvernoApp
			labels[common.GeneratePolicyLabel] = cp.policyName
			// add trigger metadata labels
			labels[common.GenerateTriggerNameLabel] = cp.triggerName
			labels[common.GenerateTriggerNSLabel] = cp.triggerNamespace
			labels[common.GenerateTriggerUIDLabel] = cp.triggerUID
			labels[common.GenerateTriggerKindLabel] = cp.triggerKind
			labels[common.GenerateTriggerGroupLabel] = cp.triggerGroup
			labels[common.GenerateTriggerVersionLabel] = cp.triggerAPIVersion
			// add source labels
			if resource.GetResourceVersion() != "" {
				labels[common.GenerateSourceUIDLabel] = string(resource.GetUID())
			}
			resource.SetLabels(labels)
			resource.SetNamespace(namespace)
			resource.SetResourceVersion("")
			generatedRes, err := cp.client.CreateResource(context.TODO(), resource.GetAPIVersion(), resource.GetKind(), namespace, resource, false)
			if err != nil {
				return err
			}
			cp.generatedResources = append(cp.generatedResources, generatedRes)
		}
	}
	return nil
}

func (cp *contextProvider) SetPolicyName(name string) {
	cp.policyName = name
}

func (cp *contextProvider) SetTriggerMetadata(name, namespace, uid, apiVersion, group, kind string) {
	cp.triggerName = name
	cp.triggerNamespace = namespace
	cp.triggerUID = uid
	cp.triggerAPIVersion = apiVersion
	cp.triggerGroup = group
	cp.triggerKind = kind
}

func (cp *contextProvider) GetGeneratedResources() []*unstructured.Unstructured {
	return cp.generatedResources
}

func (cp *contextProvider) ClearGeneratedResources() {
	cp.generatedResources = make([]*unstructured.Unstructured, 0)
}

func (cp *contextProvider) getResourceClient(groupVersion schema.GroupVersion, resource string, namespace string) dynamic.ResourceInterface {
	client := cp.client.GetDynamicInterface().Resource(groupVersion.WithResource(resource))
	if namespace != "" {
		return client.Namespace(namespace)
	} else {
		return client
	}
}

func isLikelyKubernetesObject(data any) bool {
	if data == nil {
		return false
	}
	if m, ok := data.(map[string]any); ok {
		_, hasAPIVersion := m["apiVersion"]
		_, hasKind := m["kind"]
		return hasAPIVersion && hasKind
	}
	if _, ok := data.(runtime.Object); ok {
		return true
	}
	return false
}
