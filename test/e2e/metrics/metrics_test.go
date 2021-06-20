package metrics

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	. "github.com/onsi/gomega"
)

func Test_MetricsServerAvailability(t *testing.T) {
	RegisterTestingT(t)
	// if os.Getenv("E2E") == "" {
	// 	t.Skip("Skipping E2E Test")
	// }
	requestObj := e2e.APIRequest{
		URL:  "http://localhost:8000/metrics",
		Type: "GET",
	}
	response, err := e2e.CallAPI(requestObj)
	Expect(err).NotTo(HaveOccurred())
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	newStr := buf.String()
	fmt.Println("==============================================================")
	fmt.Println(newStr)
	fmt.Println("==============================================================")
	processMetrics(newStr, "multi-tenancy", time.Now())
	Expect(response.StatusCode).To(Equal(200))
}

func processMetrics(newStr, e2ePolicyName string, e2eTime time.Time) {
	var action, policyName string
	var timeInTimeFormat time.Time
	var err error
	splitByNewLine := strings.Split(newStr, "\n")
	for _, lineSplitedByNewLine := range splitByNewLine {
		if strings.HasPrefix(lineSplitedByNewLine, "kyverno_policy_changes_info{") {
			// fmt.Println(lineSplitedByNewLine)
			splitByComma := strings.Split(lineSplitedByNewLine, ",")
			for _, lineSplitedByComma := range splitByComma {
				// fmt.Println(lineSplitedByComma)
				if strings.HasPrefix(lineSplitedByComma, "policy_change_type=") {
					// action = lineSplitedByComma
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					action = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitedByComma, "policy_name=") {
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					policyName = splitByQuote[1]
				}
				if strings.HasPrefix(lineSplitedByComma, "timestamp=") {
					splitByQuote := strings.Split(lineSplitedByComma, "\"")
					timeInTimeFormat, err = time.Parse(splitByQuote[1], "2014-11-17 23:02:03 +0000 UTC")
					if err != nil {
						fmt.Println("error: ", err)
					}
				}
			}
			break
		}
	}
	fmt.Println("action: ", action)
	fmt.Println("policyName: ", policyName)
	fmt.Println("timeInTimeFormat: ", timeInTimeFormat)

	diff := time.Now().Sub(timeInTimeFormat)
	fmt.Println(diff)

	diff = timeInTimeFormat.Sub(time.Now())
	fmt.Println(diff)

}
