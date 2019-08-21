package testrunner

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	ospath "path"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

type test struct {
	ap       string
	t        *testing.T
	testCase *testCase
	// input
	policy        *kyverno.Policy
	tResource     *resourceInfo
	loadResources []*resourceInfo
	// expected
	genResources    []*resourceInfo
	patchedResource *resourceInfo
}

func (t *test) run() {
	var client *client.Client
	var err error
	//mock client is used if generate is defined
	if t.testCase.Expected.Generation != nil {
		// create mock client & load resources
		client, err = createClient(t.loadResources)
		if err != nil {
			t.t.Errorf("Unable to create client. err %s", err)
		}
		// TODO: handle generate
		// assuming its namespaces creation
		decode := kscheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(t.tResource.rawResource), nil, nil)
		_, err = client.CreateResource(t.tResource.gvk.Kind, "", obj, false)
		if err != nil {
			t.t.Errorf("error while creating namespace %s", err)
		}

	}
	// apply the policy engine
	pr, policyInfo, err := t.applyPolicy(t.policy, t.tResource, client)
	if err != nil {
		t.t.Error(err)
		return
	}
	// Expected Result
	// Test succesfuly ?
	t.overAllPass(policyInfo.IsSuccessful(), t.testCase.Expected.Passes)
	t.checkMutationResult(pr, policyInfo)
	t.checkValidationResult(policyInfo)
	t.checkGenerationResult(client, policyInfo)
}

func (t *test) checkMutationResult(pr *resourceInfo, policyInfo info.PolicyInfo) {
	if t.testCase.Expected.Mutation == nil {
		glog.Info("No Mutation check defined")
		return
	}
	// patched resource
	if !compareResource(pr, t.patchedResource) {
		fmt.Println(string(t.patchedResource.rawResource))
		fmt.Println(string(pr.rawResource))
		glog.Warningf("Expected resource %s ", string(pr.rawResource))
		t.t.Error("Patched resources not as expected")
	}

	// check if rules match
	t.compareRules(policyInfo.Rules, t.testCase.Expected.Mutation.Rules)
}

func (t *test) overAllPass(result bool, expected string) {
	b, err := strconv.ParseBool(expected)
	if err != nil {
		t.t.Error(err)
	}
	if result != b {
		t.t.Errorf("Expected value %v and actual value %v dont match", expected, result)
	}
}

func (t *test) compareRules(ruleInfos []info.RuleInfo, rules []tRules) {
	// Compare the rules specified in the expected against the actual rule info returned by the apply policy
	for _, eRule := range rules {
		// Look-up the rule from the policy info
		rule := lookUpRule(eRule.Name, ruleInfos)
		if reflect.DeepEqual(rule, info.RuleInfo{}) {
			t.t.Errorf("Rule with name %s not found", eRule.Name)
			continue
		}
		// get the corresponding rule
		if rule.Name != eRule.Name {
			t.t.Errorf("Rule Name not matching!. expected %s , actual %s", eRule.Name, rule.Name)
		}
		if rule.RuleType.String() != eRule.Type {
			t.t.Errorf("Rule type mismatch!. expected %s, actual %s", eRule.Type, rule.RuleType.String())
		}
		if len(eRule.Messages) != len(rule.Msgs) {
			t.t.Errorf("Number of rule messages not same. expected %d, actual %d", len(eRule.Messages), len(rule.Msgs))
		}
		for i, msg := range eRule.Messages {
			if msg != rule.Msgs[i] {
				t.t.Errorf("Messges dont match!. expected %s, actual %s", msg, rule.Msgs[i])
			}
		}
	}
}

func lookUpRule(name string, ruleInfos []info.RuleInfo) info.RuleInfo {

	for _, r := range ruleInfos {
		if r.Name == name {
			return r
		}
	}
	return info.RuleInfo{}
}

func (t *test) checkValidationResult(policyInfo info.PolicyInfo) {
	if t.testCase.Expected.Validation == nil {
		glog.Info("No Validation check defined")
		return
	}

	// check if rules match
	t.compareRules(policyInfo.Rules, t.testCase.Expected.Validation.Rules)
}

func (t *test) checkGenerationResult(client *client.Client, policyInfo info.PolicyInfo) {
	if t.testCase.Expected.Generation == nil {
		glog.Info("No Generate check defined")
		return
	}
	if client == nil {
		t.t.Error("client needs to be configured")
	}

	// check if rules match
	t.compareRules(policyInfo.Rules, t.testCase.Expected.Generation.Rules)

	// check if the expected resources are generated
	for _, r := range t.genResources {
		n := ParseNameFromObject(r.rawResource)
		ns := ParseNamespaceFromObject(r.rawResource)
		_, err := client.GetResource(r.gvk.Kind, ns, n)
		if err != nil {
			t.t.Errorf("Resource %s/%s of kinf %s not found", ns, n, r.gvk.Kind)
		}
		// compare if the resources are same
		//TODO: comapre []bytes vs unstrcutured resource
	}
}

func (t *test) applyPolicy(policy *kyverno.Policy,
	tresource *resourceInfo,
	client *client.Client) (*resourceInfo, info.PolicyInfo, error) {
	// apply policy on the trigger resource
	// Mutate
	var zeroPolicyInfo info.PolicyInfo
	var err error
	rawResource := tresource.rawResource
	rname := engine.ParseNameFromObject(rawResource)
	rns := engine.ParseNamespaceFromObject(rawResource)
	rkind := engine.ParseKindFromObject(rawResource)
	policyInfo := info.NewPolicyInfo(policy.Name,
		rkind,
		rname,
		rns,
		policy.Spec.ValidationFailureAction)

	resource, err := ConvertToUnstructured(rawResource)
	if err != nil {
		return nil, zeroPolicyInfo, err
	}

	// Apply Mutation Rules
	engineResponse := engine.Mutate(*policy, *resource)
	// patches, ruleInfos := engine.Mutate(*policy, rawResource, *tresource.gvk)
	policyInfo.AddRuleInfos(engineResponse.RuleInfos)
	// TODO: only validate if there are no errors in mutate, why?
	if policyInfo.IsSuccessful() {
		if len(engineResponse.Patches) != 0 {
			rawResource, err = engine.ApplyPatches(rawResource, engineResponse.Patches)
			if err != nil {
				return nil, zeroPolicyInfo, err
			}
		}
	}
	// Validate
	engineResponse = engine.Validate(*policy, *resource)
	policyInfo.AddRuleInfos(engineResponse.RuleInfos)
	if err != nil {
		return nil, zeroPolicyInfo, err
	}

	if rkind == "Namespace" {
		if client != nil {
			engineResponse := engine.Generate(client, *policy, *resource)
			policyInfo.AddRuleInfos(engineResponse.RuleInfos)
		}
	}
	// Generate
	// transform the patched Resource into resource Info
	ri, err := extractResourceRaw(rawResource)
	if err != nil {
		return nil, zeroPolicyInfo, err
	}
	// return the results
	return ri, policyInfo, nil
}

func NewTest(ap string, t *testing.T, tc *testCase) (*test, error) {
	//---INPUT---
	p, err := tc.loadPolicy(ospath.Join(ap, tc.Input.Policy))
	if err != nil {
		return nil, err
	}
	r, err := tc.loadTriggerResource(ap)
	if err != nil {
		return nil, err
	}

	lr, err := tc.loadPreloadedResources(ap)
	if err != nil {
		return nil, err
	}

	//---EXPECTED---
	pr, err := tc.loadPatchedResource(ap)
	if err != nil {
		return nil, err
	}
	gr, err := tc.loadGeneratedResources(ap)
	if err != nil {
		return nil, err
	}
	return &test{
		ap:              ap,
		t:               t,
		testCase:        tc,
		policy:          p,
		tResource:       r,
		loadResources:   lr,
		genResources:    gr,
		patchedResource: pr,
	}, nil
}
