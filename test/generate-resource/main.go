package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	ioutil "io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

var policyPath, resourcePath string

func main() {
	generatePolicies()
}

func generatePolicies() error {
	var policy *kubepolicy.Policy
	numbers := 10

	file, err := ioutil.ReadFile(policyPath)
	if err != nil {
		return fmt.Errorf("failed to load file: %v", err)
	}

	fmt.Printf("Generating policies from %s\n", policyPath)

	rawPolicy, err := yaml.ToJSON(file)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(rawPolicy, &policy); err != nil {
		return fmt.Errorf("failed to decode policy %s, err: %v", policy.Name, err)
	}

	oldName := policy.Name

	for i := 0; i < numbers; i++ {
		newName := oldName + "-" + strconv.Itoa(i)
		data := bytes.Replace(file, []byte(oldName), []byte(newName), -1)

		writeToFile(data, "./.policy/"+newName+".yaml")
	}

	return nil
}

func writeToFile(data []byte, filename string) {

	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			fmt.Println(err)
		}
	}

	if err := ioutil.WriteFile(filename, data, 0755); err != nil {
		fmt.Println(err)
	}
}

func init() {
	flag.StringVar(&policyPath, "policyPath", "", "Path to a policy")
	flag.StringVar(&resourcePath, "resourcePath", "", "Path to a resource")

	flag.Parse()
}
