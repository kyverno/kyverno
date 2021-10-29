package validate

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_validateUsingPolicyCRD(t *testing.T) {
	type TestCase struct {
		rawPolicy   []byte
		errorDetail string
		detail      string
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
			detail:      "Test: char count for rule name",
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
			detail:      "Test: basic vaild policy",
		},

		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					  "name": "disallow-singleton"
					},
					"spec": {
					  "validationFailureAction": "audit",
					  "rules": [
						{
						  "name": "validate-replicas",
						  "match": {
							"resources": {
							  "kinds": [
								"Deployment"
							  ],
							  "annotations": {
								"singleton": "true"
							  }
							}
						  },
						  "validate": {
							"message": "Replicasets require at least 2 replicas.",
							"pattern": {
							  "spec": {
								"replicas": ">1"
							  }
							}
						  }
						}
					  ]
					}
				  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.match.resources.annotations",
		},

		{
			rawPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
				  "name": "disallow-singleton"
				},
				"spec": {
				  "validationFailureAction": "audit",
				  "rules": [
					{
					  "name": "validate-replicas",
					  "match": {
						"resources": {
						  "kinds": [
							"Deployment"
						  ]
						}
					  },
					  "exclude": {
						"resources": {
						  "annotations": {
							"singleton": "true"
						  }
						}
					  },
					  "validate": {
						"message": "Replicasets require at least 2 replicas.",
						"pattern": {
						  "spec": {
							"replicas": ">1"
						  }
						}
					  }
					}
				  ]
				}
			  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.exclude.resources.annotations",
		},

		{
			rawPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
				  "name": "enforce-pod-name"
				},
				"spec": {
				  "validationFailureAction": "audit",
				  "background": true,
				  "rules": [
					{
					  "name": "validate-name",
					  "match": {
						"resources": {
						  "kinds": [
							"Pod"
						  ],
						  "namespaceSelector": {
							"matchLabels": {
							  "app-namespace": "true"
							}
						  }
						}
					  },
					  "validate": {
						"message": "The Pod must end with -nginx",
						"pattern": {
						  "metadata": {
							"name": "*-nginx"
						  }
						}
					  }
					}
				  ]
				}
			  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.match.resources.namespaceSelector.matchLabels",
		},

		{
			rawPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
				  "name": "enforce-pod-name"
				},
				"spec": {
				  "validationFailureAction": "audit",
				  "background": true,
				  "rules": [
					{
					  "name": "validate-name",
					  "match": {
						"resources": {
						  "kinds": [
							"Pod"
						  ]
						}
					  },
					  "exclude": {
						"resources": {
						  "namespaceSelector": {
							"matchLabels": {
							  "app-namespace": "true"
							}
						  }
						}
					  },
					  "validate": {
						"message": "The Pod must end with -nginx",
						"pattern": {
						  "metadata": {
							"name": "*-nginx"
						  }
						}
					  }
					}
				  ]
				}
			  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.exclude.resources.namespaceSelector.matchLabels",
		},

		{
			rawPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
				  "name": "enforce-pod-name"
				},
				"spec": {
				  "validationFailureAction": "audit",
				  "background": true,
				  "rules": [
					{
					  "name": "validate-name",
					  "match": {
						"resources": {
						  "kinds": [
							"Pod"
						  ],
						  "selector": {
							"matchLabels": {
							  "app-namespace": "true"
							}
						  }
						}
					  },
					  "validate": {
						"message": "The Pod must end with -nginx",
						"pattern": {
						  "metadata": {
							"name": "*-nginx"
						  }
						}
					  }
					}
				  ]
				}
			  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.match.resources.selector.matchLabels",
		},

		{
			rawPolicy: []byte(`
			{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
				  "name": "enforce-pod-name"
				},
				"spec": {
				  "validationFailureAction": "audit",
				  "background": true,
				  "rules": [
					{
					  "name": "validate-name",
					  "match": {
						"resources": {
						  "kinds": [
							"Pod"
						  ]
						}
					  },
					  "exclude": {
						"resources": {
						  "selector": {
							"matchLabels": {
							  "app-namespace": "true"
							}
						  }
						}
					  },
					  "validate": {
						"message": "The Pod must end with -nginx",
						"pattern": {
						  "metadata": {
							"name": "*-nginx"
						  }
						}
					  }
					}
				  ]
				}
			  }
		`),
			errorDetail: "",
			detail:      "Test: schema validation for spec.rules.exclude.resources.selector.matchLabels",
		},
	}

	v1crd, err := getPolicyCRD()
	assert.NilError(t, err)

	var policy kyverno.ClusterPolicy
	for _, tc := range testcases {
		err = json.Unmarshal(tc.rawPolicy, &policy)
		assert.NilError(t, err)

		_, errorList := validatePolicyAccordingToPolicyCRD(&policy, v1crd)
		fmt.Println(tc.detail)
		for _, e := range errorList {
			assert.Assert(t, tc.errorDetail == e.Detail)
		}
	}
}
