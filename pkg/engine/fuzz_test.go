package engine

import (
	"bytes"
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

/*
VerifyAndPatchImage
*/
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
)

func buildFuzzContext(policy, resource, oldResource []byte) (*PolicyContext, error) {
	var cpol kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	if err != nil {
		return nil, err
	}

	resourceUnstructured, err := kubeutils.BytesToUnstructured(resource)
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
		WithPolicy(&cpol).
		WithNewResource(*resourceUnstructured)

	if !bytes.Equal(oldResource, []byte("")) {
		oldResourceUnstructured, err := kubeutils.BytesToUnstructured(oldResource)
		if err != nil {
			return nil, err
		}

		err = enginecontext.AddOldResource(policyContext.JSONContext(), oldResource)
		if err != nil {
			return nil, err
		}

		policyContext = policyContext.WithOldResource(*oldResourceUnstructured)
	}

	return policyContext, nil
}

func FuzzVerifyImageAndPatchTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, policy, resource, oldResource []byte) {
		pc, err := buildFuzzContext(policy, resource, oldResource)
		if err != nil {
			return
		}

		verifyImageAndPatchEngine := NewEngine(
			cfg,
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

// Creates an unstructured k8s object
func createUnstructuredObject(f *fuzz.ConsumeFuzzer) (*unstructured.Unstructured, error) {
	var sb strings.Builder

	sb.WriteString("{ \"apiVersion\": \"apps/v1\", \"kind\": \"Deployment\", \"metadata\": { \"creationTimestamp\": \"2020-09-21T12:56:35Z\", \"name\": \"fuzz\", \"labels\": { \"test\": \"qos\" } }, \"spec\": { ")

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
	f.Fuzz(func(t *testing.T, resourceRaw, policyRaw []byte) {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(policyRaw, &policy)
		if err != nil {
			return
		}
		var resource unstructured.Unstructured
		err = resource.UnmarshalJSON(resourceRaw)
		if err != nil {
			return
		}

		// create policy context
		pc, err := NewPolicyContext(
			fuzzJp,
			resource,
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
			pc.WithPolicy(&policy),
		)
		panic("Here")
	})
}
