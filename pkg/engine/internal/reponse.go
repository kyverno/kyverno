package internal

import (
	"sync"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gomodules.xyz/jsonpatch/v2"
)

type response struct {
	sync.Mutex
	responses []*engineapi.RuleResponse
	patches   []jsonpatch.JsonPatchOperation
}

func (r *response) AddResponse(resp *engineapi.RuleResponse) {
	r.Lock()
	defer r.Unlock()

	r.responses = append(r.responses, resp)
}

func (r *response) AddJsonPatch(patch jsonpatch.JsonPatchOperation) {
	r.Lock()
	defer r.Unlock()

	r.patches = append(r.patches, patch)
}

func (r *response) Get() ([]jsonpatch.JsonPatchOperation, []*engineapi.RuleResponse) {
	return r.patches, r.responses
}
