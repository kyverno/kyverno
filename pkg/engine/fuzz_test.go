package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
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
	for i := 0; i < noOfRules%100; i++ {
		rule, err := createRule(ff)
		if err != nil {
			return rules
		}

		rules = append(rules, *rule)
	}
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
		e := NewEngine(
			fuzzCfg,
			config.NewDefaultMetricsConfiguration(),
			fuzzJp,
			adapters.Client(nil),
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
