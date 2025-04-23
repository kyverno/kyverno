package variables

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type CompiledAttestor struct {
	key               string
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
			key: att.Name,
			val: att,
		}
		if att.IsCosign() {
			if att.Cosign.Key != nil && att.Cosign.Key.CEL != "" {
				ast, iss := env.Compile(att.Cosign.Key.CEL)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, err.Error()))
				}
				compiledAtt.keyProg = prg
			} else if att.Cosign.Certificate != nil {
				if att.Cosign.Certificate.CertificateCEL != "" {
					ast, iss := env.Compile(att.Cosign.Certificate.CertificateCEL)
					if iss.Err() != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, iss.Err().Error()))
					}
					prg, err := env.Program(ast)
					if err != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, err.Error()))
					}
					compiledAtt.certProg = prg
				}
				if att.Cosign.Certificate.CertificateChainCEL != "" {
					ast, iss := env.Compile(att.Cosign.Certificate.CertificateChainCEL)
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
			if att.Notary.CertsCEL != "" {
				ast, iss := env.Compile(att.Notary.CertsCEL)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, err.Error()))
				}
				compiledAtt.notaryCertProg = prg
			}
			if att.Notary.TSACertsCEL != "" {
				ast, iss := env.Compile(att.Notary.TSACertsCEL)
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
		result, err := evalProgramString(c.key, c.keyProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert key in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Key.Data = result
	}

	if c.certProg != nil {
		result, err := evalProgramString(c.key, c.certProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Certificate.Certificate = result
	}

	if c.certChainProg != nil {
		result, err := evalProgramString(c.key, c.certChainProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert chain in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Certificate.CertificateChain = result
	}

	if c.notaryCertProg != nil {
		result, err := evalProgramString(c.key, c.notaryCertProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Notary.Certs = result
	}

	if c.notaryTSACertProg != nil {
		result, err := evalProgramString(c.key, c.notaryTSACertProg, data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary tsa cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Notary.TSACerts = result
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
