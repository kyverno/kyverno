package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	ospath "path"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/golang/glog"
	policytypes "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	defaultYamlSeparator = "---"
)

func loadFile(fileDir string) ([]byte, error) {
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		return nil, err
	}
	return ioutil.ReadFile(fileDir)
}

func extractPolicy(fileDir string) (*policytypes.Policy, error) {
	policy := &policytypes.Policy{}

	file, err := loadFile(fileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load file: %v", err)
	}

	policyBytes, err := kyaml.ToJSON(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(policyBytes, policy); err != nil {
		return nil, fmt.Errorf("failed to decode policy %s, err: %v", policy.Name, err)
	}

	if policy.TypeMeta.Kind != "Policy" {
		return nil, fmt.Errorf("failed to parse policy")
	}

	return policy, nil
}

type resourceInfo struct {
	rawResource []byte
	gvk         *metav1.GroupVersionKind
}

func (ri resourceInfo) isSame(other resourceInfo) bool {
	// compare gvk
	if *ri.gvk != *other.gvk {
		return false
	}
	// compare rawResource
	return bytes.Equal(ri.rawResource, other.rawResource)
}

func getResourceYAML(d []byte) {
	// fmt.Println(string(d))
	// convert json to yaml
	// print the result for reference
	// can be used as a dry run the get the expected result
}

func extractResourceRaw(d []byte) (string, *resourceInfo) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, gvk, err := decode([]byte(d), nil, nil)
	if err != nil {
		glog.Warningf("Error while decoding YAML object, err: %s\n", err)
		return "", nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		glog.Warningf("Error while marshalling manifest, err: %v\n", err)
		return "", nil
	}
	gvkInfo := &metav1.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind}
	rn := ParseNameFromObject(raw)
	rns := ParseNamespaceFromObject(raw)
	if rns != "" {
		rn = rns + "/" + rn
	}
	return rn, &resourceInfo{rawResource: raw, gvk: gvkInfo}
}

func extractResource(resource string) (map[string]*resourceInfo, error) {
	resources := make(map[string]*resourceInfo)
	data, err := loadFile(resource)
	if err != nil {
		glog.Warningf("Error while loading file: %v\n", err)
		return nil, err
	}
	dd := bytes.Split(data, []byte(defaultYamlSeparator))
	for _, d := range dd {
		rn, r := extractResourceRaw(d)
		resources[rn] = r
	}
	return resources, nil
}

//ParseNameFromObject extracts resource name from JSON obj
func ParseNameFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if name, ok := meta["name"].(string); ok {
		return name
	}
	return ""
}

// ParseNamespaceFromObject extracts the namespace from the JSON obj
func ParseNamespaceFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if namespace, ok := meta["namespace"].(string); ok {
		return namespace
	}
	return ""
}

type IApplyTest interface {
	applyPolicy(policy *policytypes.Policy, resource *resourceInfo, client *client.Client) (*resourceInfo, result.Result, result.Result, error)
}

type testPolicy struct {
}

func (tp *testPolicy) applyPolicy(policy *policytypes.Policy, resource *resourceInfo, client *client.Client) (*resourceInfo, result.Result, result.Result, error) {
	// apply policy on the trigger resource
	// Mutate
	var vResult result.Result
	var patchedResource []byte
	mPatches, mResult := engine.Mutate(*policy, resource.rawResource, *resource.gvk)
	// TODO: only validate if there are no errors in mutate, why?
	err := mResult.ToError()
	if err == nil && len(mPatches) != 0 {
		patchedResource, err = engine.ApplyPatches(resource.rawResource, mPatches)
		if err != nil {
			return nil, nil, nil, err
		}
		// Validate
		vResult = engine.Validate(*policy, patchedResource, *resource.gvk)
	}
	// Generate
	if client == nil {
		glog.Warning("Client is required to test generate")
	}

	// transform the patched Resource into resource Info
	_, ri := extractResourceRaw(patchedResource)
	// return the results
	return ri, mResult, vResult, nil
	// TODO: merge the results for mutation and validation
}

type tScenario struct {
	Policy        string       `yaml:"policy"`
	Resource      string       `yaml:"resource"`
	InitResources []string     `yaml:"initResources,omitempty"`
	Mutation      *tMutation   `yaml:"mutation,omitempty"`
	Validation    *tValidation `yaml:"validation,omitempty"`
}

type tValidation struct {
	Reason  string `yaml:"reason,omitempty"`
	Message string `yaml:"message,omitempty"`
	Error   string `yaml:"error,omitempty"`
}

type tMutation struct {
	MPatchedResource string `yaml:"mPatchedResource,omitempty"`
	Reason           string `yaml:"reason,omitempty"`
	Message          string `yaml:"message,omitempty"`
	Error            string `yaml:"error,omitempty"`
}

func LoadScenarios(file string) ([]*tScenario, error) {
	ts := []*tScenario{}
	// read the file
	data, err := loadFile(file)
	if err != nil {
		glog.Warningf("Error while loading file: %v\n", err)
		return nil, err
	}
	dd := bytes.Split(data, []byte(defaultYamlSeparator))
	for _, d := range dd {
		s := &tScenario{}
		err := yaml.Unmarshal([]byte(d), s)
		if err != nil {
			glog.Warningf("Error while decoding YAML object, err: %s", err)
			continue
		}
		ts = append(ts, s)
	}
	return ts, nil
}

// Load policy & resource files
// engine pass the (policy, resource)
// check the expected response

const examplesPath string = "examples"
const resourcesFolder string = "resources"
const tScenarioFile string = "testScenarios.yaml"
const outputFolder string = "output"

//LoadTestSuite  reads the resource, policy and scenario files
func LoadTestSuite(t *testing.T, path string) *testSuite {
	glog.Infof("loading test suites at %s", path)
	// gp := os.Getenv("GOPATH")
	// ap := ospath.Join(gp, "src/github.com/nirmata/kyverno")
	// build test suite
	// each suite contains test bundles for test sceanrios
	// ts := NewTestSuite(t, ospath.Join(ap, examplesPath))
	ts := NewTestSuite(t, path)
	ts.buildTestSuite()
	glog.Infof("done loading test suite at %s", path)
	return ts
}

func checkMutationRPatches(er *resourceInfo, pr *resourceInfo) bool {
	if !er.isSame(*pr) {
		getResourceYAML(pr.rawResource)
		return false
	}
	return true
}

func getYAMLfiles(path string) (yamls []string) {
	fileInfo, err := ioutil.ReadDir(path)
	if err != nil {
		return nil
	}
	for _, file := range fileInfo {
		if file.Name() == tScenarioFile {
			continue
		}
		if filepath.Ext(file.Name()) == ".yml" || filepath.Ext(file.Name()) == ".yaml" {
			yamls = append(yamls, ospath.Join(path, file.Name()))
		}
	}
	return yamls
}
