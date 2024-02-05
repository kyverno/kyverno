package scan

import (
	"encoding/json"
	"fmt"
	"io"

	jsonengine "github.com/kyverno/kyverno-json/pkg/json-engine"
)

type output interface {
	println(args ...any)
	responses(responses ...jsonengine.Response)
}

type textOutput struct {
	out io.Writer
}

func (t *textOutput) println(args ...any) {
	fmt.Fprintln(t.out, args...)
}

func (t *textOutput) responses(responses ...jsonengine.Response) {
}

type jsonOutput struct {
	out io.Writer
}

func (t *jsonOutput) println(args ...any) {
}

func (t *jsonOutput) responses(responses ...jsonengine.Response) {
	payload, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		fmt.Fprintln(t.out, err)
	} else {
		fmt.Fprintln(t.out, string(payload))
	}
}

func newOutput(out io.Writer, format string) output {
	if format == "json" {
		return &jsonOutput{out: out}
	}
	return &textOutput{out: out}
}
