package handlers

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func WithResponses(rrs ...*engineapi.RuleResponse) []engineapi.RuleResponse {
	var out []engineapi.RuleResponse
	for _, rr := range rrs {
		if rr != nil {
			out = append(out, *rr)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
