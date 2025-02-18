package fuzz

import (
	"fmt"
	"sync"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func CreatePolicySpec(ff *fuzz.ConsumeFuzzer) (kyvernov1.Spec, error) {
	spec := &kyvernov1.Spec{}
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
		aa := kyvernov1.ApplyAll
		spec.ApplyRules = &aa
	} else {
		ao := kyvernov1.ApplyOne
		spec.ApplyRules = &ao
	}

	failPolicy, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	if failPolicy {
		fa := kyvernov1.Fail
		spec.FailurePolicy = &fa
	} else {
		ig := kyvernov1.Ignore
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
		vfao := make([]kyvernov1.ValidationFailureActionOverride, 0)
		err = ff.CreateSlice(&vfao)
		if err != nil {
			return *spec, err
		}
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

	generateExisting, err := ff.GetBool()
	if err != nil {
		return *spec, err
	}
	spec.GenerateExisting = generateExisting

	return *spec, nil
}

// Creates a slice of Rules
func createRules(ff *fuzz.ConsumeFuzzer) []kyvernov1.Rule {
	rules := make([]kyvernov1.Rule, 0)
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
func createRule(f *fuzz.ConsumeFuzzer) (*kyvernov1.Rule, error) {
	rule := &kyvernov1.Rule{}
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
		c := make([]kyvernov1.ContextEntry, 0)
		err = f.CreateSlice(&c)
		if err != nil {
			return rule, err
		}
		if len(c) != 0 {
			rule.Context = c
		}
	}

	mr := &kyvernov1.MatchResources{}
	err = f.GenerateStruct(mr)
	if err != nil {
		return rule, err
	}
	rule.MatchResources = *mr

	setExcludeResources, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setExcludeResources {
		er := &kyvernov1.MatchResources{}
		err = f.GenerateStruct(mr)
		if err != nil {
			return rule, err
		}
		rule.ExcludeResources = er
	}

	setRawAnyAllConditions, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setRawAnyAllConditions {
		raac := &kyvernov1.ConditionsWrapper{}
		err = f.GenerateStruct(raac)
		if err != nil {
			return rule, err
		}
		rule.RawAnyAllConditions = raac
	}

	setCELPreconditions, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setCELPreconditions {
		celp := make([]admissionregistrationv1.MatchCondition, 0)
		err = f.CreateSlice(&celp)
		if err != nil {
			return rule, err
		}
		if len(celp) != 0 {
			rule.CELPreconditions = celp
		}
	}

	setMutation, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setMutation {
		m := &kyvernov1.Mutation{}
		err = f.GenerateStruct(m)
		if err != nil {
			return rule, err
		}
		rule.Mutation = m
	}

	setValidation, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setValidation {
		v := &kyvernov1.Validation{}
		err = f.GenerateStruct(v)
		if err != nil {
			return rule, err
		}
		rule.Validation = v
	}

	setGeneration, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setGeneration {
		g := &kyvernov1.Generation{}
		err = f.GenerateStruct(g)
		if err != nil {
			return rule, err
		}
		rule.Generation = g
	}

	setVerifyImages, err := f.GetBool()
	if err != nil {
		return rule, err
	}
	if setVerifyImages {
		iv := make([]kyvernov1.ImageVerification, 0)
		err = f.CreateSlice(&iv)
		if err != nil {
			return rule, err
		}
		if len(iv) != 0 {
			rule.VerifyImages = iv
		}
	}

	return rule, nil
}
