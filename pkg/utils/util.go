package utils

import (
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/logging"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// ExtractResources extracts the new and old resource as unstructured
func ExtractResources(newRaw []byte, request *admissionv1.AdmissionRequest) (unstructured.Unstructured, unstructured.Unstructured, error) {
	var emptyResource unstructured.Unstructured
	var newResource unstructured.Unstructured
	var oldResource unstructured.Unstructured
	var err error

	// New Resource
	if newRaw == nil {
		newRaw = request.Object.Raw
	}

	if newRaw != nil {
		newResource, err = ConvertResource(newRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err != nil {
			return emptyResource, emptyResource, fmt.Errorf("failed to convert new raw to unstructured: %v", err)
		}
	}

	// Old Resource
	oldRaw := request.OldObject.Raw
	if oldRaw != nil {
		oldResource, err = ConvertResource(oldRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err != nil {
			return emptyResource, emptyResource, fmt.Errorf("failed to convert old raw to unstructured: %v", err)
		}
	}

	return newResource, oldResource, err
}

// ConvertResource converts raw bytes to an unstructured object
func ConvertResource(raw []byte, group, version, kind, namespace string) (unstructured.Unstructured, error) {
	obj, err := engineutils.ConvertToUnstructured(raw)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to convert raw to unstructured: %v", err)
	}

	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})

	if namespace != "" && kind != "Namespace" {
		obj.SetNamespace(namespace)
	}

	if obj.GetKind() == "Namespace" && obj.GetNamespace() != "" {
		obj.SetNamespace("")
	}

	return *obj, nil
}

func NormalizeSecret(resource *unstructured.Unstructured) (unstructured.Unstructured, error) {
	var secret corev1.Secret
	data, err := json.Marshal(resource.Object)
	if err != nil {
		return *resource, err
	}
	err = json.Unmarshal(data, &secret)
	if err != nil {
		return *resource, errors.Wrap(err, "object unable to convert to secret")
	}
	for k, v := range secret.Data {
		if len(v) == 0 {
			secret.Data[k] = []byte("")
		}
	}
	updateSecret := map[string]interface{}{}
	raw, err := json.Marshal(&secret)
	if err != nil {
		return *resource, nil
	}

	err = json.Unmarshal(raw, &updateSecret)
	if err != nil {
		return *resource, nil
	}

	if err != nil {
		return *resource, errors.Wrap(err, "object unable to convert from secret")
	}
	if secret.Data != nil {
		err = unstructured.SetNestedMap(resource.Object, updateSecret["data"].(map[string]interface{}), "data")
		if err != nil {
			return *resource, errors.Wrap(err, "failed to set secret.data")
		}
	}
	return *resource, nil
}

// RedactSecret masks keys of data and metadata.annotation fields of Secrets.
func RedactSecret(resource *unstructured.Unstructured) (unstructured.Unstructured, error) {
	var secret *corev1.Secret
	data, err := json.Marshal(resource.Object)
	if err != nil {
		return *resource, err
	}
	err = json.Unmarshal(data, &secret)
	if err != nil {
		return *resource, errors.Wrap(err, "unable to convert object to secret")
	}
	stringSecret := struct {
		Data map[string]string `json:"string_data"`
		*corev1.Secret
	}{
		Data:   make(map[string]string),
		Secret: secret,
	}
	for key := range secret.Data {
		secret.Data[key] = []byte("**REDACTED**")
		stringSecret.Data[key] = string(secret.Data[key])
	}
	for key := range secret.Annotations {
		secret.Annotations[key] = "**REDACTED**"
	}
	updateSecret := map[string]interface{}{}
	raw, err := json.Marshal(stringSecret)
	if err != nil {
		return *resource, nil
	}
	err = json.Unmarshal(raw, &updateSecret)
	if err != nil {
		return *resource, errors.Wrap(err, "unable to convert object from secret")
	}
	if secret.Data != nil {
		v := updateSecret["string_data"].(map[string]interface{})
		err = unstructured.SetNestedMap(resource.Object, v, "data")
		if err != nil {
			return *resource, errors.Wrap(err, "failed to set secret.data")
		}
	}
	if secret.Annotations != nil {
		metadata, err := datautils.ToMap(resource.Object["metadata"])
		if err != nil {
			return *resource, errors.Wrap(err, "unable to convert metadata to map")
		}
		updatedMeta := updateSecret["metadata"].(map[string]interface{})
		if err != nil {
			return *resource, errors.Wrap(err, "unable to convert object from secret")
		}
		err = unstructured.SetNestedMap(metadata, updatedMeta["annotations"].(map[string]interface{}), "annotations")
		if err != nil {
			return *resource, errors.Wrap(err, "failed to set secret.annotations")
		}
	}
	return *resource, nil
}

// ApiextensionsJsonToKyvernoConditions takes in user-provided conditions in abstract apiextensions.JSON form
// and converts it into []kyverno.Condition or kyverno.AnyAllConditions according to its content.
// it also helps in validating the condtions as it returns an error when the conditions are provided wrongfully by the user.
func ApiextensionsJsonToKyvernoConditions(original apiextensions.JSON) (interface{}, error) {
	path := "preconditions/validate.deny.conditions"

	// checks for the existence any other field apart from 'any'/'all' under preconditions/validate.deny.conditions
	unknownFieldChecker := func(jsonByteArr []byte, path string) error {
		allowedKeys := map[string]bool{
			"any": true,
			"all": true,
		}
		var jsonDecoded map[string]interface{}
		if err := json.Unmarshal(jsonByteArr, &jsonDecoded); err != nil {
			return fmt.Errorf("error occurred while checking for unknown fields under %s: %+v", path, err)
		}
		for k := range jsonDecoded {
			if !allowedKeys[k] {
				return fmt.Errorf("unknown field '%s' found under %s", k, path)
			}
		}
		return nil
	}

	// marshalling the abstract apiextensions.JSON back to JSON form
	jsonByte, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("error occurred while marshalling %s: %+v", path, err)
	}

	var kyvernoOldConditions []kyvernov1.Condition
	if err = json.Unmarshal(jsonByte, &kyvernoOldConditions); err == nil {
		var validConditionOperator bool

		for _, jsonOp := range kyvernoOldConditions {
			for _, validOp := range kyvernov1.ConditionOperators {
				if jsonOp.Operator == validOp {
					validConditionOperator = true
				}
			}
			if !validConditionOperator {
				return nil, fmt.Errorf("invalid condition operator: %s", jsonOp.Operator)
			}
			validConditionOperator = false
		}

		return kyvernoOldConditions, nil
	}

	var kyvernoAnyAllConditions kyvernov1.AnyAllConditions
	if err = json.Unmarshal(jsonByte, &kyvernoAnyAllConditions); err == nil {
		// checking if unknown fields exist or not
		err = unknownFieldChecker(jsonByte, path)
		if err != nil {
			return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
		}
		return kyvernoAnyAllConditions, nil
	}
	return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
}

func OverrideRuntimeErrorHandler() {
	logger := logging.WithName("RuntimeErrorHandler")
	if len(runtime.ErrorHandlers) > 0 {
		runtime.ErrorHandlers[0] = func(err error) {
			logger.V(6).Info("runtime error", "msg", err.Error())
		}
	} else {
		runtime.ErrorHandlers = []func(err error){
			func(err error) {
				logger.V(6).Info("runtime error", "msg", err.Error())
			},
		}
	}
}
