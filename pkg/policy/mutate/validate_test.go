package mutate

import (
	"context"
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_Validate_PatchStrategicMerge_Has_Conditional_Anchors(t *testing.T) {
	var mutation kyvernov1.Mutation
	var err error
	//case 1
	rawPolicy := []byte(`{
		"foreach": [{
		  "patchStrategicMerge": {
			"spec": {
			  "containers": {
				"(name)": "*",
				"image": "{{regex_replace_all('^([^/]+\\.[^/]+/)?(.*)$','{{@}}','myregistry.corp.com/$2')}}"
			  }
			}
		  }
		}]
	}`)

	err = json.Unmarshal(rawPolicy, &mutation)
	assert.NilError(t, err)

	checker := NewFakeMutate(mutation)
	if _, err := checker.Validate(context.TODO()); err != nil {
		assert.Assert(t, err != nil)
	}

	//case 2
	rawPolicy = []byte(`{
		"foreach": [{
		  "patchStrategicMerge": {
			"spec": {
			  "template": {
			    "spec": {
				  "containers": {
					"(name)": "*",
					"image": "{{regex_replace_all('^([^/]+\\.[^/]+/)?(.*)$','{{@}}','myregistry.corp.com/$2')}}"
				  }
				}
			  }
			}
		  }
		}]
	}`)

	err = json.Unmarshal(rawPolicy, &mutation)
	assert.NilError(t, err)

	checker = NewFakeMutate(mutation)
	if _, err := checker.Validate(context.TODO()); err != nil {
		assert.Assert(t, err != nil)
	}

	//case 3
	rawPolicy = []byte(`{
		"foreach": [{
		  "patchStrategicMerge": {
			"spec": {
			  "jobTemplate": {
				"spec": {
				  "template": {
					"spec": {
					  "containers": {
						"(name)": "*",
						"image": "{{regex_replace_all('^([^/]+\\.[^/]+/)?(.*)$','{{@}}','myregistry.corp.com/$2')}}"
					  }
					}
				  }
				}
		  	  }
			}
		  }
		}]
	}`)

	err = json.Unmarshal(rawPolicy, &mutation)
	assert.NilError(t, err)

	checker = NewFakeMutate(mutation)
	if _, err := checker.Validate(context.TODO()); err != nil {
		assert.Assert(t, err != nil)
	}
}
