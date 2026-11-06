package autogen

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	ivpol = &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{
						"team": "platform",
					},
				},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{
					Glob: "ghcr.io/*",
				},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{
					Name:       "containers",
					Expression: "object.spec.containers.map(e, e.image)",
				},
			},
			Attestors: []policiesv1beta1.Attestor{
				{
					Name: "notary",
					Notary: &policiesv1beta1.Notary{
						Certs: &policiesv1beta1.StringOrExpression{
							Value: `-----BEGIN CERTIFICATE----------END CERTIFICATE-----`,
						},
					},
				},
			},
			Attestations: []policiesv1beta1.Attestation{
				{
					Name: "sbom",
					Referrer: &policiesv1beta1.Referrer{
						Type: "sbom/cyclone-dx",
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "images.bar.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)",
					Message:    "failed to verify image with notary cert",
				},
			},
			AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{
						"cronjobs",
					},
				},
			},
		},
	}
)

func TestRewriteExceptions(t *testing.T) {
	newException := func(expression, validationExpr, messageExpr string) *policiesv1beta1.PolicyException {
		polex := &policiesv1beta1.PolicyException{
			ObjectMeta: v1.ObjectMeta{Name: "exception"},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{
					{Name: "policy", Kind: "ImageValidatingPolicy"},
				},
			},
		}
		if expression != "" {
			polex.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
				{Name: "match", Expression: expression},
			}
		}
		if validationExpr != "" || messageExpr != "" {
			polex.Spec.Validations = []admissionregistrationv1.Validation{
				{Expression: validationExpr, MessageExpression: messageExpr},
			}
		}
		return polex
	}
	twoConditions := newException("object.spec.containers.size() > 0", "", "")
	twoConditions.Spec.MatchConditions = append(twoConditions.Spec.MatchConditions,
		admissionregistrationv1.MatchCondition{Name: "second", Expression: "object.spec.initContainers.size() == 0"})

	tests := []struct {
		name       string
		exceptions []*policiesv1beta1.PolicyException
		config     string
		want       []string
	}{
		{
			name:       "every match condition is rewritten, not only the first",
			exceptions: []*policiesv1beta1.PolicyException{twoConditions},
			config:     autogen.AutogenDefaults,
			want:       []string{"object.spec.template.spec.containers.size() > 0", "object.spec.template.spec.initContainers.size() == 0"},
		},
		{
			name:       "defaults: containers expression is rewritten for deployments",
			exceptions: []*policiesv1beta1.PolicyException{newException("object.spec.containers.exists(c, c.name == 'nginx')", "", "")},
			config:     autogen.AutogenDefaults,
			want:       []string{"object.spec.template.spec.containers.exists(c, c.name == 'nginx')"},
		},
		{
			name:       "cronjobs: containers expression is rewritten",
			exceptions: []*policiesv1beta1.PolicyException{newException("object.spec.containers.exists(c, c.name == 'nginx')", "", "")},
			config:     autogen.AutogenCronjobs,
			want:       []string{"object.spec.jobTemplate.spec.template.spec.containers.exists(c, c.name == 'nginx')"},
		},
		{
			name:       "extract: expressions are not rewritten",
			exceptions: []*policiesv1beta1.PolicyException{newException("object.spec.containers.exists(c, c.name == 'nginx')", "", "")},
			config:     autogen.ExtractionReplacementsRef,
			want:       []string{"object.spec.containers.exists(c, c.name == 'nginx')"},
		},
		{
			name:       "metadata is preserved while spec is rewritten",
			exceptions: []*policiesv1beta1.PolicyException{newException("object.metadata.labels.app == 'nginx' && object.spec.containers.size() > 0", "", "")},
			config:     autogen.AutogenDefaults,
			want:       []string{"object.metadata.labels.app == 'nginx' && object.spec.template.spec.containers.size() > 0"},
		},
		{
			name:       "validations are left untouched",
			exceptions: []*policiesv1beta1.PolicyException{newException("", "object.spec.containers.exists(c, c.name == 'nginx')", "oldObject.spec.containers.exists(c, c.name == 'nginx') ? 'yes' : 'no'")},
			config:     autogen.AutogenDefaults,
			want:       []string{"object.spec.containers.exists(c, c.name == 'nginx')", "oldObject.spec.containers.exists(c, c.name == 'nginx') ? 'yes' : 'no'"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewritten, err := RewriteExceptions(test.exceptions, test.config)
			assert.NoError(t, err)
			got := make([]string, 0)
			for _, polex := range rewritten {
				for _, condition := range polex.Spec.MatchConditions {
					got = append(got, condition.Expression)
				}
				if len(polex.Spec.Validations) > 0 {
					if polex.Spec.Validations[0].Expression != "" {
						got = append(got, polex.Spec.Validations[0].Expression)
					}
					if polex.Spec.Validations[0].MessageExpression != "" {
						got = append(got, polex.Spec.Validations[0].MessageExpression)
					}
				}
			}
			assert.Equal(t, test.want, got)
		})
	}

	t.Run("original exceptions are not mutated", func(t *testing.T) {
		exceptions := []*policiesv1beta1.PolicyException{newException("object.spec.containers.exists(c, c.name == 'nginx')", "", "")}
		_, err := RewriteExceptions(exceptions, autogen.AutogenDefaults)
		assert.NoError(t, err)
		assert.Equal(t, "object.spec.containers.exists(c, c.name == 'nginx')", exceptions[0].Spec.MatchConditions[0].Expression)
	})

	t.Run("empty exceptions returns as-is", func(t *testing.T) {
		rewritten, err := RewriteExceptions(nil, autogen.AutogenDefaults)
		assert.NoError(t, err)
		assert.Nil(t, rewritten)
	})

	t.Run("no replacements returns a slice independent of the input", func(t *testing.T) {
		exceptions := []*policiesv1beta1.PolicyException{newException("object.spec.containers.size() > 0", "", "")}
		rewritten, err := RewriteExceptions(exceptions, autogen.ExtractionReplacementsRef)
		assert.NoError(t, err)
		rewritten[0] = nil
		assert.NotNil(t, exceptions[0])
	})
}

func Test_AutogenImageVerify(t *testing.T) {
	cronRule := []admissionregistrationv1.NamedRuleWithOperations{
		{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"batch"},
					APIVersions: []string{"v1"},
					Resources:   []string{"cronjobs"},
				},
			},
		},
	}
	podctrl := []admissionregistrationv1.NamedRuleWithOperations{
		{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments", "statefulsets"},
				},
			},
		},
	}
	cronimg := []policiesv1beta1.ImageExtractor{
		{
			Name:       "containers",
			Expression: "object.spec.jobTemplate.spec.template.spec.containers.map(e, e.image)",
		},
	}
	podctrlimg := []policiesv1beta1.ImageExtractor{
		{
			Name:       "containers",
			Expression: "object.spec.template.spec.containers.map(e, e.image)",
		},
	}
	autogenerated, err := Autogen(ivpol)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(autogenerated))
	assert.Equal(t, cronRule, autogenerated[autogen.AutogenCronjobs].Spec.MatchConstraints.ResourceRules)
	assert.Equal(t, ivpol.Spec.MatchConstraints.NamespaceSelector, autogenerated[autogen.AutogenCronjobs].Spec.MatchConstraints.NamespaceSelector)
	assert.Equal(t, 1, len(autogenerated[autogen.AutogenCronjobs].Spec.ImageExtractors))
	assert.Equal(t, cronimg, autogenerated[autogen.AutogenCronjobs].Spec.ImageExtractors)
	pol := ivpol.DeepCopy()
	pol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{
		"cronjobs",
		"deployments",
		"statefulsets",
	}
	autogenerated, err = Autogen(pol)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(autogenerated))
	assert.Equal(t, podctrl, autogenerated[autogen.AutogenDefaults].Spec.MatchConstraints.ResourceRules)
	assert.Equal(t, pol.Spec.MatchConstraints.NamespaceSelector, autogenerated[autogen.AutogenDefaults].Spec.MatchConstraints.NamespaceSelector)
	assert.Equal(t, 1, len(autogenerated[autogen.AutogenDefaults].Spec.ImageExtractors))
	assert.Equal(t, podctrlimg, autogenerated[autogen.AutogenDefaults].Spec.ImageExtractors)
	assert.Equal(t, cronRule, autogenerated[autogen.AutogenCronjobs].Spec.MatchConstraints.ResourceRules)
	assert.Equal(t, pol.Spec.MatchConstraints.NamespaceSelector, autogenerated[autogen.AutogenCronjobs].Spec.MatchConstraints.NamespaceSelector)

	polDisabled := ivpol.DeepCopy()
	polDisabled.Spec.AutogenConfiguration = &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
		PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
			Enabled: ptr.To(false),
		},
	}
	disabledAutogen, err := Autogen(polDisabled)
	assert.NoError(t, err)
	assert.Nil(t, disabledAutogen)

	polDisabledWithControllers := ivpol.DeepCopy()
	polDisabledWithControllers.Spec.AutogenConfiguration.PodControllers.Enabled = ptr.To(false)
	disabledWithControllers, err := Autogen(polDisabledWithControllers)
	assert.NoError(t, err)
	assert.Nil(t, disabledWithControllers)

	polEnabled := ivpol.DeepCopy()
	polEnabled.Spec.AutogenConfiguration = &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
		PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
			Enabled:     ptr.To(true),
			Controllers: []string{"deployments"},
		},
	}
	enabledAutogen, err := Autogen(polEnabled)
	assert.NoError(t, err)
	assert.NotEmpty(t, enabledAutogen)
	assert.Contains(t, enabledAutogen, autogen.AutogenDefaults)
	assert.NotContains(t, enabledAutogen, autogen.AutogenCronjobs)
}
