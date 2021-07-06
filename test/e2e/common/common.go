package common

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
)

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

// ProcessMetrics checks the metrics log and identify if the policy is added in cache or not
func ProcessMetrics(newStr, e2ePolicyName string, e2eTime time.Time) error {
	var action, policyName string
	var timeInTimeFormat time.Time
	var err error
	splitByNewLine := strings.Split(newStr, "\n")
	for _, lineSplitByNewLine := range splitByNewLine {
		if strings.HasPrefix(lineSplitByNewLine, "kyverno_policy_changes_info{") {
			splitByComma := strings.Split(lineSplitByNewLine, ",")
			for _, lineSplitByComma := range splitByComma {
				if strings.HasPrefix(lineSplitByComma, "policy_change_type=") {
					splitByQuote := strings.Split(lineSplitByComma, "\"")
					action = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitByComma, "policy_name=") {
					splitByQuote := strings.Split(lineSplitByComma, "\"")
					policyName = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitByComma, "timestamp=") {
					splitByQuote := strings.Split(lineSplitByComma, "\"")
					layout := "2006-01-02 15:04:05 -0700 MST"
					timeInTimeFormat, err = time.Parse(layout, splitByQuote[1])
					if err != nil {
						return err
					}
				}
			}

			if policyName == e2ePolicyName && action == "created" {
				if timeInTimeFormat.After(e2eTime) || timeInTimeFormat.Equal(e2eTime) {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("policy %s after %s not found in metrics %s", e2ePolicyName, e2eTime, newStr)
}

func PolicyCreated(policyName string, timeBeforePolicyCreation time.Time) error {
	return e2e.GetWithRetry(1*time.Second, 60, checkPolicyCreated(policyName, timeBeforePolicyCreation))
}

func checkPolicyCreated(policyName string, timeBeforePolicyCreation time.Time) func() error {
	return func() error {
		var metricsString string
		metricsString, err := CallMetrics()
		if err != nil {
			return fmt.Errorf("failed to get metrics: %v", err)
		}

		err = ProcessMetrics(metricsString, policyName, timeBeforePolicyCreation)
		if err != nil {
			return fmt.Errorf("policy not created: %v", err)
		}

		return nil
	}
}
