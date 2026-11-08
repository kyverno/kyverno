package libs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/cel/libs/generator"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/kyverno/sdk/extensions/registryclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	corev1listers "k8s.io/client-go/listers/core/v1"
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

	GetHTTPMocks() map[string]interface{}
	GetGeneratedResources() []*unstructured.Unstructured
	ClearGeneratedResources()
	SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache, useServerSideApply bool)
	Clone() Context
}

type generateContext struct {
	policyName         string
	policyNamespace    string
	triggerName        string
	triggerNamespace   string
	triggerAPIVersion  string
	triggerGroup       string
	triggerKind        string
	triggerUID         string
	restoreCache       bool
	useServerSideApply bool
}

type contextProvider struct {
	client             dclient.Interface
	imagedata          imagedataloader.Fetcher
	gctxStore          gctxstore.Store
	generatedResources []*unstructured.Unstructured
	// UID-qualified identities from the current UpdateRequest, isolated per
	// evaluation. These authorize only matching legacy downstream objects.
	acknowledgedGeneratedResources map[string]string
	genCtx             generateContext
	cliEvaluation      bool // if true, libraries that create resources (like the generator library) don't post the created resource to an actual cluster
	restMapper         meta.RESTMapper
}

func NewContextProvider(
	client dclient.Interface,
	secretLister corev1listers.SecretLister,
	gctxStore gctxstore.Store,
	restMapper meta.RESTMapper,
	cliEvaluation bool,
) (Context, error) {
	// By default, the libraries context uses the global registry client credentials.
	// callers who will need to pass in different authentication options (the ivpol)
	// will simply pass different opts to the image data loader during image fetching
	authOpts, nameOpts := registryclient.GlobalOptsOrDefault(context.Background())

	idl, err := imagedataloader.New(secretLister, authOpts, nameOpts)
	if err != nil {
		return nil, err
	}
	ctx := &contextProvider{
		client:             client,
		imagedata:          idl,
		gctxStore:          gctxStore,
		restMapper:         restMapper,
		cliEvaluation:      cliEvaluation,
		generatedResources: make([]*unstructured.Unstructured, 0),
	}
	LibraryContext = ctx
	return ctx, nil
}

func (cp *contextProvider) GetHTTPMocks() map[string]interface{} {
	return nil
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
			// Return the map form: since cel-go v0.31 (#17067) NativeToValue only converts
			// registered native types, and unstructured.Unstructured is not one.
			return out.UnstructuredContent(), nil
		} else {
			return nil, errors.New("failed to convert to Unstructured")
		}
	} else {
		return data, nil
	}
}

func (cp *contextProvider) GetImageData(image string, remoteOpts []remote.Option) (map[string]any, error) {
	return cp.GetImageDataContext(context.TODO(), image, remoteOpts)
}

func (cp *contextProvider) GetImageDataContext(ctx context.Context, image string, remoteOpts []remote.Option) (map[string]any, error) {
	// NOTE: we deliberately not pass name options here because there is currently only one
	// name option we build, which is name.Insecure. This option already gets build and passed
	// during the fetching of the global registry client options and then building the image data
	// loader from those options.
	// the current state means we are using the flags of the registry client to denote whether we use the name insecure option here
	// so we aren't honoring it per policy. but if we did per policy, then a policy without anything wouldn't pass this opt
	// but the if the flag is set, the registry client opts will come with the name insecure option
	data, err := cp.imagedata.FetchImageData(ctx, image, remoteOpts, nil)
	if err != nil {
		return nil, err
	}
	return utils.GetValue(data.Data())
}

func (cp *contextProvider) ListResources(apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error) {
	return cp.ListResourcesContext(context.TODO(), apiVersion, resource, namespace, l)
}

func (cp *contextProvider) ListResourcesContext(ctx context.Context, apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	if err := validateGroupVersion(groupVersion); err != nil {
		return nil, err
	}
	if err := validateNamespaceSegment(namespace); err != nil {
		return nil, err
	}
	if err := validatePathSegment("resource", resource); err != nil {
		return nil, err
	}

	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)

	labelSelector := labels.Everything()
	if len(l) > 0 {
		labelSelector = labels.SelectorFromSet(l)
	}

	return resourceInteface.List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
}

func (cp *contextProvider) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return cp.GetResourceContext(context.TODO(), apiVersion, resource, namespace, name)
}

func (cp *contextProvider) GetResourceContext(ctx context.Context, apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	if err := validateGroupVersion(groupVersion); err != nil {
		return nil, err
	}
	if err := validateNamespaceSegment(namespace); err != nil {
		return nil, err
	}
	resource, subresources, err := splitResourcePath(resource)
	if err != nil {
		return nil, err
	}
	if err := validatePathSegment("name", name); err != nil {
		return nil, err
	}

	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Get(ctx, name, metav1.GetOptions{}, subresources...)
}

func (cp *contextProvider) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return cp.PostResourceContext(context.TODO(), apiVersion, resource, namespace, data)
}

func (cp *contextProvider) PostResourceContext(ctx context.Context, apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	if err := validateGroupVersion(groupVersion); err != nil {
		return nil, err
	}
	if err := validateNamespaceSegment(namespace); err != nil {
		return nil, err
	}
	resource, subresources, err := splitResourcePath(resource)
	if err != nil {
		return nil, err
	}

	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Create(ctx, &unstructured.Unstructured{Object: data}, metav1.CreateOptions{}, subresources...)
}

func (cp *contextProvider) GenerateResources(namespace string, dataList []map[string]any) error {
	return cp.GenerateResourcesContext(context.TODO(), namespace, dataList)
}

func (cp *contextProvider) GenerateResourcesContext(ctx context.Context, namespace string, dataList []map[string]any) error {
	if ctx == nil {
		ctx = context.Background()
	}
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
			targetNamespace := namespace
			if !cp.isNamespacedResource(item.GetAPIVersion(), item.GetKind()) {
				// A non-empty namespace means the call is scoped to a single
				// namespace, which for a namespaced policy is its own namespace
				// (enforced in the generator lib). Cluster-scoped resources have
				// no namespace, so generating one would escape that scope. Reject
				// it instead of silently creating the resource cluster-wide.
				if namespace != "" && cp.genCtx.policyNamespace != "" {
					return fmt.Errorf("cross-scope generation denied: a policy scoped to namespace %q cannot generate cluster-scoped resource %s/%s", namespace, item.GetAPIVersion(), item.GetKind())
				}
				targetNamespace = ""
			}

			// When generating a namespaced resource into a different namespace than
			// its source (the typical cross-namespace clone case), inherited
			// ownerReferences may point to a namespaced owner that does not exist in
			// the target namespace, causing the garbage collector to delete the
			// generated resource almost immediately. Mirror the legacy clone behavior
			// (see pkg/background/generate/clone.go) by stripping all ownerReferences
			// when the source namespace is set and differs from the target.
			if srcNamespace := item.GetNamespace(); srcNamespace != "" && srcNamespace != targetNamespace && item.GetOwnerReferences() != nil {
				item.SetOwnerReferences(nil)
			}

			// In CLI evaluation mode, we do not create the resource in the cluster
			// but just store it in the generated resources list.
			if cp.cliEvaluation {
				item.SetUID("")
				item.SetManagedFields(nil)
				item.SetAnnotations(nil)
				item.SetNamespace(targetNamespace)
				item.SetResourceVersion("")
				item.SetCreationTimestamp(metav1.Time{})
				cp.generatedResources = append(cp.generatedResources, item)
				continue
			}
			cp.addGenerateLabels(item)
			// Clean up server-populated metadata that must not be copied to the
			// generated resource, mirroring the legacy clone behavior
			// (see pkg/background/generate/clone.go). In particular, a non-nil
			// managedFields is rejected by server-side apply. This must run after
			// addGenerateLabels, which reads the source UID and resourceVersion to
			// set the generate.kyverno.io/source-uid label.
			item.SetUID("")
			item.SetSelfLink("")
			item.SetCreationTimestamp(metav1.Time{})
			item.SetManagedFields(nil)
			item.SetResourceVersion("")
			item.SetNamespace(targetNamespace)
			// check if the resource already exists
			existing, err := cp.client.GetResource(
				ctx,
				item.GetAPIVersion(),
				item.GetKind(),
				targetNamespace,
				item.GetName(),
			)
			if err != nil {
				// if the resource is not found, create it
				if apierrors.IsNotFound(err) {
					if !cp.genCtx.restoreCache {
						var generatedRes *unstructured.Unstructured
						if cp.genCtx.useServerSideApply {
							generatedRes, err = cp.client.ApplyResource(
								ctx,
								item.GetAPIVersion(),
								item.GetKind(),
								targetNamespace,
								item.GetName(),
								item,
								false,
								"generate",
							)
						} else {
							generatedRes, err = cp.client.CreateResource(
								ctx,
								item.GetAPIVersion(),
								item.GetKind(),
								targetNamespace,
								item,
								false,
							)
						}
						if err != nil {
							return err
						}
						cp.generatedResources = append(cp.generatedResources, generatedRes)
					}
					continue
				}
				return err
			}
			owned, ownershipErr := cp.isManagedByPolicyChecked(ctx, existing)
			if ownershipErr != nil {
				// An incomplete desired set must not be reconciled as success.
				// In particular, legacy scope ambiguity must stop evaluation,
				// not silently make a previously generated object disappear.
				return fmt.Errorf("verify generated resource %s/%s ownership: %w", targetNamespace, item.GetName(), ownershipErr)
			}
			if !owned {
				// A pre-existing resource belonging to somebody else must not
				// be reported or updated by this generating policy.
				continue
			}
			// the resource already exists and is managed by this policy
			// (e.g. a resync, a retry, or a cacheRestore pass over a
			// resource generated by an earlier UR).
			if cp.genCtx.restoreCache {
				// Report existing generated resource so callers relying on the generated
				// resources list (e.g. WatchManager.SyncWatchers) keep the watcher/cache.
				cp.generatedResources = append(cp.generatedResources, existing)
				continue
			}
			if cp.genCtx.useServerSideApply {
				generatedRes, err := cp.client.ApplyResource(
					ctx,
					item.GetAPIVersion(),
					item.GetKind(),
					targetNamespace,
					item.GetName(),
					item,
					false,
					"generate",
				)
				if err != nil {
					return err
				}
				cp.generatedResources = append(cp.generatedResources, generatedRes)
				continue
			}
			// Update the downstream resource in place with the newly rendered
			// content so trigger updates are propagated without deleting and
			// recreating it. The UPDATE API requires UID and resourceVersion to
			// match the existing object; copy them from the fetched resource.
			item.SetUID(existing.GetUID())
			item.SetResourceVersion(existing.GetResourceVersion())
			generatedRes, err := cp.client.UpdateResource(
				ctx,
				item.GetAPIVersion(),
				item.GetKind(),
				targetNamespace,
				item,
				false,
			)
			if err != nil {
				return err
			}
			cp.generatedResources = append(cp.generatedResources, generatedRes)
		}
	}
	return nil
}

// isManagedByPolicy reports whether obj is already labeled as a downstream
// resource generated by this provider's policy for the trigger currently
// being processed. It is used to avoid reporting a pre-existing, unrelated
// resource (one that merely shares the same GVK/namespace/name) as if it
// were generated by us. The trigger UID check additionally prevents two
// different triggers of the same policy from "adopting" each other's
// downstream resource when they happen to render the same target name.
func (cp *contextProvider) isManagedByPolicy(ctx context.Context, obj *unstructured.Unstructured) bool {
	owned, _ := cp.isManagedByPolicyChecked(ctx, obj)
	return owned
}

func (cp *contextProvider) isManagedByPolicyChecked(ctx context.Context, obj *unstructured.Unstructured) (bool, error) {
	if obj == nil {
		return false, nil
	}
	labels := obj.GetLabels()
	if labels[kyverno.LabelAppManagedBy] != kyverno.ValueKyvernoApp ||
		labels[common.GeneratePolicyLabel] != cp.genCtx.policyName ||
		labels[common.GenerateTriggerUIDLabel] != cp.genCtx.triggerUID {
		return false, nil
	}
	if cp.genCtx.policyNamespace != "" && !common.MatchesPolicyTriggerNamespace(labels, cp.genCtx.policyNamespace) {
		return false, nil
	}
	if scope, marked := labels[common.GeneratePolicyScopeLabel]; marked {
		if cp.genCtx.policyNamespace == "" {
			return scope == common.GeneratePolicyScopeCluster, nil
		}
		return scope == common.GeneratePolicyScopeNamespaced, nil
	}
	// Apart from Namespace, cluster-scoped triggers cannot belong to a
	// namespaced policy. Namespace triggers can, so legacy objects need the
	// counterpart policy lookup below before being adopted.
	if cp.genCtx.policyNamespace == "" && labels[common.GenerateTriggerNSLabel] == "" &&
		labels[common.GenerateTriggerKindLabel] != "Namespace" {
		return true, nil
	}
	// Historical downstreams with a namespaced or Namespace trigger have no
	// durable policy-scope identity. The absence of a same-named policy in the
	// other scope is NOT proof of origin: that policy may have been deleted.
	// Do not adopt, update or delete these ambiguous objects without a
	// UID-qualified UpdateRequest record. Compare the *live* object's UID to
	// the persisted identity supplied for this evaluation by the controller.
	if key := acknowledgedResourceKey(obj.GetAPIVersion(), obj.GetKind(), obj.GetNamespace(), obj.GetName()); key != "" &&
		cp.acknowledgedGeneratedResources[key] != "" && cp.acknowledgedGeneratedResources[key] == string(obj.GetUID()) {
		return true, nil
	}
	// Reject the evaluation rather than reporting a successful empty desired
	// set that could prune an otherwise live watcher registration.
	return false, fmt.Errorf("ambiguous legacy downstream without verified policy ownership")
}

func (cp *contextProvider) addGenerateLabels(obj *unstructured.Unstructured) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 8)
	}

	labels[kyverno.LabelAppManagedBy] = kyverno.ValueKyvernoApp
	labels[common.GeneratePolicyLabel] = cp.genCtx.policyName
	if cp.genCtx.policyNamespace == "" {
		labels[common.GeneratePolicyScopeLabel] = common.GeneratePolicyScopeCluster
	} else {
		labels[common.GeneratePolicyScopeLabel] = common.GeneratePolicyScopeNamespaced
	}
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
	polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string,
	restoreCache, useServerSideApply bool,
) {
	cp.genCtx.policyName = polName
	cp.genCtx.policyNamespace = policyNamespace
	cp.genCtx.triggerName = triggerName
	cp.genCtx.triggerNamespace = triggerNamespace
	cp.genCtx.triggerAPIVersion = triggerAPIVersion
	cp.genCtx.triggerGroup = triggerGroup
	cp.genCtx.triggerKind = triggerKind
	cp.genCtx.triggerUID = triggerUID
	cp.genCtx.restoreCache = restoreCache
	cp.genCtx.useServerSideApply = useServerSideApply
}

// acknowledgedResourceKey ignores served API version but preserves the group,
// kind and namespaced identity of a Kubernetes object.
func acknowledgedResourceKey(apiVersion, kind, namespace, name string) string {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return ""
	}
	return gv.Group + "\x00" + kind + "\x00" + namespace + "\x00" + name
}

// SetAcknowledgedGeneratedResources supplies the persisted UID identities of
// previously generated resources for this evaluation only. It does not infer
// ownership from the absence of a same-name policy in another scope.
func (cp *contextProvider) SetAcknowledgedGeneratedResources(specs []kyverno.ResourceSpec) {
	cp.acknowledgedGeneratedResources = make(map[string]string, len(specs))
	for _, spec := range specs {
		if spec.UID == "" || spec.Kind == "" || spec.Name == "" {
			continue
		}
		if key := acknowledgedResourceKey(spec.APIVersion, spec.Kind, spec.Namespace, spec.Name); key != "" {
			cp.acknowledgedGeneratedResources[key] = string(spec.UID)
		}
	}
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

func (cp *contextProvider) isNamespacedResource(apiVersion, kind string) bool {
	if cp.restMapper == nil || apiVersion == "" || kind == "" {
		return true
	}
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return true
	}
	r, err := cp.restMapper.RESTMapping(schema.GroupKind{Group: groupVersion.Group, Kind: kind}, groupVersion.Version)
	if err != nil || r.Scope == nil {
		return true
	}
	return r.Scope.Name() == meta.RESTScopeNameNamespace
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

func (cp *contextProvider) Clone() Context {
	// Returns a shallow copy. Maps, clients, and other referenced mutable state remain shared.
	// Only the copied top-level struct fields and the per-worker generatedResources list are isolated here.
	clone := *cp

	// generatedResources is per-evaluation state. Ensure each worker starts with a clean slate.
	clone.generatedResources = make([]*unstructured.Unstructured, 0)

	return &clone
}
