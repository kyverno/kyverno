package fuzz

import (
	"fmt"
	"sync"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

func CreatePolicySpec(ff *fuzz.ConsumeFuzzer) (kyverno.Spec, error) {
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
		celp := make([]admissionregistrationv1alpha1.MatchCondition, 0)
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
