package credentials

import (
	"reflect"
	"strings"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ScopePolicy returns a private copy of a namespaced policy with all secret
// references confined to its namespace. Cluster policies keep their configured
// references and defaults.
func ScopePolicy(policy policiesv1beta1.ImageValidatingPolicyLike) (policiesv1beta1.ImageValidatingPolicyLike, field.ErrorList) {
	if errs := validatePolicyNamespace(policy); len(errs) != 0 {
		return nil, errs
	}
	if policy.GetKind() != policieskyvernoio.NamespacedImageValidatingPolicyKind {
		return policy, nil
	}

	scoped := policy.DeepCopyObject().(policiesv1beta1.ImageValidatingPolicyLike)
	spec := scoped.GetSpec()
	var errs field.ErrorList
	if spec.Credentials != nil {
		secrets, err := kubeutils.ScopeSecretReferences(spec.Credentials.Secrets, scoped.GetNamespace())
		if err != nil {
			errs = append(errs, field.Forbidden(field.NewPath("spec", "credentials", "secrets"), err.Error()))
		} else {
			spec.Credentials.Secrets = secrets
		}
	}
	attestors, attestorErrs := ScopeAttestors(scoped, spec.Attestors)
	errs = append(errs, attestorErrs...)
	if len(errs) != 0 {
		return nil, errs
	}
	spec.Attestors = attestors
	return scoped, nil
}

// ScopeAttestors also checks attestors passed directly to CEL verification
// functions, which need not come from spec.attestors. It never changes its input.
func ScopeAttestors(policy policiesv1beta1.ImageValidatingPolicyLike, attestors []policiesv1beta1.Attestor) ([]policiesv1beta1.Attestor, field.ErrorList) {
	if errs := validatePolicyNamespace(policy); len(errs) != 0 {
		return nil, errs
	}
	if policy.GetKind() != policieskyvernoio.NamespacedImageValidatingPolicyKind {
		return attestors, nil
	}

	scoped := make([]policiesv1beta1.Attestor, len(attestors))
	var errs field.ErrorList
	for i := range attestors {
		scoped[i] = *attestors[i].DeepCopy()
		if scoped[i].Cosign == nil || scoped[i].Cosign.Source == nil {
			continue
		}
		source := scoped[i].Cosign.Source
		references := make([]string, len(source.SignaturePullSecrets))
		for j, reference := range source.SignaturePullSecrets {
			references[j] = reference.Name
		}
		secrets, err := kubeutils.ScopeSecretReferences(references, policy.GetNamespace())
		if err != nil {
			errs = append(errs, field.Forbidden(field.NewPath("spec", "attestors").Index(i).Child("cosign", "source", "PullSecrets"), err.Error()))
			continue
		}
		for j, secret := range secrets {
			source.SignaturePullSecrets[j].Name = secret
		}
	}
	return scoped, errs
}

func validatePolicyNamespace(policy policiesv1beta1.ImageValidatingPolicyLike) field.ErrorList {
	if policy == nil || (reflect.ValueOf(policy).Kind() == reflect.Pointer && reflect.ValueOf(policy).IsNil()) {
		return field.ErrorList{field.Required(field.NewPath("policy"), "image verification policy must not be nil")}
	}
	if policy.GetKind() == policieskyvernoio.NamespacedImageValidatingPolicyKind {
		if policy.GetNamespace() == "" {
			return field.ErrorList{field.Required(field.NewPath("metadata", "namespace"), "namespaced image verification policy namespace must not be empty")}
		}
		if messages := validation.IsDNS1123Label(policy.GetNamespace()); len(messages) != 0 {
			return field.ErrorList{field.Invalid(field.NewPath("metadata", "namespace"), policy.GetNamespace(), strings.Join(messages, "; "))}
		}
	}
	return nil
}
