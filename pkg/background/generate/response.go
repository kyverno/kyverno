package generate

import "github.com/kyverno/kyverno/api/kyverno"

// ResourceMode defines the mode for generated resource
type resourceMode string

const (
	// Skip : failed to process rule, will not update the resource
	Skip resourceMode = "SKIP"
	// Create : create a new resource
	Create = "CREATE"
	// Update : update/overwrite the new resource
	Update = "UPDATE"
)

type generateResponse struct {
	data   map[string]interface{}
	action resourceMode
	target kyverno.ResourceSpec
	err    error
}

func newGenerateResponse(data map[string]interface{}, action resourceMode, target kyverno.ResourceSpec, err error) generateResponse {
	return generateResponse{
		data:   data,
		action: action,
		target: target,
		err:    err,
	}
}

func newSkipGenerateResponse(data map[string]interface{}, target kyverno.ResourceSpec, err error) generateResponse {
	return newGenerateResponse(data, Skip, target, err)
}

func newUpdateGenerateResponse(data map[string]interface{}, target kyverno.ResourceSpec, err error) generateResponse {
	return newGenerateResponse(data, Update, target, err)
}

func newCreateGenerateResponse(data map[string]interface{}, target kyverno.ResourceSpec, err error) generateResponse {
	return newGenerateResponse(data, Create, target, err)
}

func (resp *generateResponse) GetData() map[string]interface{} {
	return resp.data
}

func (resp *generateResponse) GetAction() resourceMode {
	return resp.action
}

func (resp *generateResponse) GetTarget() kyverno.ResourceSpec {
	return resp.target
}

func (resp *generateResponse) GetError() error {
	return resp.err
}
