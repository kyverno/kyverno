package policy

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/googleapis/gnostic/compiler"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"

	"gopkg.in/yaml.v2"
)

var validationGlobalState struct {
	document    *openapi_v2.Document
	definitions map[string]*openapi_v2.Schema
	models      proto.Models
	isSet       bool
}

func init() {
	err := setValidationGlobalState()
	if err != nil {
		panic(err)
	}
}

func ValidatePolicyMutation(policy v1.ClusterPolicy) error {
	if validationGlobalState.isSet == false {
		glog.V(4).Info("Cannot Validate policy: Validation global state not set")
		return nil
	}

	var kindToRules = make(map[string][]v1.Rule)
	for _, rule := range policy.Spec.Rules {
		rule.MatchResources.Selector = nil
		if rule.HasMutate() {
			for _, kind := range rule.MatchResources.Kinds {
				kindToRules[kind] = append(kindToRules[kind], rule)
			}
		}
	}

	for kind, rules := range kindToRules {
		newPolicy := policy
		newPolicy.Spec.Rules = rules

		resource, _ := generateEmptyResource(validationGlobalState.definitions["io.k8s.api.core.v1."+kind]).(map[string]interface{})
		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)
		policyContext := engine.PolicyContext{
			Policy:      newPolicy,
			NewResource: newResource,
			Context:     context.NewContext(),
		}
		resp := engine.Mutate(policyContext)
		if len(resp.GetSuccessRules()) != len(rules) {
			var errMessages []string
			for _, rule := range resp.PolicyResponse.Rules {
				if rule.Success == false {
					errMessages = append(errMessages, fmt.Sprintf("Invalid rule : %v, %v", rule.Name, rule.Message))
				}
			}
			return fmt.Errorf(strings.Join(errMessages, "\n"))
		}
		err := ValidateResource(resp.PatchedResource.UnstructuredContent(), kind)
		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateResource(patchedResource interface{}, kind string) error {
	if validationGlobalState.isSet == false {
		glog.V(4).Info("Cannot Validate resource: Validation global state not set")
		return nil
	}

	kind = "io.k8s.api.core.v1." + kind

	schema := validationGlobalState.models.LookupModel(kind)
	if schema == nil {
		return fmt.Errorf("pre-validation: couldn't find model %s", kind)
	}

	if errs := validation.ValidateModel(patchedResource, schema, kind); len(errs) > 0 {
		var errorMessages []string
		for i := range errs {
			errorMessages = append(errorMessages, errs[i].Error())
		}

		return fmt.Errorf(strings.Join(errorMessages, "\n\n"))
	}

	return nil
}

func setValidationGlobalState() error {
	if validationGlobalState.isSet == false {
		var err error
		validationGlobalState.document, err = getSchemaDocument()
		if err != nil {
			return err
		}

		validationGlobalState.definitions = make(map[string]*openapi_v2.Schema)

		for _, definition := range validationGlobalState.document.GetDefinitions().AdditionalProperties {
			validationGlobalState.definitions[definition.GetName()] = definition.GetValue()
		}

		validationGlobalState.models, err = proto.NewOpenAPIData(validationGlobalState.document)
		if err != nil {
			return err
		}

		validationGlobalState.isSet = true
	}
	return nil
}

func getSchemaDocument() (*openapi_v2.Document, error) {
	docReq, _ := http.NewRequest("GET", "https://raw.githubusercontent.com/kubernetes/kubernetes/master/api/openapi-spec/swagger.json", nil)
	docReq.Header.Set("accept-encoding", "gzip")
	doc, err := http.DefaultClient.Do(docReq)
	if err != nil {
		return nil, fmt.Errorf("Could not fetch openapi document from the internet, underlying error : %v", err)
	}

	gzipReader, err := gzip.NewReader(doc.Body)
	defer gzipReader.Close()
	if err != nil {
		return nil, err
	}

	var spec yaml.MapSlice
	err = yaml.NewDecoder(gzipReader).Decode(&spec)
	if err != nil {
		return nil, err
	}

	return openapi_v2.NewDocument(spec, compiler.NewContext("$root", nil))
}

func generateEmptyResource(kindSchema *openapi_v2.Schema) interface{} {

	types := kindSchema.GetType().GetValue()

	if kindSchema.GetXRef() != "" {
		return generateEmptyResource(validationGlobalState.definitions[strings.TrimPrefix(kindSchema.GetXRef(), "#/definitions/")])
	}

	if len(types) != 1 {
		return nil
	}

	switch types[0] {
	case "object":
		var props = make(map[string]interface{})
		properties := kindSchema.GetProperties().GetAdditionalProperties()
		if len(properties) == 0 {
			return props
		}

		var wg sync.WaitGroup
		var mutex sync.Mutex
		wg.Add(len(properties))
		for _, property := range properties {
			go func(property *openapi_v2.NamedSchema) {
				prop := generateEmptyResource(property.GetValue())
				mutex.Lock()
				props[property.GetName()] = prop
				mutex.Unlock()
				wg.Done()
			}(property)
		}
		wg.Wait()
		return props
	case "array":
		var array []interface{}
		for _, schema := range kindSchema.GetItems().GetSchema() {
			array = append(array, generateEmptyResource(schema))
		}
		return array
	case "string":
		if kindSchema.GetDefault() != nil {
			return string(kindSchema.GetDefault().Value.Value)
		}
		if kindSchema.GetExample() != nil {
			return string(kindSchema.GetExample().GetValue().Value)
		}
		return ""
	case "integer":
		if kindSchema.GetDefault() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetDefault().Value.Value))
			return val
		}
		if kindSchema.GetExample() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetExample().GetValue().Value))
			return val
		}
		return 0
	case "number":
		if kindSchema.GetDefault() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetDefault().Value.Value))
			return val
		}
		if kindSchema.GetExample() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetExample().GetValue().Value))
			return val
		}
		return 0
	case "boolean":
		if kindSchema.GetDefault() != nil {
			if string(kindSchema.GetDefault().Value.Value) == "true" {
				return true
			}
			return false
		}
		if kindSchema.GetExample() != nil {
			if string(kindSchema.GetExample().GetValue().Value) == "true" {
				return true
			}
			return false
		}
		return false
	}

	return nil
}
