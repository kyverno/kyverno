package testutils

import (
	"bytes"
	"fmt"
	"os"
	ospath "path"
	"testing"

	"github.com/golang/glog"
	policytypes "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/result"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewTestBundle(path string) *testBundle {
	return &testBundle{
		path:      path,
		policies:  make(map[string]*policytypes.Policy),
		resources: make(map[string]*resourceInfo),
		output:    make(map[string]*resourceInfo),
	}
}

func loadResources(tbPath string, rs map[string]*resourceInfo, file string) {
	path := ospath.Join(tbPath, file)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		glog.Warningf("%s directory not present at %s", file, tbPath)
		return
	}
	// Load the resources from the output folder
	yamls := getYAMLfiles(path)
	if len(yamls) == 0 {
		glog.Warningf("No resource yaml found at path %s", path)
		return
	}
	for _, r := range yamls {
		resources, err := extractResource(r)
		if err != nil {
			glog.Errorf("unable to extract resource: %s", err)
		}
		mergeResources(rs, resources)
	}
}

func (tb *testBundle) loadOutput() {
	// check if output folder is defined
	opath := ospath.Join(tb.path, outputFolder)
	_, err := os.Stat(opath)
	if os.IsNotExist(err) {
		glog.Warningf("Output directory not present at %s", tb.path)
		return
	}
	// Load the resources from the output folder
	oYAMLs := getYAMLfiles(opath)
	if len(oYAMLs) == 0 {
		glog.Warningf("No resource yaml found at path %s", opath)
		return
	}

	for _, r := range oYAMLs {
		resources, err := extractResource(r)
		if err != nil {
			glog.Errorf("unable to extract resource: %s", err)
		}
		mergeResources(tb.output, resources)
	}
}

func loadScenarios(tbPath string, file string) ([]*tScenario, error) {
	// check if scenario yaml is defined
	spath := ospath.Join(tbPath, file)
	_, err := os.Stat(spath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("Scenario file %s not defined at %s", file, tbPath)
	}
	ts := []*tScenario{}
	// read the file
	data, err := loadFile(spath)
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

// Load test structure folder
func (tb *testBundle) load() error {
	// scenario file defines the mapping of resources and policies
	scenarios, err := loadScenarios(tb.path, tScenarioFile)
	if err != nil {
		return err
	}
	tb.scenarios = scenarios
	// check if there are any files
	pYAMLs := getYAMLfiles(tb.path)
	if len(pYAMLs) == 0 {
		return fmt.Errorf("No policy yaml found at path %s", tb.path)
	}
	for _, p := range pYAMLs {
		// extract policy
		policy, err := extractPolicy(p)
		if err != nil {
			glog.Errorf("unable to extract policy: %s", err)
			continue
		}
		tb.policies[policy.GetName()] = policy
	}
	// load trigger resources
	loadResources(tb.path, tb.resources, resourcesFolder)
	// load output resources
	loadResources(tb.path, tb.output, outputFolder)

	return nil
}

func mergeResources(rs map[string]*resourceInfo, other map[string]*resourceInfo) {
	for k, v := range other {
		if _, ok := rs[k]; ok {
			glog.Infof("resource already defined %s ", k)
			continue
		}
		rs[k] = v
	}
}

type testBundle struct {
	path      string
	policies  map[string]*policytypes.Policy
	resources map[string]*resourceInfo
	output    map[string]*resourceInfo
	scenarios []*tScenario
}

func (tb *testBundle) createClient(t *testing.T, resources []string) *dclient.Client {
	scheme := runtime.NewScheme()
	objects := []runtime.Object{}
	// registered group versions
	regResources := []schema.GroupVersionResource{}
	for _, resource := range resources {
		// get resources
		r, ok := tb.resources[resource]
		if !ok {
			glog.Warningf("Resource %s not found", resource)
			continue
		}
		// get group version resource
		gv := schema.GroupVersion{Group: r.gvk.Group, Version: r.gvk.Version}
		gvr := gv.WithResource(getResourceFromKind(r.gvk.Kind))
		regResources = append(regResources, gvr)

		decode := kscheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(r.rawResource), nil, nil)
		if err != nil {
			glog.Warning("Unable to deocde")
			continue
		}
		// create unstructured
		rdata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
		if err != nil {
			fmt.Println(err)
			continue
		}
		unstr := unstructured.Unstructured{Object: rdata}
		objects = append(objects, &unstr)
	}
	// new mock client
	// Mock Client
	c, err := dclient.NewMockClient(scheme, objects...)
	if err != nil {
		t.Error(err)
	}
	// set discovery Client
	c.SetDiscovery(dclient.NewFakeDiscoveryClient(regResources))

	return c
}

func (tb *testBundle) run(t *testing.T, testingapplyTest IApplyTest) {
	glog.Infof("Start: test on test bundles %s", tb.path)
	// run each scenario
	for _, ts := range tb.scenarios {
		// TODO create client only for generate
		// If there are init resources defined then load them
		c := tb.createClient(t, ts.InitResources)
		// get policy
		p, ok := tb.policies[ts.Policy]
		if !ok {
			glog.Warningf("Policy %s not found", ts.Policy)
			continue
		}
		// get resources
		r, ok := tb.resources[ts.Resource]
		if !ok {
			glog.Warningf("Resource %s not found", ts.Resource)
			continue
		}
		// TODO: handle generate
		if ts.Generation != nil {
			// assuming its namespaces creation
			decode := kscheme.Codecs.UniversalDeserializer().Decode
			obj, _, err := decode([]byte(r.rawResource), nil, nil)
			_, err = c.CreateResource(getResourceFromKind(r.gvk.Kind), "", obj)
			if err != nil {
				t.Errorf("error while creating namespace %s", ts.Resource)
			}
		}

		mPatchedResource, mResult, vResult, err := testingapplyTest.applyPolicy(p, r, c)
		if err != nil {
			t.Error(err)
		}
		// check the expected scenario
		tb.checkMutationResult(t, ts.Mutation, mPatchedResource, mResult)
		tb.checkValidationResult(t, ts.Validation, vResult)
		tb.checkGeneration(t, ts.Generation, c)
	}
	glog.Infof("Done: test on test bundles %s", tb.path)
}

func (tb *testBundle) checkGeneration(t *testing.T, expect *tGeneration, c *dclient.Client) {
	if expect == nil {
		glog.Info("No Generate check defined")
		return
	}
	// iterate throught the expected resources and check if the client has them
	for _, r := range expect.Resources {
		_, err := c.GetResource(getResourceFromKind(r.Kind), r.Namespace, r.Name)
		if err != nil {
			t.Errorf("Resource %s/%s of kind %s not found", r.Namespace, r.Name, r.Kind)
		}
	}
}

func (tb *testBundle) checkValidationResult(t *testing.T, expect *tValidation, vResult result.Result) {
	if expect == nil {
		glog.Info("No Validation check defined")
		return
	}
	// compare reason
	if len(expect.Reason) > 0 && expect.Reason != vResult.GetReason().String() {
		t.Error("Reason not matching")
	}
	// compare message
	if len(expect.Message) > 0 && expect.Message != vResult.String() {
		t.Error(("Message not matching"))
	}
}

func (tb *testBundle) checkMutationResult(t *testing.T, expect *tMutation, pr *resourceInfo, mResult result.Result) {
	if expect == nil {
		glog.Info("No Mutation check defined")
		return
	}
	// get expected patched resource
	er, ok := tb.output[expect.MPatchedResource]
	if !ok {
		glog.Warningf("Resource %s not found", expect.MPatchedResource)
		return
	}
	// compare patched resources
	if !checkMutationRPatches(pr, er) {
		fmt.Printf("Expected Resource %s \n", string(er.rawResource))
		fmt.Printf("Patched Resource %s \n", string(pr.rawResource))

		glog.Warningf("Expected resource %s ", string(pr.rawResource))
		t.Error("Patched resources not as expected")
	}
	// compare reason
	if len(expect.Reason) > 0 && expect.Reason != mResult.GetReason().String() {
		t.Error("Reason not matching")
	}
	// compare message
	if len(expect.Message) > 0 && expect.Message != mResult.String() {
		t.Error(("Message not matching"))
	}
}
