package testrunner

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"os"
	ospath "path"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

func runner(t *testing.T, relpath string) {
	gp := os.Getenv("GOPATH")
	ap := ospath.Join(gp, projectPath)
	// build load scenarios
	path := ospath.Join(ap, relpath)
	// Load the scenario files
	scenarioFiles := getYAMLfiles(path)
	for _, secenarioFile := range scenarioFiles {
		sc := newScenario(t, ap, secenarioFile)
		if err := sc.load(); err != nil {
			t.Error(err)
			return
		}
		// run test cases
		sc.run()
	}
}

type scenario struct {
	ap   string
	t    *testing.T
	path string
	tcs  []*testCase
}

func newScenario(t *testing.T, ap string, path string) *scenario {
	return &scenario{
		ap:   ap,
		t:    t,
		path: path,
	}
}

func getYAMLfiles(path string) (yamls []string) {
	fileInfo, err := ioutil.ReadDir(path)
	if err != nil {
		return nil
	}
	for _, file := range fileInfo {
		if filepath.Ext(file.Name()) == ".yml" || filepath.Ext(file.Name()) == ".yaml" {
			yamls = append(yamls, ospath.Join(path, file.Name()))
		}
	}
	return yamls
}
func (sc *scenario) load() error {
	// read file
	data, err := LoadFile(sc.path)
	if err != nil {
		return err
	}
	tcs := []*testCase{}
	// load test cases seperated by '---'
	// each test case defines an input & expected result
	dd := bytes.Split(data, []byte(defaultYamlSeparator))
	for _, d := range dd {
		tc := &testCase{}
		err := yaml.Unmarshal([]byte(d), tc)
		if err != nil {
			glog.Warningf("Error while decoding YAML object, err: %s", err)
			continue
		}
		tcs = append(tcs, tc)
	}
	sc.tcs = tcs
	return nil
}

func (sc *scenario) run() {
	if len(sc.tcs) == 0 {
		sc.t.Error("No test cases to load")
		return
	}

	for _, tc := range sc.tcs {
		t, err := NewTest(sc.ap, sc.t, tc)
		if err != nil {
			sc.t.Error(err)
			continue
		}
		t.run()
	}
}
