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

func CompileAttestors(path *field.Path, att []v1alpha1.Attestor, envOpts []cel.EnvOption) ([]*CompiledAttestor, field.ErrorList) {
	var allErrs field.ErrorList
	var compiledAttestors []*CompiledAttestor
	e, err := cel.NewEnv(envOpts...)
	if err != nil {
		return nil, append(allErrs, field.Invalid(path, att, err.Error()))
	}
	for i, att := range att {
		path := path.Index(i).Child("expression")
		compiledAtt := &CompiledAttestor{
			key: att.Name,
		}
		if att.IsCosign() {
			if att.Cosign.Key != nil && att.Cosign.Key.CEL != "" {
				ast, iss := e.Compile(att.Cosign.Key.CEL)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, iss.Err().Error()))
				}
				prg, err := e.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.Key, err.Error()))
				}
				compiledAtt.keyProg = prg
			} else if att.Cosign.Certificate != nil {
				if att.Cosign.Certificate.CertificateCEL != "" {
					ast, iss := e.Compile(att.Cosign.Certificate.CertificateCEL)
					if iss.Err() != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, iss.Err().Error()))
					}
					prg, err := e.Program(ast)
					if err != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, err.Error()))
					}
					compiledAtt.certProg = prg
				}
				if att.Cosign.Certificate.CertificateChainCEL != "" {
					ast, iss := e.Compile(att.Cosign.Certificate.CertificateChainCEL)
					if iss.Err() != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, iss.Err().Error()))
					}
					prg, err := e.Program(ast)
					if err != nil {
						return nil, append(allErrs, field.Invalid(path, att.Cosign.Certificate, err.Error()))
					}
					compiledAtt.certChainProg = prg
				}
			}
		} else if att.IsNotary() {
			if att.Notary.CertificateCEL != "" {
				ast, iss := e.Compile(att.Notary.CertificateCEL)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, iss.Err().Error()))
				}
				prg, err := e.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, err.Error()))
				}
				compiledAtt.notaryCertProg = prg
			}
			if att.Notary.TSACertificateCEL != "" {
				ast, iss := e.Compile(att.Notary.TSACertificateCEL)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Notary, iss.Err().Error()))
				}
				prg, err := e.Program(ast)
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
		v, _, err := c.keyProg.Eval(data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", c.key, err)
		}
		result, err := utils.ConvertToNative[string](v)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert key in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Key.Data = result
	}

	if c.certProg != nil {
		v, _, err := c.certProg.Eval(data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", c.key, err)
		}
		result, err := utils.ConvertToNative[string](v)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Certificate.Certificate = result
	}

	if c.certChainProg != nil {
		v, _, err := c.certChainProg.Eval(data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", c.key, err)
		}
		result, err := utils.ConvertToNative[string](v)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert cert chain in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Cosign.Certificate.CertificateChain = result
	}

	if c.notaryCertProg != nil {
		v, _, err := c.notaryCertProg.Eval(data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", c.key, err)
		}
		result, err := utils.ConvertToNative[string](v)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Notary.Certs = result
	}

	if c.notaryTSACertProg != nil {
		v, _, err := c.notaryTSACertProg.Eval(data)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to evaluate compiled attestor: %s, error: %w", c.key, err)
		}
		result, err := utils.ConvertToNative[string](v)
		if err != nil {
			return v1alpha1.Attestor{}, fmt.Errorf("failed to convert notary tsa cert in compiled attestor: %s, error: %w", c.key, err)
		}
		c.val.Notary.TSACerts = result
	}

	return c.val, nil
}
