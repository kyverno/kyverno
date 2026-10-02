package test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/stretchr/testify/assert"
)

func TestPrintOutputFormatsJunitFailureIsValidXML(t *testing.T) {
	for _, detailedResults := range []bool{false, true} {
		var resultTable table.Table
		row := table.Row{Message: `validation error: label "app" must be <a & b>`}
		row.Policy = `require-"labels"`
		row.Rule = "check-labels<&>"
		row.Resource = "v1/Pod/default/nginx"
		row.Result = "Fail"
		row.Reason = `Want "pass", got <fail> & more`
		row.IsFailure = true
		resultTable.Add(row)
		passRow := table.Row{Message: "data ]]> end"}
		passRow.Policy = "require-labels"
		passRow.Rule = "check-labels"
		passRow.Resource = "v1/Pod/default/web"
		passRow.Result = "Pass"
		passRow.Reason = "Ok ]]> done"
		resultTable.Add(passRow)

		var out bytes.Buffer
		printOutputFormats(&out, "junit", resultTable, detailedResults)

		decoder := xml.NewDecoder(strings.NewReader(strings.TrimSpace(out.String())))
		var err error
		for err == nil {
			_, err = decoder.Token()
		}
		assert.True(t, errors.Is(err, io.EOF), "detailedResults=%v: %v", detailedResults, err)
	}
}
