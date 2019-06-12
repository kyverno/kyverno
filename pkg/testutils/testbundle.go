package testutils

import (
	"fmt"
	"os"
	ospath "path"
	"testing"

	"github.com/golang/glog"
	policytypes "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
)

func NewTestBundle(path string) *testBundle {
	return &testBundle{
		path:          path,
		policies:      make(map[string]*policytypes.Policy),
		resources:     make(map[string]*resourceInfo),
		initResources: make(map[string]*resourceInfo),
	}
}

func (tb *testBundle) load() error {
	// check if resources folder is defined
	rpath := ospath.Join(tb.path, resourcesFolder)
	_, err := os.Stat(rpath)
	if os.IsNotExist(err) {
		glog.Warningf("Resources directory not present at %s", tb.path)
		return fmt.Errorf("Resources directory not present at %s", tb.path)
	}

	// check if scenario yaml is defined
	spath := ospath.Join(tb.path, tScenarioFile)
	_, err = os.Stat(spath)
	if os.IsNotExist(err) {
		return fmt.Errorf("Scenario file %s not defined at %s", tScenarioFile, tb.path)
	}
	tb.scenarios, err = LoadScenarios(spath)
	if err != nil {
		return err
	}
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

	// extract resources
	rYAMLs := getYAMLfiles(rpath)
	if len(rYAMLs) == 0 {
		return fmt.Errorf("No resource yaml found at path %s", rpath)
	}
	for _, r := range rYAMLs {
		resources, err := extractResource(r)
		if err != nil {
			glog.Errorf("unable to extract resource: %s", err)
		}
		tb.mergeResources(resources)
	}
	return nil
}

func (tb *testBundle) mergeResources(rs map[string]*resourceInfo) {
	for k, v := range rs {
		if _, ok := tb.resources[k]; ok {
			glog.Infof("resource already defined %s ", k)
			continue
		}
		tb.resources[k] = v
	}
}

type testBundle struct {
	path          string
	policies      map[string]*policytypes.Policy
	resources     map[string]*resourceInfo
	initResources map[string]*resourceInfo
	scenarios     []*tScenario
}

func (tb *testBundle) run(t *testing.T, testingapplyTest IApplyTest) {
	// run each scenario
	for _, ts := range tb.scenarios {
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
		mPatchedResource, mResult, vResult, err := testingapplyTest.applyPolicy(p, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		// check the expected scenario
		tb.checkMutationResult(t, ts.Mutation, mPatchedResource, mResult)
		tb.checkValidationResult(t, ts.Validation, vResult)
	}
}

func (tb *testBundle) checkValidationResult(t *testing.T, expect *tValidation, vResult result.Result) {
	if expect == nil {
		glog.Info("No Validation check defined")
		return
	}
	// compare result
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
	pr, ok := tb.resources[expect.MPatchedResource]
	if !ok {
		glog.Warningf("Resource %s not found", expect.MPatchedResource)
		return
	}
	// compare patched resources
	if !checkMutationRPatches(pr, pr) {
		t.Error("Patched resources not as expected")
	}
	// compare result
	// compare reason
	if len(expect.Reason) > 0 && expect.Reason != mResult.GetReason().String() {
		t.Error("Reason not matching")
	}
	// compare message
	if len(expect.Message) > 0 && expect.Message != mResult.String() {
		t.Error(("Message not matching"))
	}
}
