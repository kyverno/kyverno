package testutils

import (
	"fmt"
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
	fmt.Println(ts.path)
	err := filepath.Walk(ts.path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			// check if there are resources dir and policies yaml
			tb := NewTestBundle(path)
			if tb != nil {
				// load resources
				err := tb.load()
				if err != nil {
					//					glog.Error(err)
					return nil
				}
				ts.tb = append(ts.tb, tb)
				//				fmt.Println(path)
			}
		}
		return nil
	})
	if err != nil {
		ts.t.Fatal(err)
	}
	return nil
}
