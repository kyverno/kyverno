package testrunner

import (
	"fmt"
	"testing"

	ospath "path"

	"github.com/golang/glog"
	pt "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/result"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

type test struct {
	ap       string
	t        *testing.T
	testCase *testCase
	// input
	policy        *pt.Policy
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
		_, err = client.CreateResource(getResourceFromKind(t.tResource.gvk.Kind), "", obj, false)
		if err != nil {
			t.t.Errorf("error while creating namespace %s", err)
		}

	}
	// apply the policy engine
	pr, mResult, vResult, err := t.applyPolicy(t.policy, t.tResource, client)
	if err != nil {
		t.t.Error(err)
		return
	}
	// Expected Result
	t.checkMutationResult(pr, mResult)
	t.checkValidationResult(vResult)
	t.checkGenerationResult(client)
}

func (t *test) checkMutationResult(pr *resourceInfo, result result.Result) {
	if t.testCase.Expected.Mutation == nil {
		glog.Info("No Mutation check defined")
		return
	}
	// patched resource
	if !compareResource(pr, t.patchedResource) {
		fmt.Printf("Expected Resource %s \n", string(t.patchedResource.rawResource))
		fmt.Printf("Patched Resource %s \n", string(pr.rawResource))
		glog.Warningf("Expected resource %s ", string(pr.rawResource))
		t.t.Error("Patched resources not as expected")
	}
	// reason
	reason := t.testCase.Expected.Mutation.Reason
	if len(reason) > 0 && result.GetReason().String() != reason {
		t.t.Error("Reason not matching")
	}
}

func (t *test) checkValidationResult(result result.Result) {
	if t.testCase.Expected.Validation == nil {
		glog.Info("No Validation check defined")
		return
	}
	// reason
	reason := t.testCase.Expected.Validation.Reason
	if len(reason) > 0 && result.GetReason().String() != reason {
		t.t.Error("Reason not matching")
	}
}

func (t *test) checkGenerationResult(client *client.Client) {
	if t.testCase.Expected.Generation == nil {
		glog.Info("No Generate check defined")
		return
	}
	if client == nil {
		glog.Info("client needs to be configured")
	}
	// check if the expected resources are generated
	for _, r := range t.genResources {
		n := ParseNameFromObject(r.rawResource)
		ns := ParseNamespaceFromObject(r.rawResource)
		_, err := client.GetResource(getResourceFromKind(r.gvk.Kind), ns, n)
		if err != nil {
			t.t.Errorf("Resource %s/%s of kinf %s not found", ns, n, r.gvk.Kind)
		}
		// compare if the resources are same
		//TODO: comapre []bytes vs unstrcutured resource
	}
}

func (t *test) applyPolicy(policy *pt.Policy,
	tresource *resourceInfo,
	client *client.Client) (*resourceInfo, result.Result, result.Result, error) {
	// apply policy on the trigger resource
	// Mutate
	var vResult result.Result
	var patchedResource []byte
	mPatches, mResult := engine.Mutate(*policy, tresource.rawResource, *tresource.gvk)
	// TODO: only validate if there are no errors in mutate, why?
	err := mResult.ToError()
	if err == nil && len(mPatches) != 0 {
		patchedResource, err = engine.ApplyPatches(tresource.rawResource, mPatches)
		if err != nil {
			return nil, nil, nil, err
		}
		// Validate
		vResult = engine.Validate(*policy, patchedResource, *tresource.gvk)
	}
	// Generate
	if client != nil {
		engine.Generate(client, *policy, tresource.rawResource, *tresource.gvk)
	}
	// transform the patched Resource into resource Info
	ri, err := extractResourceRaw(patchedResource)
	if err != nil {
		return nil, nil, nil, err
	}
	// return the results
	return ri, mResult, vResult, nil
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
