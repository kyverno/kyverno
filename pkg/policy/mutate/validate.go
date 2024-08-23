package mutate

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	mutation    kyvernov1.Mutation
	authChecker auth.AuthChecks
}

// NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(m kyvernov1.Mutation, client dclient.Interface, mock bool, backgroundSA string) *Mutate {
	var authCheck auth.AuthChecks
	if mock {
		authCheck = fake.NewFakeAuth()
	} else {
		authCheck = auth.NewAuth(client, backgroundSA, logging.GlobalLogger())
	}

	return &Mutate{
		mutation:    m,
		authChecker: authCheck,
	}
}

// Validate validates the 'mutate' rule
func (m *Mutate) Validate(ctx context.Context) (warnings []string, path string, err error) {
	if m.hasForEach() {
		if m.hasPatchStrategicMerge() || m.hasPatchesJSON6902() {
			return nil, "foreach", fmt.Errorf("only one of `foreach`, `patchStrategicMerge`, or `patchesJson6902` is allowed")
		}

		return m.validateForEach("", m.mutation.ForEachMutation)
	}

	if m.hasPatchesJSON6902() && m.hasPatchStrategicMerge() {
		return nil, "foreach", fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
	}

	if m.mutation.Targets != nil {
		if err := m.validateAuth(ctx, m.mutation.Targets); err != nil {
			return nil, "targets", fmt.Errorf("auth check fails, additional privileges are required for the service account '%s': %v", m.authChecker.User(), err)
		}
	}
	return nil, "", nil
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
	return len(m.mutation.ForEachMutation) > 0
}

func (m *Mutate) hasPatchStrategicMerge() bool {
	return m.mutation.GetPatchStrategicMerge() != nil
}

func (m *Mutate) hasPatchesJSON6902() bool {
	return m.mutation.PatchesJSON6902 != ""
}

func (m *Mutate) validateAuth(ctx context.Context, targets []kyvernov1.TargetResourceSpec) error {
	var errs []error
	for _, target := range targets {
		if regex.IsVariable(target.Kind) {
			continue
		}
		_, _, k, sub := kubeutils.ParseKindSelector(target.Kind)
		gvk := strings.Join([]string{target.APIVersion, k}, "/")
		verbs := []string{"get", "update"}
		ok, msg, err := m.authChecker.CanI(ctx, verbs, gvk, target.Namespace, target.Name, sub)
		if err != nil {
			return err
		}
		if !ok {
			errs = append(errs, fmt.Errorf(msg))
		}
	}

	return multierr.Combine(errs...)
}
