package libs

import (
	"context"
	"errors"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/cel/libs/generator"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
	"github.com/kyverno/sdk/cel/libs/imagedata"
	"github.com/kyverno/sdk/cel/libs/resource"
	"github.com/kyverno/sdk/cel/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// a global store for the libaries context, gets initialized when NewContextProvider gets called
// in the controller main functions
var LibraryContext Context

func GetLibsCtx() Context {
	if LibraryContext == nil {
		klog.V(2).Info("global library context was nil, setting to a fake context. If a real context is needed ensure that the variable is set")
		LibraryContext = NewFakeContextProvider()
	}
	return LibraryContext
}

type Context interface {
	globalcontext.ContextInterface
	imagedata.ContextInterface
	resource.ContextInterface
	generator.ContextInterface

	GetGeneratedResources() []*unstructured.Unstructured
	ClearGeneratedResources()
	SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool)
}

type generateContext struct {
	policyName        string
	triggerName       string
	triggerNamespace  string
	triggerAPIVersion string
	triggerGroup      string
	triggerKind       string
	triggerUID        string
	restoreCache      bool
}

type contextProvider struct {
	client             dclient.Interface
	imagedata          imagedataloader.Fetcher
	gctxStore          gctxstore.Store
	generatedResources []*unstructured.Unstructured
	genCtx             generateContext
	cliEvaluation      bool
	restMapper         meta.RESTMapper
}

func NewContextProvider(
	client dclient.Interface,
	imageOpts []imagedataloader.Option,
	gctxStore gctxstore.Store,
	restMapper meta.RESTMapper,
	cliEvaluation bool,
) (Context, error) {
	idl, err := imagedataloader.New(client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace()), imageOpts...)
	if err != nil {
		return nil, err
	}
	ctx := &contextProvider{
		client:             client,
		imagedata:          idl,
		gctxStore:          gctxStore,
		cliEvaluation:      cliEvaluation,
		restMapper:         restMapper,
		generatedResources: make([]*unstructured.Unstructured, 0),
	}
	LibraryContext = ctx
	return ctx, nil
}

func (cp *contextProvider) GetGlobalReference(name, projection string) (any, error) {
	ent, ok := cp.gctxStore.Get(name)
	if !ok {
		logger := logging.GlobalLogger()
		logger.V(2).Info("global context entry not found, returning nil", "entry", name, "projection", projection)
		return nil, nil
	}
	data, err := ent.Get(projection)
	if err != nil {
		return nil, err
	}
	if isLikelyKubernetesObject(data) {
		out, err := kubeutils.ObjToUnstructured(data)
		if err != nil {
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

func (cp *contextProvider) ListResources(apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)

	labelSelector := labels.Everything()
	if len(l) > 0 {
		labelSelector = labels.SelectorFromSet(l)
	}

	return resourceInteface.List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
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

		var items []*unstructured.Unstructured
		if resource.IsList() {
			resourceList, err := resource.ToList()
			if err != nil {
				return err
			}
			for i := range resourceList.Items {
				items = append(items, &resourceList.Items[i])
			}
		} else {
			items = append(items, resource)
		}

		for _, item := range items {
			// In CLI evaluation mode, we do not create the resource in the cluster
			// but just store it in the generated resources list.
			if cp.cliEvaluation {
				item.SetUID("")
				item.SetManagedFields(nil)
				item.SetAnnotations(nil)
				item.SetNamespace(namespace)
				item.SetResourceVersion("")
				item.SetCreationTimestamp(metav1.Time{})
				cp.generatedResources = append(cp.generatedResources, item)
				continue
			}
			cp.addGenerateLabels(item)
			item.SetNamespace(namespace)
			item.SetResourceVersion("")
			// check if the resource is already generated
			_, err := cp.client.GetResource(
				context.TODO(),
				item.GetAPIVersion(),
				item.GetKind(),
				namespace,
				item.GetName(),
			)

			// if the resource is not found, create it
			if err != nil && apierrors.IsNotFound(err) {
				if !cp.genCtx.restoreCache {
					generatedRes, err := cp.client.CreateResource(
						context.TODO(),
						item.GetAPIVersion(),
						item.GetKind(),
						namespace,
						item,
						false,
					)
					if err != nil {
						return err
					}
					cp.generatedResources = append(cp.generatedResources, generatedRes)
				}
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cp *contextProvider) addGenerateLabels(obj *unstructured.Unstructured) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 8)
	}

	labels[kyverno.LabelAppManagedBy] = kyverno.ValueKyvernoApp
	labels[common.GeneratePolicyLabel] = cp.genCtx.policyName
	labels[common.GenerateTriggerNameLabel] = cp.genCtx.triggerName
	labels[common.GenerateTriggerNSLabel] = cp.genCtx.triggerNamespace
	labels[common.GenerateTriggerUIDLabel] = cp.genCtx.triggerUID
	labels[common.GenerateTriggerKindLabel] = cp.genCtx.triggerKind
	labels[common.GenerateTriggerGroupLabel] = cp.genCtx.triggerGroup
	labels[common.GenerateTriggerVersionLabel] = cp.genCtx.triggerAPIVersion

	// Only set source UID label if the object has a resource version
	if obj.GetResourceVersion() != "" {
		labels[common.GenerateSourceUIDLabel] = string(obj.GetUID())
	}

	obj.SetLabels(labels)
}

func (cp *contextProvider) SetGenerateContext(
	polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string,
	restoreCache bool,
) {
	cp.genCtx.policyName = polName
	cp.genCtx.triggerName = triggerName
	cp.genCtx.triggerNamespace = triggerNamespace
	cp.genCtx.triggerAPIVersion = triggerAPIVersion
	cp.genCtx.triggerGroup = triggerGroup
	cp.genCtx.triggerKind = triggerKind
	cp.genCtx.triggerUID = triggerUID
	cp.genCtx.restoreCache = restoreCache
}

func (cp *contextProvider) GetGeneratedResources() []*unstructured.Unstructured {
	return cp.generatedResources
}

func (cp *contextProvider) ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}

	r, err := cp.restMapper.RESTMapping(schema.GroupKind{Group: groupVersion.Group, Kind: kind}, groupVersion.Version)
	if err != nil {
		return nil, err
	}

	return &r.Resource, nil
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
