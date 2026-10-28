package api

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/jmespath"
)

var fuzzCfg = config.NewDefaultConfiguration(false)

func FuzzJmespath(f *testing.F) {
	f.Fuzz(func(t *testing.T, jmsString, value string) {
		jp := jmespath.New(fuzzCfg)
		q, err := jp.Query(jmsString)
		if err != nil {
			return
		}
		q.Search(value)
	})
}
