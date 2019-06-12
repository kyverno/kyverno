package testutils

import (
	"fmt"
	"io/ioutil"
	ospath "path"
	"path/filepath"
	"testing"
)

// Load policy & resource files
// engine pass the (policy, resource)
// check the expected response

const examplesPath string = "examples"
const resourcesFolder string = "resources"
const tScenarioFile string = "testScenarios.yaml"

//LoadTestSuite  reads the resource, policy and scenario files
func LoadTestSuite(t *testing.T, path string) *testSuite {
	// gp := os.Getenv("GOPATH")
	// ap := ospath.Join(gp, "src/github.com/nirmata/kyverno")
	// build test suite
	// each suite contains test bundles for test sceanrios
	// ts := NewTestSuite(t, ospath.Join(ap, examplesPath))
	ts := NewTestSuite(t, path)
	ts.buildTestSuite()
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
		fmt.Println(filepath.Ext(file.Name()))
		if file.Name() == tScenarioFile {
			continue
		}
		if filepath.Ext(file.Name()) == ".yml" || filepath.Ext(file.Name()) == ".yaml" {
			yamls = append(yamls, ospath.Join(path, file.Name()))
		}
	}
	return yamls
}
