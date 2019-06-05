package engine

import (
	"encoding/json"
	"github.com/nirmata/kyverno/pkg/result"
	"reflect"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"gotest.tools/assert"
)

func compareJsonAsMap(t *testing.T, expected, actual []byte) {
	var expectedMap, actualMap map[string]interface{}
	assert.NilError(t, json.Unmarshal(expected, &expectedMap))
	assert.NilError(t, json.Unmarshal(actual, &actualMap))
	assert.Assert(t, reflect.DeepEqual(expectedMap, actualMap))
}

func TestApplyOverlay_NestedListWithAnchor(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)

	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "ports":[  
				 {  
					"(name)":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := result.NewRuleApplicationResult("")
	patches := applyOverlay(resource, overlay, "/", &res)
	assert.NilError(t, res.ToError())
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)
	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)

	expectedResult := []byte(`{"apiVersion":"v1","kind":"Endpoints","metadata":{"name":"test-endpoint","labels":{"label":"test"}},"subsets":[{"addresses":[{"ip":"192.168.10.171"}],"ports":[{"name":"secure-connection","port":444.000000,"protocol":"UDP"}]}]}`)
	compareJsonAsMap(t, expectedResult, patched)
}

func TestApplyOverlay_InsertIntoArray(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)
	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.172"
				 },
				 {  
					"ip":"192.168.10.173"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"insecure-connection",
					"port":80,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := result.NewRuleApplicationResult("")
	patches := applyOverlay(resource, overlay, "/", &res)
	assert.NilError(t, res.ToError())
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)

	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)

	expectedResult := []byte(`{"apiVersion":"v1","kind":"Endpoints","metadata":{"name":"test-endpoint","labels":{"label":"test"}},"subsets":[{"addresses":[{"ip":"192.168.10.172"},{"ip":"192.168.10.173"}],"ports":[{"name":"insecure-connection","port":80.000000,"protocol":"UDP"}]},{"addresses":[{"ip":"192.168.10.171"}],"ports":[{"name":"secure-connection","port":443,"protocol":"TCP"}]}]}`)
	compareJsonAsMap(t, expectedResult, patched)
}

func TestApplyOverlay_TestInsertToArray(t *testing.T) {
	overlayRaw := []byte(`
	 {  
		"spec":{  
		   "template":{  
			  "spec":{  
				 "containers":[  
					{  
					   "name":"pi1",
					   "image":"vasylev.perl"
					}
				 ]
			  }
		   }
		}
	 }`)
	resourceRaw := []byte(`{  
		"apiVersion":"batch/v1",
		"kind":"Job",
		"metadata":{  
		   "name":"pi"
		},
		"spec":{  
		   "template":{  
			  "spec":{  
				 "containers":[  
					{  
					   "name":"piv0",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					},
					{  
					   "name":"pi",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					},
					{  
					   "name":"piv1",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					}
				 ],
				 "restartPolicy":"Never"
			  }
		   },
		   "backoffLimit":4
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	res := result.NewRuleApplicationResult("")
	patches := applyOverlay(resource, overlay, "/", &res)
	assert.NilError(t, res.ToError())
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)

	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)
}
