package v1

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func FromJSON(in *apiextv1.JSON) apiextensions.JSON {
	var out apiextensions.JSON
	if err := apiextv1.Convert_v1_JSON_To_apiextensions_JSON(in, &out, nil); err != nil {
		log.Log.Error(err, "failed to convert JSON to interface")
	}
	return out
}

func ToJSON(in apiextensions.JSON) *apiextv1.JSON {
	if in == nil {
		return nil
	}
	var out apiextv1.JSON
	if err := apiextv1.Convert_apiextensions_JSON_To_v1_JSON(&in, &out, nil); err != nil {
		log.Log.Error(err, "failed to convert interface to JSON")
	}
	return &out
}

// ValidatePolicyName validates policy name
func ValidatePolicyName(path *field.Path, name string) field.ErrorList {
	var errs field.ErrorList
	// policy name is stored in the label of the report change request
	if len(name) > 63 {
		errs = append(errs, field.TooLong(path, name, 63))
	}
	return errs
}

// ViolatedRule stores the information regarding the rule.
type ViolatedRule struct {
	// Name specifies violated rule name.
	Name string `json:"name" yaml:"name"`

	// Type specifies violated rule type.
	Type string `json:"type" yaml:"type"`

	// Message specifies violation message.
	// +optional
	Message string `json:"message" yaml:"message"`

	// Status shows the rule response status
	Status string `json:"status" yaml:"status"`
}
