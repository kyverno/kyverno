package validate

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_Validate_OverlayPattern_Empty(t *testing.T) {
	rawValidation := []byte(`
   {}`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)

	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}
func Test_Validate_OverlayPattern_Nil_PatternAnypattern(t *testing.T) {
	rawValidation := []byte(`
 	{ "message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false"
      }
	`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_OverlayPattern_Exist_PatternAnypattern(t *testing.T) {
	rawValidation := []byte(`
	{
		"message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false",
		"anyPattern": [
		  {
			"spec": {
			  "securityContext": {
				"allowPrivilegeEscalation": false,
				"privileged": false
			  }
			}
		  }
		],
		"pattern": {
		  "spec": {
			"containers": [
			  {
				"name": "*",
				"securityContext": {
				  "allowPrivilegeEscalation": false,
				  "privileged": false
				}
			  }
			]
		  }
		}
	  }`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}
func Test_Validate_OverlayPattern_Valid(t *testing.T) {
	rawValidation := []byte(`
	{
		"message": "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false",
		"anyPattern": [
		  {
			"spec": {
			  "securityContext": {
				"allowPrivilegeEscalation": false,
				"privileged": false
			  }
			}
		  },
		  {
			"spec": {
			  "containers": [
				{
				  "name": "*",
				  "securityContext": {
					"allowPrivilegeEscalation": false,
					"privileged": false
				  }
				}
			  ]
			}
		  }
		]
	  }
`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.NilError(t, err)
	}
}

func Test_Validate_ExistingAnchor_AnchorOnMap(t *testing.T) {
	rawValidation := []byte(`
	{
		"message": "validate container security contexts",
		"anyPattern": [
		  {
			"spec": {
			  "template": {
				"spec": {
				  "containers": [
					{
					  "^(securityContext)": {
						"runAsNonRoot": true
					  }
					}
				  ]
				}
			  }
			}
		  }
		]
	  }
`)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_ExistingAnchor_AnchorOnString(t *testing.T) {
	rawValidation := []byte(`{
		"message": "validate container security contexts",
		"pattern": {
		  "spec": {
			"template": {
			  "spec": {
				"containers": [
				  {
					"securityContext": {
					  "allowPrivilegeEscalation": "^(false)"
					}
				  }
				]
			  }
			}
		  }
		}
	  }
	  		  `)

	var validation kyverno.Validation
	err := json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_ExistingAnchor_Valid(t *testing.T) {
	var err error
	var validation kyverno.Validation
	rawValidation := []byte(`
	{
		"message": "validate container security contexts",
		"anyPattern": [
		   {
			  "spec": {
				 "template": {
					"spec": {
					   "^(containers)": [
						  {
							 "securityContext": {
								"runAsNonRoot": "true"
							 }
						  }
					   ]
					}
				 }
			  }
		   }
		]
	 }`)

	err = json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
	rawValidation = []byte(`
	{
		"message": "validate container security contexts",
		"pattern": {
		   "spec": {
			  "template": {
				 "spec": {
					"^(containers)": [
					   {
						  "securityContext": {
							 "allowPrivilegeEscalation": "false"
						  }
					   }
					]
				 }
			  }
		   }
		}
	 }	`)
	err = json.Unmarshal(rawValidation, &validation)
	assert.NilError(t, err)
	checker = NewValidateFactory(&validation)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

}

func Test_Validate_Validate_ValidAnchor(t *testing.T) {
	var err error
	var validate kyverno.Validation
	var rawValidate []byte
	// case 1
	rawValidate = []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"anyPattern": [
		    {
			  "spec": {
				 "securityContext": {
					"(runAsNonRoot)": true
				 }
			  }
		   },
		   {
			  "spec": {
				 "^(containers)": [
					{
					   "name": "*",
					   "securityContext": {
						  "runAsNonRoot": true
					   }
					}
				 ]
			  }
		   }
		]
	 }`)

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	checker := NewValidateFactory(&validate)
	if _, err := checker.Validate(); err != nil {
		assert.NilError(t, err)
	}

	// case 2
	validate = kyverno.Validation{}
	rawValidate = []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "=(securityContext)": {
				 "runAsNonRoot": "true"
			  }
		   }
		}
	}`)

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	checker = NewValidateFactory(&validate)
	if _, err := checker.Validate(); err != nil {
		assert.NilError(t, err)
	}
}

func Test_Validate_Validate_Mismatched(t *testing.T) {
	rawValidate := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "+(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	 }`)

	var validate kyverno.Validation
	err := json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validate)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_Validate_Unsupported(t *testing.T) {
	var err error
	var validate kyverno.Validation

	// case 1
	rawValidate := []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "!(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	}`)

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)
	checker := NewValidateFactory(&validate)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

	// case 2
	rawValidate = []byte(`
	{
		"message": "Root user is not allowed. Set runAsNonRoot to true.",
		"pattern": {
		   "spec": {
			  "containers": [
				 {
					"name": "*",
					"securityContext": {
					   "~(runAsNonRoot)": true
					}
				 }
			  ]
		   }
		}
	}`)

	err = json.Unmarshal(rawValidate, &validate)
	assert.NilError(t, err)

	checker = NewValidateFactory(&validate)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

}
