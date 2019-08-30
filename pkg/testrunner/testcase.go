package testrunner

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	ospath "path"

// 	"github.com/golang/glog"
// 	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	yaml "k8s.io/apimachinery/pkg/util/yaml"
// 	"k8s.io/client-go/kubernetes/scheme"
// )

// //testCase defines the input and the expected result
// // it stores the path to the files that are to be loaded
// // for references
// type testCase struct {
// 	Input    *tInput    `yaml:"input"`
// 	Expected *tExpected `yaml:"expected"`
// }

// // load resources store the resources that are pre-requisite
// // for the test case and are pre-loaded in the client before
// /// test case in evaluated
// type tInput struct {
// 	Policy        string   `yaml:"policy"`
// 	Resource      string   `yaml:"resource"`
// 	LoadResources []string `yaml:"load_resources,omitempty"`
// }

// type tExpected struct {
// 	Passes     string       `yaml:"passes"`
// 	Mutation   *tMutation   `yaml:"mutation,omitempty"`
// 	Validation *tValidation `yaml:"validation,omitempty"`
// 	Generation *tGeneration `yaml:"generation,omitempty"`
// }

// type tMutation struct {
// 	PatchedResource string   `yaml:"patched_resource,omitempty"`
// 	Rules           []tRules `yaml:"rules"`
// }

// type tValidation struct {
// 	Rules []tRules `yaml:"rules"`
// }

// type tGeneration struct {
// 	Resources []string `yaml:"resources"`
// 	Rules     []tRules `yaml:"rules"`
// }

// type tRules struct {
// 	Name     string   `yaml:"name"`
// 	Type     string   `yaml:"type"`
// 	Messages []string `yaml:"messages"`
// }

// type tResult struct {
// 	Reason string `yaml:"reason, omitempty"`
// }

// func (tc *testCase) policyEngineTest() {

// }
// func (tc *testCase) loadPreloadedResources(ap string) ([]*resourceInfo, error) {
// 	return loadResources(ap, tc.Input.LoadResources...)
// 	//	return loadResources(ap, tc.Input.LoadResources...)
// }

// func (tc *testCase) loadGeneratedResources(ap string) ([]*resourceInfo, error) {
// 	if tc.Expected.Generation == nil {
// 		return nil, nil
// 	}
// 	return loadResources(ap, tc.Expected.Generation.Resources...)
// }

// func (tc *testCase) loadPatchedResource(ap string) (*resourceInfo, error) {
// 	if tc.Expected.Mutation == nil {
// 		return nil, nil
// 	}
// 	rs, err := loadResources(ap, tc.Expected.Mutation.PatchedResource)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(rs) != 1 {
// 		glog.Warning("expects single resource mutation but multiple defined, will use first one")
// 	}
// 	return rs[0], nil

// }
// func (tc *testCase) loadResources(files []string) ([]*resourceInfo, error) {
// 	lr := []*resourceInfo{}
// 	for _, r := range files {
// 		rs, err := loadResources(r)
// 		if err != nil {
// 			// return as test case will be invalid if a resource cannot be loaded
// 			return nil, err
// 		}
// 		lr = append(lr, rs...)
// 	}
// 	return lr, nil
// }

// func (tc *testCase) loadTriggerResource(ap string) (*resourceInfo, error) {
// 	rs, err := loadResources(ap, tc.Input.Resource)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(rs) != 1 {
// 		glog.Warning("expects single resource trigger but multiple defined, will use first one")
// 	}
// 	return rs[0], nil

// }

// // Loads a single policy
// func (tc *testCase) loadPolicy(file string) (*kyverno.Policy, error) {
// 	p := &kyverno.Policy{}
// 	data, err := LoadFile(file)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pBytes, err := yaml.ToJSON(data)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err := json.Unmarshal(pBytes, p); err != nil {
// 		return nil, err
// 	}
// 	if p.TypeMeta.Kind != "Policy" {
// 		return nil, fmt.Errorf("failed to parse policy")
// 	}
// 	return p, nil
// }

// // loads multiple resources
// func loadResources(ap string, args ...string) ([]*resourceInfo, error) {
// 	ris := []*resourceInfo{}
// 	for _, file := range args {
// 		data, err := LoadFile(ospath.Join(ap, file))
// 		if err != nil {
// 			return nil, err
// 		}
// 		dd := bytes.Split(data, []byte(defaultYamlSeparator))
// 		// resources seperated by yaml seperator
// 		for _, d := range dd {
// 			ri, err := extractResourceRaw(d)
// 			if err != nil {
// 				glog.Errorf("unable to load resource. err: %s ", err)
// 				continue
// 			}
// 			ris = append(ris, ri)
// 		}
// 	}
// 	return ris, nil
// }

// func extractResourceRaw(d []byte) (*resourceInfo, error) {
// 	// decode := scheme.Codecs.UniversalDeserializer().Decode
// 	// obj, gvk, err := decode(d, nil, nil)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	obj, gvk, err := extractResourceUnMarshalled(d)
// 	// runtime.object to JSON
// 	raw, err := json.Marshal(obj)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &resourceInfo{rawResource: raw,
// 		gvk: gvk}, nil
// }

// func extractResourceUnMarshalled(d []byte) (runtime.Object, *metav1.GroupVersionKind, error) {
// 	decode := scheme.Codecs.UniversalDeserializer().Decode
// 	obj, gvk, err := decode(d, nil, nil)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	return obj, &metav1.GroupVersionKind{Group: gvk.Group,
// 		Version: gvk.Version,
// 		Kind:    gvk.Kind}, nil
// }
