package v1

import (
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func FromJSON(in *apiextv1.JSON) apiextensions.JSON {
	var out apiextensions.JSON

	if in != nil {
		var i interface{}
		if len(in.Raw) > 0 && !bytes.Equal(in.Raw, []byte(`null`)) {
			d := json.NewDecoder(strings.NewReader(string(in.Raw)))
			d.UseNumber()
			if err := d.Decode(&out); err != nil {
				log.Log.Error(err, "failed to convert JSON to interface")
			}
		}
		out = i
	} else {
		out = nil
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
func ValidateAutogenAnnotation(path *field.Path, annotations map[string]string) (errs field.ErrorList) {
	value, ok := annotations[PodControllersAnnotation]
	if ok {
		if value == "all" {
			errs = append(errs, field.Forbidden(path, "Autogen annotation does not support 'all' anymore, remove the annotation or set it to a valid value"))
		}
	}
	return errs
}

// ValidatePolicyName validates policy name
func ValidatePolicyName(path *field.Path, name string) (errs field.ErrorList) {
	// policy name is stored in the label of the report change request
	if len(name) > 63 {
		errs = append(errs, field.TooLong(path, name, 63))
	}
	return errs
}
