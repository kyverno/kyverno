package variables

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// compiledIdentity holds compiled CEL programs for a single keyless identity entry.
// It corresponds to a v1beta1.Identity whose Subject or SubjectRegExp fields
// contain a CEL expression (via StringOrExpression) rather than a static string.
type compiledIdentity struct {
	// index is the position in Keyless.Identities this entry corresponds to.
	index             int
	subjectProg       cel.Program
	subjectRegExpProg cel.Program
}

type CompiledAttestor struct {
	Key               string
	val               v1beta1.Attestor
	keyProg           cel.Program
	certProg          cel.Program
	certChainProg     cel.Program
	notaryCertProg    cel.Program
	notaryTSACertProg cel.Program
	// identityProgs holds compiled CEL programs for keyless identity fields.
	// Populated when Identity.Subject or Identity.SubjectRegExp is a StringOrExpression
	// with a non-empty Expression field.
	identityProgs []compiledIdentity
}

func CompileAttestors(path *field.Path, att []v1beta1.Attestor, env *cel.Env) ([]*CompiledAttestor, field.ErrorList) {
	var allErrs field.ErrorList
	compiledAttestors := make([]*CompiledAttestor, 0, len(att))
	for i, att := range att {
		path := path.Index(i)
		compiledAtt := &CompiledAttestor{
			Key: att.Name,
			val: att,
		}
		if att.IsCosign() {
			if att.Cosign.Key != nil && att.Cosign.Key.Expression != "" {
				ast, iss := env.Compile(att.Cosign.Key.Expression)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, err.Error()))
				}
				compiledAtt.keyProg = prg
			} else if att.Cosign.Certificate != nil {
				if att.Cosign.Certificate.Certificate != nil && att.Cosign.Certificate.Certificate.Expression != "" {
					ast, iss := env.Compile(att.Cosign.Certificate.Certificate.Expression)
					if iss.Err() != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, iss.Err().Error()))
					}
					prg, err := env.Program(ast)
					if err != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, err.Error()))
					}
					compiledAtt.certProg = prg
				}
				if att.Cosign.Certificate.CertificateChain != nil && att.Cosign.Certificate.CertificateChain.Expression != "" {
					ast, iss := env.Compile(att.Cosign.Certificate.CertificateChain.Expression)
					if iss.Err() != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, iss.Err().Error()))
					}
					prg, err := env.Program(ast)
					if err != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, err.Error()))
					}
					compiledAtt.certChainProg = prg
				}
			}
			// Compile CEL expressions for keyless identity fields.
			// NOTE: This requires the kyverno/api Identity struct to have
			// Subject and SubjectRegExp as *StringOrExpression (see kyverno/api#64).
			if att.Cosign.Keyless != nil {
				identPath := path.Child("keyless", "identities")
				for j, id := range att.Cosign.Keyless.Identities {
					ci := compiledIdentity{index: j}
					if id.Subject != nil && id.Subject.Expression != "" {
						ast, iss := env.Compile(id.Subject.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(identPath.Index(j).Child("subject"), id.Subject, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(identPath.Index(j).Child("subject"), id.Subject, err.Error()))
						}
						ci.subjectProg = prg
					}
					if id.SubjectRegExp != nil && id.SubjectRegExp.Expression != "" {
						ast, iss := env.Compile(id.SubjectRegExp.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(identPath.Index(j).Child("subjectRegExp"), id.SubjectRegExp, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(identPath.Index(j).Child("subjectRegExp"), id.SubjectRegExp, err.Error()))
						}
						ci.subjectRegExpProg = prg
					}
					if ci.subjectProg != nil || ci.subjectRegExpProg != nil {
						compiledAtt.identityProgs = append(compiledAtt.identityProgs, ci)
					}
				}
			}
		} else if att.IsNotary() {
			if att.Notary.Certs != nil && att.Notary.Certs.Expression != "" {
				ast, iss := env.Compile(att.Notary.Certs.Expression)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, err.Error()))
				}
				compiledAtt.notaryCertProg = prg
			}
			if att.Notary.TSACerts != nil && att.Notary.TSACerts.Expression != "" {
				ast, iss := env.Compile(att.Notary.TSACerts.Expression)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, err.Error()))
				}
				compiledAtt.notaryTSACertProg = prg
			}
		}
		compiledAttestors = append(compiledAttestors, compiledAtt)
	}
	return compiledAttestors, nil
}

func (c *CompiledAttestor) Evaluate(data any) (v1beta1.Attestor, error) {
	if c.keyProg != nil {
		result, err := evalProgramString(c.Key, c.keyProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert key in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Key.Data = result
	}

	if c.certProg != nil {
		result, err := evalProgramString(c.Key, c.certProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Certificate.Certificate.Value = result
	}

	if c.certChainProg != nil {
		result, err := evalProgramString(c.Key, c.certChainProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert cert chain in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Certificate.CertificateChain.Value = result
	}

	if c.notaryCertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryCertProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert notary cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Notary.Certs.Value = result
	}

	if c.notaryTSACertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryTSACertProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert notary tsa cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Notary.TSACerts.Value = result
	}

	return c.val, nil
}

// EvaluateWithImage evaluates the compiled attestor with the given data map and
// additionally evaluates any CEL expressions in keyless identity fields using
// the provided image reference string. The image is made available as the
// "image" variable in the CEL evaluation context for identity expressions.
//
// This method should be used instead of Evaluate when verifying a specific image,
// so that identity subject/subjectRegExp expressions can reference the image.
func (c *CompiledAttestor) EvaluateWithImage(data any, image string) (v1beta1.Attestor, error) {
	att, err := c.Evaluate(data)
	if err != nil {
		return v1beta1.Attestor{}, err
	}

	if len(c.identityProgs) == 0 || att.Cosign == nil || att.Cosign.Keyless == nil {
		return att, nil
	}

	// Build evaluation data with the image reference available.
	imageData := map[string]any{"image": image}

	for _, ci := range c.identityProgs {
		if ci.index >= len(att.Cosign.Keyless.Identities) {
			continue
		}
		id := &att.Cosign.Keyless.Identities[ci.index]

		if ci.subjectProg != nil {
			result, err := evalProgramString(c.Key, ci.subjectProg, imageData)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate subject expression in identity[%d] for attestor %q: %w", ci.index, c.Key, err)
			}
			// NOTE: After kyverno/api#64, id.Subject will be *StringOrExpression.
			// Set the resolved value so checkOptions can read it.
			if id.Subject != nil {
				id.Subject.Value = result
			}
		}

		if ci.subjectRegExpProg != nil {
			result, err := evalProgramString(c.Key, ci.subjectRegExpProg, imageData)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate subjectRegExp expression in identity[%d] for attestor %q: %w", ci.index, c.Key, err)
			}
			// NOTE: After kyverno/api#64, id.SubjectRegExp will be *StringOrExpression.
			if id.SubjectRegExp != nil {
				id.SubjectRegExp.Value = result
			}
		}
	}

	return att, nil
}

func evalProgramString(key string, e cel.Program, data any) (string, error) {
	v, _, err := e.Eval(data)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", key, err)
	}
	result, err := utils.ConvertToNative[string](v)
	if err != nil {
		return "", fmt.Errorf("failed to convert expression in compiled attestor: %s, error: %w", key, err)
	}
	return result, nil
}
