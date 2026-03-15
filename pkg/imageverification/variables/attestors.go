package variables

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type compiledIdentity struct {
	issuerProg        cel.Program
	subjectProg       cel.Program
	issuerRegExpProg  cel.Program
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
	identityProgs     []compiledIdentity
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
			if att.Cosign.Keyless != nil {
				for _, id := range att.Cosign.Keyless.Identities {
					compiled := compiledIdentity{}
					if id.Issuer != nil && id.Issuer.Expression != "" {
						ast, iss := env.Compile(id.Issuer.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(path, id.Issuer, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(path, id.Issuer, err.Error()))
						}
						compiled.issuerProg = prg
					}
					if id.Subject != nil && id.Subject.Expression != "" {
						ast, iss := env.Compile(id.Subject.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(path, id.Subject, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(path, id.Subject, err.Error()))
						}
						compiled.subjectProg = prg
					}
					if id.IssuerRegExp != nil && id.IssuerRegExp.Expression != "" {
						ast, iss := env.Compile(id.IssuerRegExp.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(path, id.IssuerRegExp, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(path, id.IssuerRegExp, err.Error()))
						}
						compiled.issuerRegExpProg = prg
					}
					if id.SubjectRegExp != nil && id.SubjectRegExp.Expression != "" {
						ast, iss := env.Compile(id.SubjectRegExp.Expression)
						if iss.Err() != nil {
							return nil, append(allErrs, field.Invalid(path, id.SubjectRegExp, iss.Err().Error()))
						}
						prg, err := env.Program(ast)
						if err != nil {
							return nil, append(allErrs, field.Invalid(path, id.SubjectRegExp, err.Error()))
						}
						compiled.subjectRegExpProg = prg
					}
					compiledAtt.identityProgs = append(compiledAtt.identityProgs, compiled)
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

	for i, idProg := range c.identityProgs {
		if idProg.issuerProg != nil {
			result, err := evalProgramString(c.Key, idProg.issuerProg, data)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate issuer expression in identity %d of attestor: %s, error: %w", i, c.Key, err)
			}
			c.val.Cosign.Keyless.Identities[i].Issuer.Value = result
		}
		if idProg.subjectProg != nil {
			result, err := evalProgramString(c.Key, idProg.subjectProg, data)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate subject expression in identity %d of attestor: %s, error: %w", i, c.Key, err)
			}
			c.val.Cosign.Keyless.Identities[i].Subject.Value = result
		}
		if idProg.issuerRegExpProg != nil {
			result, err := evalProgramString(c.Key, idProg.issuerRegExpProg, data)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate issuerRegExp expression in identity %d of attestor: %s, error: %w", i, c.Key, err)
			}
			c.val.Cosign.Keyless.Identities[i].IssuerRegExp.Value = result
		}
		if idProg.subjectRegExpProg != nil {
			result, err := evalProgramString(c.Key, idProg.subjectRegExpProg, data)
			if err != nil {
				return v1beta1.Attestor{}, fmt.Errorf("failed to evaluate subjectRegExp expression in identity %d of attestor: %s, error: %w", i, c.Key, err)
			}
			c.val.Cosign.Keyless.Identities[i].SubjectRegExp.Value = result
		}
	}

	return c.val, nil
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
