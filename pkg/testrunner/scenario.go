package testrunner

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	ospath "path"
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

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
	Generation sGeneration `yaml:"generation,omitempty"`
}

type sMutation struct {
	// path to the patched resource to be compared with
	PatchedResource string `yaml:"patchedresource,omitempty"`
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
}

type sValidation struct {
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
}

type sGeneration struct {
	// generated resources
	GeneratedResources []kyverno.ResourceSpec `yaml:"generatedResources"`
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
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
	// load test cases separated by '---'
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

func runScenario(t *testing.T, s *scenarioT) bool {
	for _, tc := range s.testCases {
		runTestCase(t, tc)
	}
	return true
}

func runTestCase(t *testing.T, tc scaseT) bool {
	policy := loadPolicy(t, tc.Input.Policy)
	if policy == nil {
		t.Error("Policy not loaded")
		t.FailNow()
	}

	resource := loadPolicyResource(t, tc.Input.Resource)
	if resource == nil {
		t.Error("Resources not loaded")
		t.FailNow()
	}

	var er response.EngineResponse

	er = engine.Mutate(engine.PolicyContext{Policy: *policy, NewResource: *resource, ExcludeGroupRole: []string{}})
	t.Log("---Mutation---")
	validateResource(t, er.PatchedResource, tc.Expected.Mutation.PatchedResource)
	validateResponse(t, er.PolicyResponse, tc.Expected.Mutation.PolicyResponse)

	// pass the patched resource from mutate to validate
	if len(er.PolicyResponse.Rules) > 0 {
		resource = &er.PatchedResource
	}

	er = engine.Validate(engine.PolicyContext{Policy: *policy, NewResource: *resource, ExcludeGroupRole: []string{}})
	t.Log("---Validation---")
	validateResponse(t, er.PolicyResponse, tc.Expected.Validation.PolicyResponse)

	// Generation
	if resource.GetKind() == "Namespace" {
		// 	generate mock client if resources are to be loaded
		// - create mock client
		// - load resources
		client := getClient(t, tc.Input.LoadResources)
		t.Logf("creating NS %s", resource.GetName())
		if err := createNamespace(client, resource); err != nil {
			t.Error(err)
		} else {
			policyContext := engine.PolicyContext{
				NewResource:      *resource,
				Policy:           *policy,
				Client:           client,
				ExcludeGroupRole: []string{},
			}

			er = engine.Generate(policyContext)
			t.Log(("---Generation---"))
			validateResponse(t, er.PolicyResponse, tc.Expected.Generation.PolicyResponse)
			// Expected generate resource will be in same namespaces as resource
			validateGeneratedResources(t, client, *policy, resource.GetName(), tc.Expected.Generation.GeneratedResources)
		}
	}
	return true
}

func createNamespace(client *client.Client, ns *unstructured.Unstructured) error {
	_, err := client.CreateResource("", "Namespace", "", ns, false)
	return err
}
func validateGeneratedResources(t *testing.T, client *client.Client, policy kyverno.ClusterPolicy, namespace string, expected []kyverno.ResourceSpec) {
	t.Log("--validate if resources are generated---")
	// list of expected generated resources
	for _, resource := range expected {
		if _, err := client.GetResource("", resource.Kind, namespace, resource.Name); err != nil {
			t.Errorf("generated resource %s/%s/%s not found. %v", resource.Kind, namespace, resource.Name, err)
		}
	}
}

func validateResource(t *testing.T, responseResource unstructured.Unstructured, expectedResourceFile string) {
	resourcePrint := func(obj unstructured.Unstructured, msg string) {
		t.Logf("-----%s----", msg)
		if data, err := obj.MarshalJSON(); err == nil {
			t.Log(string(data))
		}
	}
	if expectedResourceFile == "" {
		t.Log("expected resource file not specified, wont compare resources")
		return
	}
	// load expected resource
	expectedResource := loadPolicyResource(t, expectedResourceFile)
	if expectedResource == nil {
		t.Logf("failed to get the expected resource: %s", expectedResourceFile)
		return
	}

	resourcePrint(responseResource, "response resource")
	resourcePrint(*expectedResource, "expected resource")
	// compare the resources
	if !reflect.DeepEqual(responseResource, *expectedResource) {
		t.Error("failed: response resource returned does not match expected resource")
		return
	}
	t.Log("success: response resource returned matches expected resource")
}

func validateResponse(t *testing.T, er response.PolicyResponse, expected response.PolicyResponse) {
	if reflect.DeepEqual(expected, (response.PolicyResponse{})) {
		t.Log("no response expected")
		return
	}
	// cant do deepEquals and the stats will be different, or we nil those fields and then do a comparison
	// forcing only the fields that are specified to be comprared
	// doing a field by fields comparison will allow us to provied more details logs and granular error reporting
	// check policy name is same :P
	if er.Policy != expected.Policy {
		t.Errorf("Policy name: expected %s, received %s", expected.Policy, er.Policy)
	}
	// compare resource spec
	compareResourceSpec(t, er.Resource, expected.Resource)
	// //TODO stats
	// if er.RulesAppliedCount != expected.RulesAppliedCount {
	// 	t.Log("RulesAppliedCount: error")
	// }

	// rules
	if len(er.Rules) != len(expected.Rules) {
		t.Errorf("rule count error, er.Rules=%d, expected.Rules=%d", len(er.Rules), len(expected.Rules))
		return
	}
	if len(er.Rules) == len(expected.Rules) {
		// if there are rules being applied then we compare the rule response
		// as the rules are applied in the order defined, the comparison of rules will be in order
		for index, r := range expected.Rules {
			compareRules(t, er.Rules[index], r)
		}
	}
}

func compareResourceSpec(t *testing.T, resource response.ResourceSpec, expectedResource response.ResourceSpec) {
	// kind
	if resource.Kind != expectedResource.Kind {
		t.Errorf("kind: expected %s, received %s", expectedResource.Kind, resource.Kind)
	}
	// //TODO apiVersion
	// if resource.APIVersion != expectedResource.APIVersion {
	// 	t.Error("error: apiVersion")
	// }

	// namespace
	if resource.Namespace != expectedResource.Namespace {
		t.Errorf("namespace: expected %s, received %s", expectedResource.Namespace, resource.Namespace)
	}
	// name
	if resource.Name != expectedResource.Name {
		t.Errorf("name: expected %s, received %s", expectedResource.Name, resource.Name)
	}
}

func compareRules(t *testing.T, rule response.RuleResponse, expectedRule response.RuleResponse) {
	// name
	if rule.Name != expectedRule.Name {
		t.Errorf("rule name: expected %s, received %+v", expectedRule.Name, rule.Name)
		// as the rule names dont match no need to compare the rest of the information
	}
	// type
	if rule.Type != expectedRule.Type {
		t.Errorf("rule type: expected %s, received %s", expectedRule.Type, rule.Type)
	}
	// message
	// compare messages if expected rule message is not empty
	if expectedRule.Message != "" && rule.Message != expectedRule.Message {
		t.Errorf("rule message: expected %s, received %s", expectedRule.Message, rule.Message)
	}
	// //TODO patches
	// if reflect.DeepEqual(rule.Patches, expectedRule.Patches) {
	// 	t.Log("error: patches")
	// }

	// success
	if rule.Success != expectedRule.Success {
		t.Errorf("rule success: expected %t, received %t", expectedRule.Success, rule.Success)
	}
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
	var objects []runtime.Object
	if files != nil {

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
	// get GVR from GVK
	gvrs := getGVRForResources(objects)
	c.SetDiscovery(client.NewFakeDiscoveryClient(gvrs))
	t.Log("created mock client with pre-loaded resources")
	return c
}

func getGVRForResources(objects []runtime.Object) []schema.GroupVersionResource {
	var gvrs []schema.GroupVersionResource
	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		gv := gvk.GroupVersion()
		// maintain a static map for kind -> Resource
		gvr := gv.WithResource(getResourceFromKind(gvk.Kind))
		gvrs = append(gvrs, gvr)
	}
	return gvrs
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
		obj, _, err := decode(r, nil, nil)
		if err != nil {
			t.Logf("failed to decode resource: %v", err)
			continue
		}

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

func loadPolicy(t *testing.T, path string) *kyverno.ClusterPolicy {
	t.Logf("loading policy from %s", path)
	data, err := loadFile(t, path)
	if err != nil {
		return nil
	}
	var policies []*kyverno.ClusterPolicy
	pBytes := bytes.Split(data, []byte("---"))
	for _, p := range pBytes {
		policy := kyverno.ClusterPolicy{}
		pBytes, err := apiyaml.ToJSON(p)
		if err != nil {
			t.Error(err)
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

func testScenario(t *testing.T, path string) {

	// flag.Set("logtostderr", "true")
	// flag.Set("v", "8")

	scenario, err := loadScenario(t, path)
	if err != nil {
		t.Error(err)
		return
	}

	runScenario(t, scenario)
}
