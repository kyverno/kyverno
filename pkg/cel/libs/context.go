package libs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/cel/libs/generator"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
	"github.com/kyverno/sdk/cel/libs/imagedata"
	"github.com/kyverno/sdk/cel/libs/resource"
	"github.com/kyverno/sdk/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
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
	// genMu serialises the SetGenerateContext → GenerateResources →
	// ClearGeneratedResources sequence. With 10 concurrent background workers
	// sharing this singleton, two URs for different policies can otherwise race
	// on genCtx and stamp each other's policy name onto generated resource labels.
	genMu         sync.Mutex
	genLocked     atomic.Bool // true while genMu is held; guards against unlock-without-lock panics
	cliEvaluation bool
	restMapper    meta.RESTMapper
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
	parts := strings.Split(resource, "/")
	resource = parts[0]
	subresources := parts[1:]

	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Get(context.TODO(), name, metav1.GetOptions{}, subresources...)
}

func (cp *contextProvider) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(resource, "/")
	resource = parts[0]
	subresources := parts[1:]

	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Create(context.TODO(), &unstructured.Unstructured{Object: data}, metav1.CreateOptions{}, subresources...)
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
			existing, err := cp.client.GetResource(
				context.TODO(),
				item.GetAPIVersion(),
				item.GetKind(),
				namespace,
				item.GetName(),
			)

			if apierrors.IsNotFound(err) {
				// Resource doesn't exist yet — create it.
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
			} else {
				if cp.genCtx.restoreCache {
					// Bootstrap mode — populate cache without writing to the cluster.
					cp.generatedResources = append(cp.generatedResources, existing)
				} else {
					// Resource already exists — mirror the old ClusterPolicy behaviour:
					// diff the desired state against the cluster and update only if needed,
					// instead of the previous delete+recreate on every Synchronize UR.
					//
					// Build the desired object: existing metadata (UID, resourceVersion…)
					// merged with the content from the CEL expression plus correct labels.
					desired := existing.DeepCopy()
					for k, v := range item.Object {
						if k != "metadata" && k != "status" {
							desired.Object[k] = v
						}
					}
					cp.addGenerateLabels(desired)

					// Determine whether the cluster state matches the desired state.
					// Check all management labels set by addGenerateLabels, not just the
					// policy name, to catch stale trigger metadata from prior label races.
					managedLabels := []string{
						kyverno.LabelAppManagedBy,
						common.GeneratePolicyLabel,
						common.GenerateTriggerNameLabel,
						common.GenerateTriggerNSLabel,
						common.GenerateTriggerUIDLabel,
						common.GenerateTriggerKindLabel,
						common.GenerateTriggerGroupLabel,
						common.GenerateTriggerVersionLabel,
					}
					existingLabels := existing.GetLabels()
					desiredLabels := desired.GetLabels()
					needsUpdate := false
					for _, lbl := range managedLabels {
						if desiredLabels[lbl] != existingLabels[lbl] {
							needsUpdate = true
							break
						}
					}
					if !needsUpdate {
						for k := range item.Object {
							if k == "metadata" || k == "status" {
								continue
							}
							if !reflect.DeepEqual(desired.Object[k], existing.Object[k]) {
								needsUpdate = true
								break
							}
						}
					}

					if needsUpdate {
						updated, err := cp.client.UpdateResource(
							context.TODO(),
							desired.GetAPIVersion(),
							desired.GetKind(),
							desired.GetNamespace(),
							desired,
							false,
						)
						if err != nil {
							return err
						}
						cp.generatedResources = append(cp.generatedResources, updated)
					} else {
						cp.generatedResources = append(cp.generatedResources, existing)
					}
				}
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
	// Hold the lock until ClearGeneratedResources to prevent concurrent workers
	// from interleaving their genCtx writes and GenerateResources calls.
	cp.genMu.Lock()
	cp.genLocked.Store(true)
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
	// Only unlock if SetGenerateContext was called; guards against unlock-without-lock
	// panics when Evaluate is called outside the normal engine path (e.g. CLI mode).
	if cp.genLocked.CompareAndSwap(true, false) {
		cp.genMu.Unlock()
	}
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
