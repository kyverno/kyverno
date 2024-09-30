package mutate

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/kyverno/kyverno/pkg/policy/auth/fake"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	rule                  *kyvernov1.Rule
	authCheckerBackground auth.AuthChecks
	authCheckerReports    auth.AuthChecks
}

// NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(rule *kyvernov1.Rule, client dclient.Interface, mock bool, backgroundSA, reportsSA string) *Mutate {
	var authCheckBackground, authCheckerReports auth.AuthChecks
	if mock {
		authCheckBackground = fake.NewFakeAuth()
		authCheckerReports = fake.NewFakeAuth()
	} else {
		authCheckBackground = auth.NewAuth(client, backgroundSA, logging.GlobalLogger())
		authCheckerReports = auth.NewAuth(client, reportsSA, logging.GlobalLogger())
	}

	return &Mutate{
		rule:                  rule,
		authCheckerBackground: authCheckBackground,
		authCheckerReports:    authCheckerReports,
	}
}

// Validate validates the 'mutate' rule
func (m *Mutate) Validate(ctx context.Context, _ []string) (warnings []string, path string, err error) {
	if m.hasForEach() {
		if m.hasPatchStrategicMerge() || m.hasPatchesJSON6902() {
			return nil, "foreach", fmt.Errorf("only one of `foreach`, `patchStrategicMerge`, or `patchesJson6902` is allowed")
		}

		return m.validateForEach("", m.rule.Mutation.ForEachMutation)
	}

	if m.hasPatchesJSON6902() && m.hasPatchStrategicMerge() {
		return nil, "foreach", fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
	}

	if m.rule.Mutation.Targets != nil {
		if err := m.validateAuth(ctx, m.rule.Mutation.Targets); err != nil {
			return nil, "targets", fmt.Errorf("auth check fails, additional privileges are required for the service account '%s': %v", m.authCheckerBackground.User(), err)
		}
	}
	if w, err := m.validateAuthReports(ctx); err != nil {
		return nil, "", err
	} else if len(w) > 0 {
		warnings = append(warnings, w...)
	}
	return warnings, "", nil
}

func (m *Mutate) validateForEach(tag string, foreach []kyvernov1.ForEachMutation) (warnings []string, path string, err error) {
	for i, fe := range foreach {
		tag = tag + fmt.Sprintf("foreach[%d]", i)
		fem := fe.GetForEachMutation()
		if len(fem) > 0 {
			if fe.Context != nil || fe.AnyAllConditions != nil || fe.PatchesJSON6902 != "" || fe.RawPatchStrategicMerge != nil {
				return nil, tag, fmt.Errorf("a nested foreach cannot contain other declarations")
			}

			return m.validateNestedForEach(tag, fem)
		}

		psm := fe.GetPatchStrategicMerge()
		if (fe.PatchesJSON6902 == "" && psm == nil) || (fe.PatchesJSON6902 != "" && psm != nil) {
			return nil, tag, fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
		}
	}

	return nil, "", nil
}

func (m *Mutate) validateNestedForEach(tag string, j []kyvernov1.ForEachMutation) (warnings []string, path string, err error) {
	if j != nil {
		return m.validateForEach(tag, j)
	}

	return nil, "", nil
}

func (m *Mutate) hasForEach() bool {
	return len(m.rule.Mutation.ForEachMutation) > 0
}

func (m *Mutate) hasPatchStrategicMerge() bool {
	return m.rule.Mutation.GetPatchStrategicMerge() != nil
}

func (m *Mutate) hasPatchesJSON6902() bool {
	return m.rule.Mutation.PatchesJSON6902 != ""
}

func (m *Mutate) validateAuth(ctx context.Context, targets []kyvernov1.TargetResourceSpec) (err error) {
	var errs []error
	for _, target := range targets {
		if regex.IsVariable(target.Kind) {
			continue
		}
		_, _, k, sub := kubeutils.ParseKindSelector(target.Kind)
		gvk := strings.Join([]string{target.APIVersion, k}, "/")
		verbs := []string{"get", "update"}
		ok, msg, err := m.authCheckerBackground.CanI(ctx, verbs, gvk, target.Namespace, target.Name, sub)
		if err != nil {
			return err
		}
		if !ok {
			errs = append(errs, fmt.Errorf(msg)) //nolint:all
		}
	}

	return multierr.Combine(errs...)
}

func (m *Mutate) validateAuthReports(ctx context.Context) (warnings []string, err error) {
	kinds := m.rule.MatchResources.GetKinds()
	for _, k := range kinds {
		if wildcard.ContainsWildcard(k) {
			return nil, nil
		}

		verbs := []string{"get", "list", "watch"}
		ok, msg, err := m.authCheckerReports.CanI(ctx, verbs, k, "", "", "")
		if err != nil {
			return nil, err
		}
		if !ok {
			warnings = append(warnings, msg)
		}
	}

	return warnings, nil
}
