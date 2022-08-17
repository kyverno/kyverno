package common

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/kyverno/kyverno/test/e2e"
)

const defaultTestK8sVersion = "1.21.0"

func CallMetrics() (string, error) {
	requestObj := e2e.APIRequest{
		URL:  "http://localhost:8000/metrics",
		Type: "GET",
	}

	response, err := e2e.CallAPI(requestObj)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	if err != nil {
		return "", err
	}

	newStr := buf.String()
	return newStr, nil
}

func GetKubernetesVersion() semver.Version {
	ver, err := semver.Parse(os.Getenv("K8S_VERSION"))
	if err != nil {
		return semver.MustParse(defaultTestK8sVersion)
	}
	return ver
}

// ProcessMetrics checks the metrics log and identify if the policy is added in cache or not
func ProcessMetrics(newStr, e2ePolicyName string) error {
	splitByNewLine := strings.Split(newStr, "\n")
	for _, lineSplitByNewLine := range splitByNewLine {
		// kyverno_policy_rule_info_total{policy_background_mode=\"false\",policy_name=\"gen-cluster-policy\",policy_namespace=\"-\",policy_type=\"cluster\",policy_validation_mode=\"audit\",rule_name=\"gen-cluster-role\",rule_type=\"generate\",status_ready="false"} 1
		if !strings.HasPrefix(lineSplitByNewLine, "kyverno_policy_rule_info_total{") {
			continue
		}

		if !strings.HasSuffix(lineSplitByNewLine, "} 1") {
			continue
		}

		splitByComma := strings.Split(lineSplitByNewLine, ",")
		for _, lineSplitByComma := range splitByComma {
			if strings.HasPrefix(lineSplitByComma, "policy_name=") {
				splitByQuote := strings.Split(lineSplitByComma, "\"")
				policyName := splitByQuote[1]
				if policyName != e2ePolicyName {
					continue
				}
			}
			if strings.HasPrefix(lineSplitByComma, "status_ready=") {
				splitByQuote := strings.Split(lineSplitByComma, "\"")
				status := splitByQuote[1]
				if status == "true" {
					return nil
				}
			}

		}
	}

	return fmt.Errorf("policy %s not found in metrics %s", e2ePolicyName, newStr)
}

func PolicyCreated(policyName string) error {
	return e2e.GetWithRetry(1*time.Second, 60, checkPolicyCreated(policyName))
}

func checkPolicyCreated(policyName string) func() error {
	return func() error {
		var metricsString string
		metricsString, err := CallMetrics()
		if err != nil {
			return fmt.Errorf("failed to get metrics: %v", err)
		}

		err = ProcessMetrics(metricsString, policyName)
		if err != nil {
			return fmt.Errorf("policy not created: %v", err)
		}

		// Wait to make sure that the Policy is ready.
		time.Sleep(2 * time.Second)
		return nil
	}
}
