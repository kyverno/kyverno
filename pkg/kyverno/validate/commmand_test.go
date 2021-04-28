package validate

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_validateUsingPolicyCRD(t *testing.T) {
	type TestCase struct {
		rawPolicy   []byte
		errorDetail string
	}

	testcases := []TestCase{
		{
			rawPolicy: []byte(`
		{
			"apiVersion": "kyverno.io/v1",
			"kind": "ClusterPolicy",
			"metadata": {
			  "name": "add-requests"
			},
			"spec": {
			  "rules": [
				{
				  "name": "Set memory and/or cpu requests for all pods in namespaces labeled 'myprivatelabel'",
				  "match": {
					"resources": {
					  "kinds": [
						"Pod"
					  ]
					}
				  },
				  "mutate": {
					"overlay": {
					  "spec": {
						"containers": [
						  {
							"(name)": "*",
							"resources": {
							  "requests": {
								"cpu": "1000m"
							  }
							}
						  }
						]
					  }
					}
				  }
				}
			  ]
			}
		  }
		`),
			errorDetail: "spec.rules.name in body should be at most 63 chars long",
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					  "name": "min-replicas-clusterpolicy"
					},
					"spec": {
					  "validationFailureAction": "audit",
					  "rules": [
						{
						  "name": "check-min-replicas",
						  "match": {
							"resources": {
							  "kinds": [
								"Deployment"
							  ]
							}
						  },
						  "validate": {
							"message": "should have at least 2 replicas",
							"pattern": {
							  "spec": {
								"replicas": 2
							  }
							}
						  }
						}
					  ]
					}
				  }
		`),
			errorDetail: "",
		},
	}

	v1crd, err := getPolicyCRD()
	assert.NilError(t, err)

	var policy kyverno.ClusterPolicy
	for _, tc := range testcases {
		err = json.Unmarshal(tc.rawPolicy, &policy)
		assert.NilError(t, err)

		_, errorList := validatePolicyAccordingToPolicyCRD(&policy, v1crd)
		fmt.Println("errorList:  ", errorList)
		for _, e := range errorList {
			assert.Assert(t, tc.errorDetail == e.Detail)
		}
	}
}
