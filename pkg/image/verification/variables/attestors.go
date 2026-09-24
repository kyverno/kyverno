package variables

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/sigstore/sigstore/pkg/signature/kms"
	"k8s.io/apimachinery/pkg/util/validation/field"

	// Register the provider-specific KMS plugins so that kms.SupportedProviders()
	// can recognise KMS key references (e.g. "hashivault://") produced by a key
	// expression. This mirrors the blank imports in the cosign verifier package.
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"
)

// isKMSKeyRef reports whether the given key material is a reference to a key
// managed by one of the registered KMS providers (e.g. "hashivault://",
// "awskms://", "gcpkms://", "azurekms://") rather than an inline PEM-encoded
// public key. Detection mirrors sigstore's own resolution, which selects a
// provider by matching the reference against the registered provider prefixes.
func isKMSKeyRef(ref string) bool {
	for _, provider := range kms.SupportedProviders() {
		if strings.HasPrefix(ref, provider) {
			return true
		}
	}
	return false
}

type CompiledAttestor struct {
	Key               string
	val               v1beta1.Attestor
	keyProg           cel.Program
	certProg          cel.Program
	certChainProg     cel.Program
	notaryCertProg    cel.Program
	notaryTSACertProg cel.Program
	trustedRootProg   cel.Program
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
			if att.Cosign.TrustedRoot != nil && att.Cosign.TrustedRoot.Expression != "" {
				ast, iss := env.Compile(att.Cosign.TrustedRoot.Expression)
				if iss.Err() != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.TrustedRoot, iss.Err().Error()))
				}
				prg, err := env.Program(ast)
				if err != nil {
					return nil, append(allErrs, field.Invalid(path, att.Cosign.TrustedRoot, err.Error()))
				}
				compiledAtt.trustedRootProg = prg
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
	value := c.val.DeepCopy()
	if c.keyProg != nil {
		result, err := evalProgramString(c.Key, c.keyProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert key in compiled attestor: %s, error: %w", c.Key, err)
		}
		// A key expression may resolve to either an inline PEM-encoded public
		// key or a reference to a key held in a KMS (e.g. a per-namespace
		// "hashivault://..." reference). Route KMS references to Key.KMS so
		// cosign resolves them through the KMS provider instead of trying to
		// parse them as inline PEM data.
		if isKMSKeyRef(result) {
			value.Cosign.Key.KMS = result
		} else {
			value.Cosign.Key.Data = result
		}
	}

	if c.certProg != nil {
		result, err := evalProgramString(c.Key, c.certProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		value.Cosign.Certificate.Certificate.Value = result
	}

	if c.certChainProg != nil {
		result, err := evalProgramString(c.Key, c.certChainProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert cert chain in compiled attestor: %s, error: %w", c.Key, err)
		}
		value.Cosign.Certificate.CertificateChain.Value = result
	}

	if c.notaryCertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryCertProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert notary cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		value.Notary.Certs.Value = result
	}

	if c.notaryTSACertProg != nil {
		result, err := evalProgramString(c.Key, c.notaryTSACertProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert notary tsa cert in compiled attestor: %s, error: %w", c.Key, err)
		}
		value.Notary.TSACerts.Value = result
	}

	if c.trustedRootProg != nil {
		result, err := evalProgramString(c.Key, c.trustedRootProg, data)
		if err != nil {
			return v1beta1.Attestor{}, fmt.Errorf("failed to convert trustedRoot in compiled attestor: %s, error: %w", c.Key, err)
		}
		value.Cosign.TrustedRoot.Value = result
	}

	return *value, nil
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
