/*
Copyright The Kyverno Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintOutputFormatsJUnitWithDetailedResults(t *testing.T) {
	// Create a result table with a failing test case
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: true,
		Reason:    "Validation failed",
		Result:    "fail",
		Message:   "Detailed failure message",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "junit", resultTable, true)

	output := buf.String()
	require.Contains(t, output, "<failure")
	require.Contains(t, output, "Detailed failure message")
	require.Contains(t, output, "</failure>")

	// Verify XML is well-formed
	var decoded struct {
		XMLName xml.Name `xml:"testsuites"`
	}
	err := xml.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err, "Output should be valid XML")
}

func TestPrintOutputFormatsJUnitWithoutDetailedResults(t *testing.T) {
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: true,
		Reason:    "Validation failed",
		Result:    "fail",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "junit", resultTable, false)

	output := buf.String()
	require.Contains(t, output, "<failure")
	require.NotContains(t, output, "Detailed failure message")
	require.Contains(t, output, "</failure>")
}

func TestPrintOutputFormatsJUnitPassingTest(t *testing.T) {
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: false,
		Reason:    "Validation passed",
		Result:    "pass",
		Message:   "Success message",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "junit", resultTable, true)

	output := buf.String()
	require.NotContains(t, output, "<failure")
	require.Contains(t, output, "<system-out>")
	require.Contains(t, output, "Success message")
}

func TestPrintOutputFormatsJUnitMalformedXML(t *testing.T) {
	// Test with special characters that could break XML
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: true,
		Reason:    "Special chars: <>&\"",
		Result:    "fail",
		Message:   "Message with <special> & \"chars\"",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "junit", resultTable, true)

	output := buf.String()

	// Verify it's still valid XML despite special characters
	var decoded struct {
		XMLName xml.Name `xml:"testsuites"`
	}
	err := xml.Unmarshal(buf.Bytes(), &decoded)
	assert.NoError(t, err, "Output with special characters should still be valid XML")
	require.Contains(t, output, "<failure")
	require.Contains(t, output, "</failure>")
}

func TestPrintOutputFormatsJSONFormat(t *testing.T) {
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: false,
		Reason:    "Validation passed",
		Result:    "pass",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "json", resultTable, false)

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.True(t, strings.HasPrefix(output, "{") || strings.HasPrefix(output, "["), "JSON output should start with { or [")
}

func TestPrintOutputFormatsTextFormat(t *testing.T) {
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: false,
		Reason:    "Validation passed",
		Result:    "pass",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "text", resultTable, false)

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "test-policy")
}

func TestPrintOutputFormatsUnknownFormat(t *testing.T) {
	resultTable := table.Table{}
	resultTable.AddRow(table.Row{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		IsFailure: false,
		Reason:    "Validation passed",
		Result:    "pass",
	})

	var buf bytes.Buffer
	printOutputFormats(&buf, "unknown", resultTable, false)

	// Should not panic and should produce some output
	output := buf.String()
	assert.NotEmpty(t, output)
}