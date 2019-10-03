package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type anchor struct {
	left  string
	right string
}

var (
	conditionalAnchor = anchor{left: "(", right: ")"}
	existingAnchor    = anchor{left: "^(", right: ")"}
	equalityAnchor    = anchor{left: "=(", right: ")"}
	plusAnchor        = anchor{left: "+(", right: ")"}
)

func (p ClusterPolicy) Validate() error {
	var errs []error

	if err := p.ValidateUniqueRuleName(); err != nil {
		errs = append(errs, fmt.Errorf("- Invalid Policy '%s':", p.Name))
		errs = append(errs, err)
	}

	for _, rule := range p.Spec.Rules {
		if ruleErrs := rule.Validate(); len(ruleErrs) != 0 {
			errs = append(errs, fmt.Errorf("- invalid rule '%s':", rule.Name))
			errs = append(errs, ruleErrs...)
		}
	}

	return joinErrs(errs)
}

// ValidateUniqueRuleName checks if the rule names are unique across a policy
func (p ClusterPolicy) ValidateUniqueRuleName() error {
	var ruleNames []string

	for _, rule := range p.Spec.Rules {
		if containString(ruleNames, rule.Name) {
			return fmt.Errorf(`duplicate rule name: '%s'`, rule.Name)
		}
		ruleNames = append(ruleNames, rule.Name)
	}
	return nil
}

// Validate checks if rule is not empty and all substructures are valid
func (r Rule) Validate() []error {
	var errs []error

	// only one type of rule is allowed per rule
	if err := r.ValidateRuleType(); err != nil {
		errs = append(errs, err)
	}

	// validate resource description block
	if err := r.MatchResources.ResourceDescription.Validate(true); err != nil {
		errs = append(errs, fmt.Errorf("error in match block, %v", err))
	}

	if err := r.ExcludeResources.ResourceDescription.Validate(false); err != nil {
		errs = append(errs, fmt.Errorf("error in exclude block, %v", err))
	}

	// validate anchors on mutate
	if mErrs := r.Mutation.Validate(); len(mErrs) != 0 {
		errs = append(errs, mErrs...)
	}

	if vErrs := r.Validation.Validate(); len(vErrs) != 0 {
		errs = append(errs, vErrs...)
	}

	if err := r.Generation.Validate(); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// validateRuleType checks only one type of rule is defined per rule
func (r Rule) ValidateRuleType() error {
	ruleTypes := []bool{r.HasMutate(), r.HasValidate(), r.HasGenerate()}

	operationCount := func() int {
		count := 0
		for _, v := range ruleTypes {
			if v {
				count++
			}
		}
		return count
	}()

	if operationCount == 0 {
		return fmt.Errorf("no operation defined in the rule '%s'.(supported operations: mutation,validation,generation,query)", r.Name)
	} else if operationCount != 1 {
		return fmt.Errorf("multiple operations defined in the rule '%s', only one type of operation is allowed per rule", r.Name)
	}
	return nil
}

func (r Rule) HasMutate() bool {
	return !reflect.DeepEqual(r.Mutation, Mutation{})
}

func (r Rule) HasValidate() bool {
	return !reflect.DeepEqual(r.Validation, Validation{})
}

func (r Rule) HasGenerate() bool {
	return !reflect.DeepEqual(r.Generation, Generation{})
}

// Validate checks if all necesarry fields are present and have values. Also checks a Selector.
// field type is checked through openapi
// Returns error if
// - kinds is empty array in matched resource block, i.e. kinds: []
// - selector is invalid
func (rd ResourceDescription) Validate(matchedResource bool) error {
	if reflect.DeepEqual(rd, ResourceDescription{}) {
		return nil
	}

	if matchedResource && len(rd.Kinds) == 0 {
		return errors.New("field Kind is not specified")
	}

	if rd.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(rd.Selector)
		if err != nil {
			return err
		}
		requirements, _ := selector.Requirements()
		if len(requirements) == 0 {
			return errors.New("the requirements are not specified in selector")
		}
	}

	return nil
}

func (m Mutation) Validate() []error {
	var errs []error
	if len(m.Patches) != 0 {
		for _, patch := range m.Patches {
			err := patch.Validate()
			errs = append(errs, err)
		}
	}

	if m.Overlay != nil {
		_, err := validateAnchors([]anchor{conditionalAnchor, plusAnchor}, m.Overlay, "/")
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Validate if all mandatory PolicyPatch fields are set
func (pp *Patch) Validate() error {
	if pp.Path == "" {
		return errors.New("JSONPatch field 'path' is mandatory")
	}

	if pp.Operation == "add" || pp.Operation == "replace" {
		if pp.Value == nil {
			return fmt.Errorf("JSONPatch field 'value' is mandatory for operation '%s'", pp.Operation)
		}

		return nil
	} else if pp.Operation == "remove" {
		return nil
	}

	return fmt.Errorf("Unsupported JSONPatch operation '%s'", pp.Operation)
}

func (v Validation) Validate() []error {
	var errs []error

	if err := v.ValidateOverlayPattern(); err != nil {
		errs = append(errs, err)
	}

	if v.Pattern != nil {
		if _, err := validateAnchors([]anchor{conditionalAnchor, existingAnchor, equalityAnchor}, v.Pattern, "/"); err != nil {
			errs = append(errs, err)
		}
	}

	if len(v.AnyPattern) != 0 {
		for _, p := range v.AnyPattern {
			if _, err := validateAnchors([]anchor{conditionalAnchor, existingAnchor, equalityAnchor}, p, "/"); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// validateOverlayPattern checks one of pattern/anyPattern must exist
func (v Validation) ValidateOverlayPattern() error {
	if reflect.DeepEqual(v, Validation{}) {
		return nil
	}

	if v.Pattern == nil && len(v.AnyPattern) == 0 {
		return fmt.Errorf("neither pattern nor anyPattern found")
	}

	if v.Pattern != nil && len(v.AnyPattern) != 0 {
		return fmt.Errorf("either pattern or anyPattern is allowed")
	}

	return nil
}

// Validate returns error if generator is configured incompletely
func (gen Generation) Validate() error {
	if reflect.DeepEqual(gen, Generation{}) {
		return nil
	}

	if gen.Data == nil && gen.Clone == (CloneFrom{}) {
		return fmt.Errorf("neither data nor clone (source) of %s is specified", gen.Kind)
	}
	if gen.Data != nil && gen.Clone != (CloneFrom{}) {
		return fmt.Errorf("both data nor clone (source) of %s are specified", gen.Kind)
	}

	if gen.Data != nil {
		if _, err := validateAnchors(nil, gen.Data, "/"); err != nil {
			return fmt.Errorf("anchors are not allowed on generate pattern data: %v", err)
		}
	}

	if !reflect.DeepEqual(gen.Clone, CloneFrom{}) {
		if _, err := validateAnchors(nil, gen.Clone, ""); err != nil {
			return fmt.Errorf("invalid character found on pattern clone: %v", err)
		}
	}

	return nil
}

// validateAnchors validates:
// 1. existing acnchor must define on array
// 2. anchors in mutation must be one of: (), +()
// 3. anchors in validate must be one of: (), ^(), =()
// 4. no anchor is allowed in generate
func validateAnchors(anchorPatterns []anchor, pattern interface{}, path string) (string, error) {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		return validateAnchorsOnMap(anchorPatterns, typedPattern, path)
	case []interface{}:
		return validateAnchorsOnArray(anchorPatterns, typedPattern, path)
	case string, float64, int, int64, bool, nil:
		// check on type string
		if checkedPattern := reflect.ValueOf(pattern); checkedPattern.Kind() == reflect.String {
			if hasAnchor, str := hasExistingAnchor(checkedPattern.String()); hasAnchor {
				return path, fmt.Errorf("existing anchor at %s must be of type array, found: %v", path+str, checkedPattern.Kind())
			}
		}
		// return nil on all other cases
		return "", nil
	case interface{}:
		// special case for generate clone, as it is a struct
		if clone, ok := pattern.(CloneFrom); ok {
			return "", validateAnchorsOnCloneFrom(nil, clone)
		}
		return "", nil
	default:
		glog.V(4).Infof("Pattern contains unknown type %T. Path: %s", pattern, path)
		return path, fmt.Errorf("pattern contains unknown type, path: %s", path)
	}
}

func validateAnchorsOnCloneFrom(anchorPatterns []anchor, pattern CloneFrom) error {
	// namespace and name are required fields
	// if wrapped with invalid character, this field is empty during unmarshaling
	if pattern.Namespace == "" {
		return errors.New("namespace is requried")
	}

	if pattern.Name == "" {
		return errors.New("name is requried")
	}

	return nil
}

func validateAnchorsOnMap(anchorPatterns []anchor, pattern map[string]interface{}, path string) (string, error) {
	for key, patternElement := range pattern {
		if valid, str := hasValidAnchors(anchorPatterns, key); !valid {
			return path, fmt.Errorf("invalid anchor found at %s, expect: %s", path+str, joinAnchors(anchorPatterns))
		}
		if hasAnchor, str := hasExistingAnchor(key); hasAnchor {
			if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() != reflect.Slice {
				return path, fmt.Errorf("existing anchor at %s must be of type array, found: %T", path+str, patternElement)
			}
		}

		if path, err := validateAnchors(anchorPatterns, patternElement, path+key+"/"); err != nil {
			return path, err
		}
	}

	return "", nil
}

func validateAnchorsOnArray(anchorPatterns []anchor, patternArray []interface{}, path string) (string, error) {
	if len(patternArray) == 0 {
		return path, fmt.Errorf("pattern array at %s is empty", path)
	}

	for i, pattern := range patternArray {
		currentPath := path + strconv.Itoa(i) + "/"
		if path, err := validateAnchors(anchorPatterns, pattern, currentPath); err != nil {
			return path, err
		}
	}

	return "", nil
}
