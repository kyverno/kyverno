package variables

import (
	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CompileAttestorIdentities compiles CEL expressions for the Subject and
// SubjectRegExp fields of keyless identities in the given attestor.
//
// The provided CEL environment must declare an "image" variable of type string,
// which is made available to expressions at evaluation time.
//
// Returns a CompiledAttestor with identityProgs populated, or nil if the
// attestor has no identity expressions. Returns field errors if any expression
// fails to compile.
//
// NOTE: This function requires the kyverno/api Identity struct to expose
// Subject and SubjectRegExp as *StringOrExpression (see kyverno/api#64).
// Until that API change lands, this function is a no-op.
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

	identPath := path.Child("keyless", "identities")
	hasExpressions := false

	for j, id := range att.Cosign.Keyless.Identities {
		ci := compiledIdentity{index: j}

		// Subject as *StringOrExpression (post kyverno/api#64)
		if id.Subject != nil && id.Subject.Expression != "" {
			ast, iss := env.Compile(id.Subject.Expression)
			if iss.Err() != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subject", "expression"), id.Subject.Expression, iss.Err().Error()))
				continue
			}
			prg, err := env.Program(ast)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subject", "expression"), id.Subject.Expression, err.Error()))
				continue
			}
			ci.subjectProg = prg
			hasExpressions = true
		}

		// SubjectRegExp as *StringOrExpression (post kyverno/api#64)
		if id.SubjectRegExp != nil && id.SubjectRegExp.Expression != "" {
			ast, iss := env.Compile(id.SubjectRegExp.Expression)
			if iss.Err() != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subjectRegExp", "expression"), id.SubjectRegExp.Expression, iss.Err().Error()))
				continue
			}
			prg, err := env.Program(ast)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(identPath.Index(j).Child("subjectRegExp", "expression"), id.SubjectRegExp.Expression, err.Error()))
				continue
			}
			ci.subjectRegExpProg = prg
			hasExpressions = true
		}

		if ci.subjectProg != nil || ci.subjectRegExpProg != nil {
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
