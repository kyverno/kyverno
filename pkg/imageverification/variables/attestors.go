package variables

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type CompiledAttestor struct {
	Key               string
	val               v1alpha1.Attestor
	keyProg           cel.Program
	certProg          cel.Program
	certChainProg     cel.Program
	notaryCertProg    cel.Program
	notaryTSACertProg cel.Program
}

func CompileAttestors(path *field.Path, att []v1alpha1.Attestor, env *cel.Env) ([]*CompiledAttestor, field.ErrorList) {
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

func (c *CompiledAttestor) Evaluate(data any) (v1alpha1.Attestor, error) {
	if c.keyProg != nil {
		result, err := evalProgramString(c.Key, c.keyProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert key in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Key.Data = result
	}

	if c.certProg != nil {
		result, err := evalProgramString(c.Key, c.certProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Certificate.Certificate.Value = result
	}

	if c.certChainProg != nil {
		result, err := evalProgramString(c.Key, c.certChainProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert chain in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Cosign.Certificate.CertificateChain.Value = result
	}

	if c.notaryCertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryCertProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Notary.Certs.Value = result
	}

	if c.notaryTSACertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryTSACertProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary tsa cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		c.val.Notary.TSACerts.Value = result
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
