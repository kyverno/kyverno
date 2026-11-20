package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DialectV1 is the only supported dialect: a documented 2020-12 subset.
const DialectV1 = "JSONSchema202012SubsetV1"

// unsupported keywords are rejected at admission instead of being silently
// ignored, since accepting a keyword with different semantics is worse than
// rejecting it (proposal, "Schema profile and validation").
var unsupported = map[string]bool{
	"$dynamicRef": true, "$dynamicAnchor": true, "$anchor": true, "$id": true,
	"unevaluatedProperties": true, "unevaluatedItems": true, "patternProperties": true,
	"prefixItems": true, "if": true, "then": true, "else": true,
	"dependentSchemas": true, "dependentRequired": true, "propertyNames": true,
	"contains": true, "minContains": true, "maxContains": true,
}

// checkProfile rejects schema content outside the supported subset, reporting
// the JSON Pointer of the offending keyword.
func checkProfile(raw json.RawMessage) error {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	return walkSchema(root, "")
}

func walkSchema(n any, ptr string) error {
	m, ok := n.(map[string]any)
	if !ok {
		return nil // boolean schemas
	}
	for k, v := range m {
		here := ptr + "/" + escapeToken(k)
		switch {
		case unsupported[k]:
			return fmt.Errorf("unsupported keyword %q at %s", k, here)
		case k == "$ref":
			if s, _ := v.(string); !strings.HasPrefix(s, "#/") {
				return fmt.Errorf("only local $ref is supported, got %q at %s", s, here)
			}
		case k == "properties" || k == "$defs" || k == "definitions":
			if sub, ok := v.(map[string]any); ok {
				for name, s := range sub {
					if err := walkSchema(s, here+"/"+escapeToken(name)); err != nil {
						return err
					}
				}
			}
		case k == "items" || k == "additionalProperties" || k == "not":
			if err := walkSchema(v, here); err != nil {
				return err
			}
		case k == "allOf" || k == "anyOf" || k == "oneOf":
			if list, ok := v.([]any); ok {
				for i, s := range list {
					if err := walkSchema(s, fmt.Sprintf("%s/%d", here, i)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
