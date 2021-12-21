package testrunner

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	ospath "path"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"

	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

type Scenario struct {
	TestCases []TestCase
}

// TestCase defines input and output for a case
type TestCase struct {
	Input    Input    `yaml:"input"`
	Expected Expected `yaml:"expected"`
}

// Input defines input for a test scenario
type Input struct {
	Policy        string   `yaml:"policy"`
	Resource      string   `yaml:"resource"`
	LoadResources []string `yaml:"loadresources,omitempty"`
}

type Expected struct {
	Mutation   Mutation   `yaml:"mutation,omitempty"`
	Validation Validation `yaml:"validation,omitempty"`
	Generation Generation `yaml:"generation,omitempty"`
}

type Mutation struct {
	// path to the patched resource to be compared with
	PatchedResource string `yaml:"patchedresource,omitempty"`
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
}

type Validation struct {
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
}

type Generation struct {
	// generated resources
	GeneratedResources []kyverno.ResourceSpec `yaml:"generatedResources"`
	// expected response from the policy engine
	PolicyResponse response.PolicyResponse `yaml:"policyresponse"`
}

// RootDir returns the kyverno project directory based on the location of the current file.
// It assumes that the project directory is 2 levels up. This means if this function is moved
// it may not work as expected.
func RootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	d = filepath.Dir(d)
	return filepath.Dir(d)
}

//getRelativePath expects a path relative to project and builds the complete path
func getRelativePath(path string) string {
	root := RootDir()
	return ospath.Join(root, path)
}

func loadScenario(t *testing.T, path string) (*Scenario, error) {
	fileBytes, err := loadFile(t, path)
	assert.Nil(t, err)

	var testCases []TestCase
	// load test cases separated by '---'
	// each test case defines an input & expected result
	scenariosBytes := bytes.Split(fileBytes, []byte("---"))
	for _, testCaseBytes := range scenariosBytes {
		var tc TestCase
		if err := yaml.Unmarshal(testCaseBytes, &tc); err != nil {
			t.Errorf("failed to decode test case YAML: %v", err)
			continue
		}

		testCases = append(testCases, tc)
	}

	scenario := Scenario{
		TestCases: testCases,
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
	path = filepath.Clean(path)
	// We accept the risk of including a user provided file here.
	return ioutil.ReadFile(path) // #nosec G304
}

func runScenario(t *testing.T, s *Scenario) bool {
	for _, tc := range s.TestCases {
		runTestCase(t, tc)
	}
	return true
}

func runTestCase(t *testing.T, tc TestCase) bool {
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

	ctx := &engine.PolicyContext{
		Policy:           *policy,
		NewResource:      *resource,
		ExcludeGroupRole: []string{},
		JSONContext:      context.NewContext(),
	}

	er := engine.Mutate(ctx)
	t.Log("---Mutation---")
	validateResource(t, er.PatchedResource, tc.Expected.Mutation.PatchedResource)
	validateResponse(t, er.PolicyResponse, tc.Expected.Mutation.PolicyResponse)

	// pass the patched resource from mutate to validate
	if len(er.PolicyResponse.Rules) > 0 {
		resource = &er.PatchedResource
	}

	ctx = &engine.PolicyContext{
		Policy:           *policy,
		NewResource:      *resource,
		ExcludeGroupRole: []string{},
		JSONContext:      context.NewContext(),
	}

	er = engine.Validate(ctx)
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
			policyContext := &engine.PolicyContext{
				NewResource:      *resource,
				Policy:           *policy,
				Client:           client,
				ExcludeGroupRole: []string{},
				ExcludeResourceFunc: func(s1, s2, s3 string) bool {
					return false
				},
				JSONContext: context.NewContext(),
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

	// compare the resources
	if !reflect.DeepEqual(responseResource, *expectedResource) {
		t.Error("failed: response resource returned does not match expected resource")
		resourcePrint(responseResource, "response resource")
		resourcePrint(*expectedResource, "expected resource")
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
	// compare policy spec
	comparePolicySpec(t, er.Policy, expected.Policy)
	// compare resource spec
	compareResourceSpec(t, er.Resource, expected.Resource)
	// //TODO stats
	// if er.RulesAppliedCount != expected.RulesAppliedCount {
	// 	t.Log("RulesAppliedCount: error")
	// }

	// rules
	if len(er.Rules) != len(expected.Rules) {
		t.Errorf("rule count error, er.Rules=%v, expected.Rules=%v", er.Rules, expected.Rules)
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

func comparePolicySpec(t *testing.T, policy response.PolicySpec, expectedPolicy response.PolicySpec) {
	// namespace
	if policy.Namespace != expectedPolicy.Namespace {
		t.Errorf("namespace: expected %s, received %s", expectedPolicy.Namespace, policy.Namespace)
	}
	// name
	if policy.Name != expectedPolicy.Name {
		t.Errorf("name: expected %s, received %s", expectedPolicy.Name, policy.Name)
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
	if rule.Status != expectedRule.Status {
		t.Errorf("rule status mismatch: expected %s, received %s", expectedRule.Status.String(), rule.Status.String())
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

	for _, r := range resources {
		metadata := r.UnstructuredContent()["metadata"].(map[string]interface{})
		delete(metadata, "creationTimestamp")
	}

	return resources[0]
}

func getClient(t *testing.T, files []string) *client.Client {
	var objects []k8sRuntime.Object
	if files != nil {

		for _, file := range files {
			objects = loadObjects(t, file)
		}
	}
	// create mock client
	scheme := k8sRuntime.NewScheme()
	// mock client expects the resource to be as runtime.Object
	c, err := client.NewMockClient(scheme, nil, objects...)
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

func getGVRForResources(objects []k8sRuntime.Object) []schema.GroupVersionResource {
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

		data, err := k8sRuntime.DefaultUnstructuredConverter.ToUnstructured(&obj)
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

func loadObjects(t *testing.T, path string) []k8sRuntime.Object {
	var resources []k8sRuntime.Object
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
