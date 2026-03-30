package variables

import (
	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CompileAttestorIdentities compiles CEL expressions for the SubjectExpression
// field of keyless identities in the given attestor.
//
// The provided CEL environment must declare an "image" variable of type string,
// which is made available to expressions at evaluation time.
//
// Returns a CompiledAttestor with identityProgs populated, or nil if the
// attestor has no identity expressions. Returns field errors if any expression
// fails to compile.
func CompileAttestorIdentities(
	path *field.Path,
	att *v1beta1.Attestor,
	env *cel.Env,
) (*CompiledAttestor, field.ErrorList) {
	if att == nil || !att.IsCosign() || att.Cosign.Keyless == nil {
		return nil, nil
	}

	var allErrs field.ErrorList
	compiled := &CompiledAttestor{
		Key: att.Name,
		val: *att,
	}

	identPath := path.Child("cosign", "keyless", "identities")
	hasExpressions := false

	for j, id := range att.Cosign.Keyless.Identities {
		ci := compiledIdentity{index: j}

		if id.SubjectExpression != "" {
			ast, iss := env.Compile(id.SubjectExpression)
			if iss.Err() != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subjectExpression"), id.SubjectExpression, iss.Err().Error()))
				continue
			}
			prg, err := env.Program(ast)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subjectExpression"), id.SubjectExpression, err.Error()))
				continue
			}
			ci.subjectExprProg = prg
			hasExpressions = true
		}

		if ci.subjectExprProg != nil {
			compiled.identityProgs = append(compiled.identityProgs, ci)
		}
	}

	if len(allErrs) > 0 {
		return nil, allErrs
	}
	if !hasExpressions {
		return nil, nil
	}
	return compiled, nil
}
