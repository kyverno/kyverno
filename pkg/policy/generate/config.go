package generate

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/policy/auth/fake"
)

// GenerateConfig holds all configuration required to build a Generate validator.
// Callers should always set Logger; all other fields are optional depending on mode.
type GenerateConfig struct {
	Rule *kyvernov1.Rule
	Client dclient.Interface
	Mock bool
	Logger logr.Logger
	BackgroundSA string
	AdmissionSA string
	ReportsSA string
}

// generateStep pairs a Generate validator with the verbs it should use when calling Validate.
type generateStep struct {
	g     *Generate
	verbs []string
}


type configuredGenerate struct {
	steps []generateStep
}

// NewGenerateValidator builds the correct Generate validator based on cfg.
//
// Offline (Mock=true): returns a single step backed by fake auth checkers that
// always permit, so no Kubernetes API access is needed. Structural errors in the
// generate rule are still reported.
//
// Online (Mock=false): returns one or two steps backed by real SubjectAccessReview
// calls:
//   - If the rule is synchronize, an admission-controller step with ["list","get"]
//     verbs runs first.
//   - A background-controller step (verbs determined by the rule) always runs.
func NewGenerateValidator(cfg GenerateConfig) *configuredGenerate {
	if cfg.Mock {
		g := &Generate{
			rule:               cfg.Rule,
			authChecker:        fake.NewFakeAuth(),
			authCheckerReports: fake.NewFakeAuth(),
			log:                cfg.Logger,
		}
		return &configuredGenerate{
			steps: []generateStep{{g: g, verbs: nil}},
		}
	}

	var steps []generateStep

	// For synchronize rules the admission controller also needs list/get access.
	if cfg.Rule.Generation.Synchronize && cfg.AdmissionSA != "" {
		steps = append(steps, generateStep{
			g:     NewGenerateFactory(cfg.Client, cfg.Rule, cfg.AdmissionSA, cfg.ReportsSA, cfg.Logger),
			verbs: []string{"list", "get"},
		})
	}

	steps = append(steps, generateStep{
		g:     NewGenerateFactory(cfg.Client, cfg.Rule, cfg.BackgroundSA, cfg.ReportsSA, cfg.Logger),
		verbs: nil,
	})

	return &configuredGenerate{steps: steps}
}


func (c *configuredGenerate) Validate(ctx context.Context, _ []string) (warnings []string, path string, err error) {
	for _, step := range c.steps {
		w, p, e := step.g.Validate(ctx, step.verbs)
		if e != nil {
			return nil, p, e
		}
		warnings = append(warnings, w...)
	}
	return warnings, "", nil
}
