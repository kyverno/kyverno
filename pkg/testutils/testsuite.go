package testutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/glog"
)

//NewTestSuite returns new test suite
func NewTestSuite(t *testing.T, path string) *testSuite {
	return &testSuite{
		t:    t,
		path: path,
		tb:   []*testBundle{},
	}
}
func (ts *testSuite) runTests() {
	//TODO : make sure the implementation the interface is pointing to is not nil
	if ts.applyTest == nil {
		glog.Error("Apply Test set for the test suite")
		return
	}
	// for each test bundle run the test scenario
	for _, tb := range ts.tb {
		tb.run(ts.t, ts.applyTest)
	}
}
func (ts *testSuite) setApplyTest(applyTest IApplyTest) {
	ts.applyTest = applyTest
}

type testSuite struct {
	t         *testing.T
	path      string
	tb        []*testBundle
	applyTest IApplyTest
}

func (ts *testSuite) buildTestSuite() error {
	// loading test bundles for test suite
	err := filepath.Walk(ts.path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			glog.Infof("searching for test files at %s", path)
			// check if there are resources dir and policies yaml
			tb := NewTestBundle(path)
			if tb != nil {
				// try to load the test folder structure
				err := tb.load()
				if err != nil {
					glog.Warningf("no supported test structure avaialbe at path %s", path)
					return nil
				}
				glog.Infof("loading test suite at path %s", path)
				ts.tb = append(ts.tb, tb)
			}
		}
		return nil
	})
	if err != nil {
		ts.t.Fatal(err)
	}
	return nil
}
