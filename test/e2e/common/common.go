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

func ProcessMetrics(newStr, e2ePolicyName string, e2eTime time.Time) bool {
	fmt.Println("e2ePolicyName: ", e2ePolicyName, "e2eTime: ", e2eTime)
	var action, policyName string
	var timeInTimeFormat time.Time
	var err error
	splitByNewLine := strings.Split(newStr, "\n")
	for _, lineSplitedByNewLine := range splitByNewLine {
		if strings.HasPrefix(lineSplitedByNewLine, "kyverno_policy_changes_info{") {
			splitByComma := strings.Split(lineSplitedByNewLine, ",")
			for _, lineSplitedByComma := range splitByComma {
				if strings.HasPrefix(lineSplitedByComma, "policy_change_type=") {
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					action = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitedByComma, "policy_name=") {
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					policyName = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitedByComma, "timestamp=") {
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					layout := "2006-01-02 15:04:05 -0700 MST"
					timeInTimeFormat, err = time.Parse(layout, splitByQuote[1])
					if err != nil {
						fmt.Println("error occurred: ", err)
					}
				}
			}

			if policyName == e2ePolicyName {
				fmt.Println("--------------------------------------------------------")
				fmt.Println(lineSplitedByNewLine)
				fmt.Println("action: ", action)
				fmt.Println("policyName: ", policyName)
				fmt.Println("timeInTimeFormat: ", timeInTimeFormat)

				diff := e2eTime.Sub(timeInTimeFormat)
				fmt.Println("diff: ", diff)
				if diff < time.Second {
					fmt.Println("****** condition ******")
					if action == "created" {
						fmt.Println("************policy created**************")
						return true
					}
				}
			}
		}
	}
	return false
}
