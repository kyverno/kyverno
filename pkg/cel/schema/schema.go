// Package schema is a Phase 0 spike for schema-driven CEL processing
// (docs/dev/proposals/schema-driven-cel-processing.md). It is not wired into
// any binary: it only proves that a JSON Schema document can give typed CEL
// over an arbitrary payload.
package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

const rootTypeName = "payload"

// Computed is a named CEL expression evaluated after validation. Expressions
// see `object` and the `derived` fields declared before them.
type Computed struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// Schema is a compiled payload schema: a validator plus the CEL declarations.
type Schema struct {
	validator *validate.SchemaValidator
	provider  *apiservercel.DeclTypeProvider
	root      *apiservercel.DeclType
	computed  []computedProgram
	env       *cel.Env
}

// New compiles a JSON Schema document (and optional computed fields).
func New(schemaJSON []byte, computed []Computed) (*Schema, error) {
	var s spec.Schema
	if err := json.Unmarshal(schemaJSON, &s); err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	validator := validate.NewSchemaValidator(&s, nil, "", strfmt.Default)
	root := common.SchemaDeclType(&openapi.Schema{Schema: &s}, false)
	if root == nil {
		return nil, fmt.Errorf("schema has no CEL declaration")
	}
	provider := apiservercel.NewDeclTypeProvider(root.MaybeAssignTypeName(rootTypeName))
	base, err := cel.NewEnv(ext.Lists())
	if err != nil {
		return nil, err
	}
	opts, err := provider.EnvOptions(base.CELTypeProvider())
	if err != nil {
		return nil, err
	}
	rootType, ok := provider.FindDeclType(rootTypeName)
	if !ok {
		return nil, fmt.Errorf("root declaration type not found")
	}
	opts = append(opts,
		cel.Variable("object", rootType.CelType()),
		cel.Variable("derived", cel.MapType(cel.StringType, cel.DynType)),
	)
	env, err := base.Extend(opts...)
	if err != nil {
		return nil, err
	}
	out := &Schema{validator: validator, provider: provider, root: rootType, env: env}
	for _, c := range computed {
		prog, err := out.program(c.Expression)
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", c.Name, err)
		}
		out.computed = append(out.computed, computedProgram{name: c.Name, prog: prog})
	}
	return out, nil
}
