package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openapiv2 "github.com/google/gnostic-models/openapiv2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

var (
	fuzzCfg        = config.NewDefaultConfiguration(false)
	fuzzMetricsCfg = config.NewDefaultMetricsConfiguration()
	fuzzJp         = jmespath.New(fuzzCfg)

	validateContext = context.Background()
	regClient       = registryclient.NewOrDie()
	validateEngine  = NewEngine(
		fuzzCfg,
		config.NewDefaultMetricsConfiguration(),
		fuzzJp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(regClient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
		"",
	)
	k8sKinds = map[int]string{
		0:  "Config",
		1:  "ConfigMap",
		2:  "CronJob",
		3:  "DaemonSet",
		4:  "Deployment",
		5:  "EndpointSlice",
		6:  "Ingress",
		7:  "Job",
		8:  "LimitRange",
		9:  "List",
		10: "NetworkPolicy",
		11: "PersistentVolume",
		12: "PersistentVolumeClaim",
		13: "Pod",
		14: "ReplicaSet",
		15: "ReplicationController",
		16: "RuntimeClass",
		17: "Secret",
		18: "Service",
		19: "StorageClass",
		20: "VolumeSnapshot",
		21: "VolumeSnapshotClass",
		22: "VolumeSnapshotContent",
	}

	kindToVersion = map[string]string{
		"Config":                "v1",
		"ConfigMap":             "v1",
		"CronJob":               "batch/v1",
		"DaemonSet":             "apps/v1",
		"Deployment":            "apps/v1",
		"EndpointSlice":         "discovery.k8s.io/v1",
		"Ingress":               "networking.k8s.io/v1",
		"Job":                   "batch/v1",
		"LimitRange":            "v1",
		"List":                  "v1",
		"NetworkPolicy":         "networking.k8s.io/v1",
		"PersistentVolume":      "v1",
		"PersistentVolumeClaim": "v1",
		"Pod":                   "v1",
		"ReplicaSet":            "apps/v1",
		"ReplicationController": "v1",
		"RuntimeClass":          "node.k8s.io/v1",
		"Secret":                "v1",
		"Service":               "v1",
		"StorageClass":          "storage.k8s.io/v1",
		"VolumeSnapshot":        "snapshot.storage.k8s.io/v1",
		"VolumeSnapshotClass":   "snapshot.storage.k8s.io/v1",
		"VolumeSnapshotContent": "snapshot.storage.k8s.io/v1",
	}
)

func buildFuzzContext(ff *fuzz.ConsumeFuzzer) (*PolicyContext, error) {
	cpSpec, err := createPolicySpec(ff)
	if err != nil {
		return nil, err
	}
	cpol := &kyverno.ClusterPolicy{}
	cpol.Spec = cpSpec

	if len(autogen.ComputeRules(cpol)) == 0 {
		return nil, fmt.Errorf("No rules created")
	}

	resourceUnstructured, err := createUnstructuredObject(ff)
	if err != nil {
		return nil, err
	}

	policyContext, err := policycontext.NewPolicyContext(
		fuzzJp,
		*resourceUnstructured,
		kyverno.Create,
		nil,
		fuzzCfg,
	)
	if err != nil {
		return nil, err
	}

	policyContext = policyContext.
		WithPolicy(cpol).
		WithNewResource(*resourceUnstructured)

	addOldResource, err := ff.GetBool()
	if err != nil {
		return nil, err
	}

	if addOldResource {
		oldResourceUnstructured, err := createUnstructuredObject(ff)
		if err != nil {
			return nil, err
		}

		oldResource, err := json.Marshal(oldResourceUnstructured)
		if err != nil {
			return policyContext, nil
		}

		err = enginecontext.AddOldResource(policyContext.JSONContext(), oldResource)
		if err != nil {
			return nil, err
		}

		policyContext = policyContext.WithOldResource(*oldResourceUnstructured)
	}

	return policyContext, nil
}

/*
VerifyAndPatchImage
*/
func FuzzVerifyImageAndPatchTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		pc, err := buildFuzzContext(ff)
		if err != nil {
			return
		}

		verifyImageAndPatchEngine := NewEngine(
			fuzzCfg,
			fuzzMetricsCfg,
			fuzzJp,
			nil,
			factories.DefaultRegistryClientFactory(adapters.RegistryClient(registryclient.NewOrDie()), nil),
			imageverifycache.DisabledImageVerifyCache(),
			factories.DefaultContextLoaderFactory(nil),
			nil,
			"",
		)

		_, _ = verifyImageAndPatchEngine.VerifyAndPatchImages(
			context.Background(),
			pc,
		)
	})
}

/*
Validate
*/
func createPolicySpec(ff *fuzz.ConsumeFuzzer) (kyverno.Spec, error) {
	spec := &kyverno.Spec{}
	rules := createRules(ff)
	if len(rules) == 0 {
		return *spec, fmt.Errorf("no rules")
	}
	spec.Rules = rules

	applyAll, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	if applyAll {
		aa := kyverno.ApplyAll
		spec.ApplyRules = &aa
	} else {
		ao := kyverno.ApplyOne
		spec.ApplyRules = &ao
	}

	failPolicy, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	if failPolicy {
		fa := kyverno.Fail
		spec.FailurePolicy = &fa
	} else {
		ig := kyverno.Ignore
		spec.FailurePolicy = &ig
	}

	setValidationFailureAction, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	if setValidationFailureAction {
		audit, err := ff.GetBool()
		if err != nil {
			return *spec, err
		}
		if audit {
			spec.ValidationFailureAction = "Audit"
		} else {
			spec.ValidationFailureAction = "Enforce"
		}
	}

	setValidationFailureActionOverrides, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	if setValidationFailureActionOverrides {
		vfao := make([]kyverno.ValidationFailureActionOverride, 0)
		ff.CreateSlice(&vfao)
		if len(vfao) != 0 {
			spec.ValidationFailureActionOverrides = vfao
		}
	}

	admission, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.Admission = &admission

	background, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.Background = &background

	schemaValidation, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.SchemaValidation = &schemaValidation

	mutateExistingOnPolicyUpdate, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.MutateExistingOnPolicyUpdate = mutateExistingOnPolicyUpdate

	generateExistingOnPolicyUpdate, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.GenerateExistingOnPolicyUpdate = &generateExistingOnPolicyUpdate

	generateExisting, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.GenerateExisting = generateExisting

	return *spec, nil
}

// Creates a slice of Rules
func createRules(ff *fuzz.ConsumeFuzzer) []kyverno.Rule {
	rules := make([]kyverno.Rule, 0)
	noOfRules, err := ff.GetInt()
	if err != nil {
		return rules
	}
	var (
		wg sync.WaitGroup
		m  sync.Mutex
	)
	for i := 0; i < noOfRules%100; i++ {
		ruleBytes, err := ff.GetBytes()
		if err != nil {
			return rules
		}
		wg.Add(1)
		ff1 := fuzz.NewConsumer(ruleBytes)
		go func(ff2 *fuzz.ConsumeFuzzer) {
			defer wg.Done()
			rule, err := createRule(ff2)
			if err != nil {
				return
			}
			m.Lock()
			rules = append(rules, *rule)
			m.Unlock()
		}(ff1)
	}
	wg.Wait()
	return rules
}

// Creates a single rule
func createRule(f *fuzz.ConsumeFuzzer) (*kyverno.Rule, error) {
	rule := &kyverno.Rule{}
	name, err := f.GetString()
	if err != nil {
		return rule, err
	}
	rule.Name = name

	setContext, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setContext {
		c := make([]kyverno.ContextEntry, 0)
		f.CreateSlice(&c)
		if len(c) != 0 {
			rule.Context = c
		}
	}

	mr := &kyverno.MatchResources{}
	f.GenerateStruct(mr)
	rule.MatchResources = *mr

	setExcludeResources, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setExcludeResources {
		er := &kyverno.MatchResources{}
		f.GenerateStruct(mr)
		rule.ExcludeResources = *er
	}

	setRawAnyAllConditions, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setRawAnyAllConditions {
		raac := &apiextv1.JSON{}
		f.GenerateStruct(raac)
		rule.RawAnyAllConditions = raac
	}

	setCELPreconditions, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setCELPreconditions {
		celp := make([]admissionregistrationv1.MatchCondition, 0)
		f.CreateSlice(&celp)
		if len(celp) != 0 {
			rule.CELPreconditions = celp
		}
	}

	setMutation, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setMutation {
		m := &kyverno.Mutation{}
		f.GenerateStruct(m)
		rule.Mutation = *m
	}

	setValidation, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setValidation {
		v := &kyverno.Validation{}
		f.GenerateStruct(v)
		rule.Validation = *v
	}

	setGeneration, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setGeneration {
		g := &kyverno.Generation{}
		f.GenerateStruct(g)
		rule.Generation = *g
	}

	setVerifyImages, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setVerifyImages {
		iv := make([]kyverno.ImageVerification, 0)
		f.CreateSlice(&iv)
		if len(iv) != 0 {
			rule.VerifyImages = iv
		}
	}

	return rule, nil
}

func FuzzEngineValidateTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		cpSpec, err := createPolicySpec(ff)
		if err != nil {
			return
		}
		policy := &kyverno.ClusterPolicy{}
		policy.Spec = cpSpec

		if len(autogen.ComputeRules(policy)) == 0 {
			return
		}

		resourceUnstructured, err := createUnstructuredObject(ff)
		if err != nil {
			return
		}

		pc, err := NewPolicyContext(fuzzJp, *resourceUnstructured, kyverno.Create, nil, fuzzCfg)
		if err != nil {
			t.Skip()
		}

		validateEngine.Validate(
			validateContext,
			pc.WithPolicy(policy),
		)
	})
}

func GetK8sString(ff *fuzz.ConsumeFuzzer) (string, error) {
	allowedChars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.")
	stringLength, err := ff.GetInt()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 0; i < stringLength%63; i++ {
		charIndex, err := ff.GetInt()
		if err != nil {
			return "", err
		}
		sb.WriteString(string(allowedChars[charIndex%len(allowedChars)]))
	}
	return sb.String(), nil
}

func getVersionAndKind(ff *fuzz.ConsumeFuzzer) (string, error) {
	kindToCreate, err := ff.GetInt()
	if err != nil {
		return "", err
	}
	k := k8sKinds[kindToCreate%len(k8sKinds)]
	v := kindToVersion[k]
	var sb strings.Builder
	sb.WriteString("\"apiVersion\": \"")
	sb.WriteString(v)
	sb.WriteString("\", \"kind\": \"")
	sb.WriteString(k)
	sb.WriteString("\"")
	return sb.String(), nil
}

func createLabels(ff *fuzz.ConsumeFuzzer) (string, error) {
	var sb strings.Builder
	noOfLabels, err := ff.GetInt()
	if err != nil {
		return "", err
	}
	for i := 0; i < noOfLabels%30; i++ {
		key, err := GetK8sString(ff)
		if err != nil {
			return "", err
		}
		value, err := GetK8sString(ff)
		if err != nil {
			return "", err
		}
		sb.WriteString("\"")
		sb.WriteString(key)
		sb.WriteString("\":")
		sb.WriteString("\"")
		sb.WriteString(value)
		sb.WriteString("\"")
		if i != (noOfLabels%30)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String(), nil
}

// Creates an unstructured k8s object
func createUnstructuredObject(f *fuzz.ConsumeFuzzer) (*unstructured.Unstructured, error) {
	labels, err := createLabels(f)
	if err != nil {
		return nil, err
	}

	versionAndKind, err := getVersionAndKind(f)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder

	sb.WriteString("{ ")
	sb.WriteString(versionAndKind)
	sb.WriteString(", \"metadata\": { \"creationTimestamp\": \"2020-09-21T12:56:35Z\", \"name\": \"fuzz\", \"labels\": { ")
	sb.WriteString(labels)
	sb.WriteString(" } }, \"spec\": { ")

	for i := 0; i < 1000; i++ {
		typeToAdd, err := f.GetInt()
		if err != nil {
			return kubeutils.BytesToUnstructured([]byte(sb.String()))
		}
		switch typeToAdd % 11 {
		case 0:
			sb.WriteString("\"")
		case 1:
			s, err := f.GetString()
			if err != nil {
				return kubeutils.BytesToUnstructured([]byte(sb.String()))
			}
			sb.WriteString(s)
		case 2:
			sb.WriteString("{")
		case 3:
			sb.WriteString("}")
		case 4:
			sb.WriteString("[")
		case 5:
			sb.WriteString("]")
		case 6:
			sb.WriteString(":")
		case 7:
			sb.WriteString(",")
		case 8:
			sb.WriteString(" ")
		case 9:
			sb.WriteString("\t")
		case 10:
			sb.WriteString("\n")
		}
	}
	return kubeutils.BytesToUnstructured([]byte(sb.String()))
}

/*
Mutate
*/
func FuzzMutateTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {

		ff := fuzz.NewConsumer(data)
		//ff.GenerateStruct(policy)
		cpSpec, err := createPolicySpec(ff)
		if err != nil {
			return
		}
		policy := &kyverno.ClusterPolicy{}
		policy.Spec = cpSpec

		if len(autogen.ComputeRules(policy)) == 0 {
			return
		}

		resource, err := createUnstructuredObject(ff)
		if err != nil {
			return
		}

		// create policy context
		pc, err := NewPolicyContext(
			fuzzJp,
			*resource,
			kyverno.Create,
			nil,
			fuzzCfg,
		)
		if err != nil {
			t.Skip()
		}
		fuzzInterface := FuzzInterface{ff: ff}
		e := NewEngine(
			fuzzCfg,
			config.NewDefaultMetricsConfiguration(),
			fuzzJp,
			adapters.Client(fuzzInterface),
			factories.DefaultRegistryClientFactory(adapters.RegistryClient(nil), nil),
			imageverifycache.DisabledImageVerifyCache(),
			factories.DefaultContextLoaderFactory(nil),
			nil,
			"",
		)
		e.Mutate(
			context.Background(),
			pc.WithPolicy(policy),
		)
	})
}

type FuzzInterface struct {
	ff *fuzz.ConsumeFuzzer
}

func (fi FuzzInterface) GetKubeClient() kubernetes.Interface {
	return nil

}

func (fi FuzzInterface) GetEventsInterface() eventsv1.EventsV1Interface {
	return nil
}

func (fi FuzzInterface) GetDynamicInterface() dynamic.Interface {
	return DynamicFuzz{
		ff: fi.ff,
	}
}

func (fi FuzzInterface) Discovery() dclient.IDiscovery {
	return FuzzIDiscovery{ff: fi.ff}
}

func (fi FuzzInterface) SetDiscovery(discoveryClient dclient.IDiscovery) {

}

func (fi FuzzInterface) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return []byte("fuzz"), fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool) error {
	return fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

type FuzzIDiscovery struct {
	ff *fuzz.ConsumeFuzzer
}

func (fid FuzzIDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	noOfRes, err := fid.ff.GetInt()
	if err != nil {
		return nil, err
	}
	m := make(map[dclient.TopLevelApiDescription]metav1.APIResource)
	for i := 0; i < noOfRes%10; i++ {
		gvGroup, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvVersion, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelKind, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelSubResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvr := schema.GroupVersionResource{
			Group:    gvGroup,
			Version:  gvVersion,
			Resource: gvResource,
		}
		topLevel := dclient.TopLevelApiDescription{
			GroupVersion: gvr.GroupVersion(),
			Kind:         topLevelKind,
			Resource:     topLevelResource,
			SubResource:  topLevelSubResource,
		}
		apiResource := metav1.APIResource{}
		apiName, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		apiResource.Name = apiName

		apiSingularName, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		apiResource.SingularName = apiSingularName

		namespaced, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		apiResource.Namespaced = namespaced

		setGroup, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		if setGroup {
			apiGroup, err := GetK8sString(fid.ff)
			if err != nil {
				return nil, err
			}
			apiResource.Group = apiGroup
		}

		verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection", "proxy"}
		apiResource.Verbs = verbs

		setShortNames, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		var shortNames = make([]string, 0)
		if setShortNames {
			fid.ff.CreateSlice(&shortNames)
			apiResource.ShortNames = shortNames
		}
		setCategories, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		var categories = make([]string, 0)
		if setCategories {
			fid.ff.CreateSlice(&categories)
			apiResource.Categories = categories
		}

		setStorageHash, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		if setStorageHash {
			storageHash, err := fid.ff.GetString()
			if err != nil {
				return nil, err
			}
			apiResource.StorageVersionHash = storageHash
		}
		m[topLevel] = apiResource
	}
	return m, nil
}

func (fid FuzzIDiscovery) GetGVRFromGVK(schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, fmt.Errorf("Not implemented")

}

func (fid FuzzIDiscovery) GetGVKFromGVR(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("Not implemented")

}

func (fid FuzzIDiscovery) OpenAPISchema() (*openapiv2.Document, error) {
	b, err := fid.ff.GetBytes()
	if err != nil {
		return nil, err
	}
	return openapiv2.ParseDocument(b)
}

func (fid FuzzIDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return nil
}

type DynamicFuzz struct {
	ff *fuzz.ConsumeFuzzer
}

func (df DynamicFuzz) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return FuzzNamespaceableResource{
		ff: df.ff,
	}
}

type FuzzResource struct {
	ff *fuzz.ConsumeFuzzer
}

func (fr FuzzResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("Not implemented")
}
func (fr FuzzResource) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	resource, err := createUnstructuredObject(fr.ff)
	if err != nil {
		return nil, err
	}

	return resource, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

type FuzzNamespaceableResource struct {
	ff *fuzz.ConsumeFuzzer
}

func (fnr FuzzNamespaceableResource) Namespace(string) dynamic.ResourceInterface {
	return FuzzResource{
		ff: fnr.ff,
	}
}
func (fr FuzzNamespaceableResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	resource, err := createUnstructuredObject(fr.ff)
	if err != nil {
		return nil, err
	}

	return resource, nil
}
func (fr FuzzNamespaceableResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	var objs []unstructured.Unstructured
	objs = make([]unstructured.Unstructured, 0)
	noOfObjs, err := fr.ff.GetInt()
	if err != nil {
		return nil, err
	}
	for i := 0; i < noOfObjs%10; i++ {
		obj, err := createUnstructuredObject(fr.ff)
		if err != nil {
			return nil, err
		}
		objs = append(objs, *obj)
	}
	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{"kind": "List", "apiVersion": "v1"},
		Items:  objs,
	}, nil
}
func (fr FuzzNamespaceableResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
func (fr FuzzNamespaceableResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
