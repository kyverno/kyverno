package testrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	ospath "path"
	"path/filepath"
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type scenarioT struct {
	testCases []scaseT
}

//scase defines input and output for a case
type scaseT struct {
	Input    sInput    `yaml:"input"`
	Expected sExpected `yaml:"expected"`
}

//sInput defines input for a test scenario
type sInput struct {
	Policy        string   `yaml:"policy"`
	Resource      string   `yaml:"resource"`
	LoadResources []string `yaml:"loadresources,omitempty"`
}

type sExpected struct {
	Mutation   sMutation   `yaml:"mutation,omitempty"`
	Validation sValidation `yaml:"validation,omitempty"`
	// Generation sGeneration `yaml:"generation,omitempty"`
}

type sMutation struct {
	// path to the patched resource to be compared with
	PatchedResource string `yaml:"patchedresource,omitempty"`
	// expected respone from the policy engine
	PolicyResponse engine.PolicyResponse `yaml:"policyresponse"`
}

type sValidation struct {
	// expected respone from the policy engine
	PolicyResponse engine.PolicyResponse `yaml:"policyresponse"`
}

//getRelativePath expects a path relative to project and builds the complete path
func getRelativePath(path string) string {
	gp := os.Getenv("GOPATH")
	ap := ospath.Join(gp, projectPath)
	return ospath.Join(ap, path)
}

func loadScenario(t *testing.T, path string) (*scenarioT, error) {
	fileBytes, err := loadFile(t, path)
	if err != nil {
		return nil, err
	}

	var testCases []scaseT
	// load test cases seperated by '---'
	// each test case defines an input & expected result
	scenariosBytes := bytes.Split(fileBytes, []byte("---"))
	for _, scenarioBytes := range scenariosBytes {
		tc := scaseT{}
		if err := yaml.Unmarshal([]byte(scenarioBytes), &tc); err != nil {
			t.Errorf("failed to decode test case YAML: %v", err)
			continue
		}
		testCases = append(testCases, tc)
	}
	scenario := scenarioT{
		testCases: testCases,
	}

	return &scenario, nil
}

// loadFile loads file in byte buffer
func loadFile(t *testing.T, path string) ([]byte, error) {
	path = getRelativePath(path)
	t.Logf("reading file %s", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

//getFiles loads all scneario files in specified folder path
func getFiles(t *testing.T, folder string) ([]string, error) {
	t.Logf("loading scneario files for folder %s", folder)
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var yamls []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yml" || filepath.Ext(file.Name()) == ".yaml" {
			yamls = append(yamls, ospath.Join(folder, file.Name()))
		}
	}
	return yamls, nil
}

func runScenario(t *testing.T, s *scenarioT) bool {
	for _, tc := range s.testCases {
		runTestCase(t, tc)
	}
	return true
}

func runTestCase(t *testing.T, tc scaseT) bool {
	// var client *client.Client
	// client, err := getClient(tc.Input.LoadResources)

	// generate mock client if resources are to be loaded
	// - create mock client
	// - load resources
	client := getClient(t, tc.Input.LoadResources)
	t.Log(client)
	// apply policy
	// convert policy -> kyverno.Policy
	policy := loadPolicy(t, tc.Input.Policy)
	fmt.Println(policy)
	// convert resource -> unstructured.Unstructured
	resource := loadPolicyResource(t, tc.Input.Resource)
	glog.Info(resource)

	var er engine.EngineResponseNew
	// Mutation
	er = engine.MutateNew(*policy, *resource)
	func() {
		if data, err := json.Marshal(er.PolicyResponse); err == nil {
			t.Log(string(data))
			fmt.Println(string(data))
			for _, r := range er.PolicyResponse.Rules {
				for _, p := range r.Patches {
					fmt.Println(string(p))
				}
			}
		}
	}()

	validateResource(t, er.PatchedResource, tc.Expected.Mutation.PatchedResource)
	validateResponse(t, er.PolicyResponse, tc.Expected.Mutation.PolicyResponse)

	// Validation
	// only compare the parametes specified ?
	return true
}

func validateResource(t *testing.T, responseResource unstructured.Unstructured, expectedResourceFile string) {
	if expectedResourceFile == "" {
		t.Log("expected resource file not specified, wont compare resources")
		return
	}
	// load expected resource
	expectedResource := loadPolicyResource(t, expectedResourceFile)
	if expectedResource == nil {
		t.Log("failed to get the expected resource")
		return
	}
	resourcePrint := func(obj unstructured.Unstructured) {
		if data, err := obj.MarshalJSON(); err == nil {
			fmt.Println(string(data))
		}
	}
	resourcePrint(responseResource)
	resourcePrint(*expectedResource)
	// compare the resources
	if !reflect.DeepEqual(responseResource, *expectedResource) {
		t.Log("failed: response resource returned does not match expected resource")
	}
	t.Log("success: response resource returned matches expected resource")
}

func validateResponse(t *testing.T, er engine.PolicyResponse, expected engine.PolicyResponse) {
	// cant do deepEquals and the stats will be different, or we nil those fields and then do a comparison
	// forcing only the fields that are specified to be comprared

	// doing a field by fields comparsion will allow us to provied more details logs and granular error reporting
	// check policy name is same :P
	if er.Policy != expected.Policy {
		t.Log("Policy: error")
	}
	// compare resource spec
	if er.Resource != expected.Resource {
		t.Log("Resource: error")
	}
	// stats
	if er.RulesAppliedCount != expected.RulesAppliedCount {
		t.Log("RulesAppliedCount: error")
	}
	// rules
	if len(er.Rules) != len(er.Rules) {
		t.Log("rule count: error")
	}
	if len(expected.Rules) == len(expected.Rules) {
		// if there are rules being applied then we compare the rule response
		// as the rules are applied in the order defined, the comparions of rules will be in order
		for index, r := range expected.Rules {
			compareRules(t, r, expected.Rules[index])
		}
	}
}

func compareResourceSpec(t *testing.T, resource engine.ResourceSpec, expectedResource engine.ResourceSpec) {
	// kind
	if resource.Kind != expectedResource.Kind {
		t.Error("error: kind")
	}
	// apiVersion
	if resource.APIVersion != expectedResource.APIVersion {
		t.Error("error: apiVersion")
	}
	// namespace
	if resource.Namespace != expectedResource.Namespace {
		t.Error("error: namespace")
	}
	// name
	if resource.Name != expectedResource.Name {
		t.Error("error: name")
	}
}

func compareRules(t *testing.T, rule engine.RuleResponse, expectedRule engine.RuleResponse) {
	// name
	if rule.Name != expectedRule.Name {
		t.Logf("error rule %s: name", rule.Name)
		// as the rule names dont match no need to compare the rest of the information
		return
	}
	// type
	if rule.Type != expectedRule.Type {
		t.Log("error: typw")
	}
	// message
	if rule.Message != expectedRule.Message {
		t.Log("error: message")
	}
	// // patches
	// if reflect.DeepEqual(rule.Patches, expectedRule.Patches) {
	// 	t.Log("error: patches")
	// }
	// success
	if rule.Success != expectedRule.Success {
		t.Log("error: success")
	}
	// statistics
}

func loadPolicyResource(t *testing.T, file string) *unstructured.Unstructured {
	// expect only one resource to be specified in the YAML
	resources := loadResource(t, file)
	if resources == nil {
		t.Log("no resource specified")
		return nil
	}
	if len(resources) > 1 {
		t.Logf("more than one resource specified in the file %s", file)
		t.Log("considering the first one for policy application")
	}
	return resources[0]
}

func getClient(t *testing.T, files []string) *client.Client {
	if files == nil {
		t.Log("no resources to load, not createing mock client")
		return nil
	}
	var objects []runtime.Object
	if files != nil {
		glog.V(4).Infof("loading resources:")
		for _, file := range files {
			objects = loadObjects(t, file)
		}
	}
	// create mock client
	scheme := runtime.NewScheme()
	// mock client expects the resource to be as runtime.Object
	c, err := client.NewMockClient(scheme, objects...)
	if err != nil {
		t.Errorf("failed to create client. %v", err)
		return nil
	}
	t.Log("created mock client with pre-loaded resources")
	return c
}

func loadResource(t *testing.T, path string) []*unstructured.Unstructured {
	var unstrResources []*unstructured.Unstructured
	t.Logf("loading resource from %s", path)
	data, err := loadFile(t, path)
	if err != nil {
		return nil
	}
	rBytes := bytes.Split(data, []byte("---"))
	for _, r := range rBytes {
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, gvk, err := decode(r, nil, nil)
		if err != nil {
			t.Logf("failed to decode resource: %v", err)
			continue
		}
		glog.Info(gvk)

		data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
		if err != nil {
			t.Logf("failed to unmarshall resource. %v", err)
			continue
		}
		unstr := unstructured.Unstructured{Object: data}
		t.Logf("loaded resource %s/%s/%s", unstr.GetKind(), unstr.GetNamespace(), unstr.GetName())
		unstrResources = append(unstrResources, &unstr)
	}
	return unstrResources
}

func loadObjects(t *testing.T, path string) []runtime.Object {
	var resources []runtime.Object
	t.Logf("loading objects from %s", path)
	data, err := loadFile(t, path)
	if err != nil {
		return nil
	}
	rBytes := bytes.Split(data, []byte("---"))
	for _, r := range rBytes {
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, gvk, err := decode(r, nil, nil)
		if err != nil {
			t.Logf("failed to decode resource: %v", err)
			continue
		}
		t.Log(gvk)
		//TODO: add more details
		t.Logf("loaded object %s", gvk.Kind)
		resources = append(resources, obj)
	}
	return resources

}

func loadPolicy(t *testing.T, path string) *kyverno.Policy {
	t.Logf("loading policy from %s", path)
	data, err := loadFile(t, path)
	if err != nil {
		return nil
	}
	var policies []*kyverno.Policy
	pBytes := bytes.Split(data, []byte("---"))
	for _, p := range pBytes {
		policy := kyverno.Policy{}
		pBytes, err := apiyaml.ToJSON(p)
		if err != nil {
			glog.Error(err)
			continue
		}

		if err := json.Unmarshal(pBytes, &policy); err != nil {
			t.Logf("failed to marshall polic. %v", err)
			continue
		}
		t.Logf("loaded policy %s", policy.Name)
		policies = append(policies, &policy)
	}

	if len(policies) == 0 {
		t.Log("no policies loaded")
		return nil
	}
	if len(policies) > 1 {
		t.Log("more than one policy defined, considering first for processing")
	}
	return policies[0]
}
