package mutate

import (
	"context"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/utils/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	mutation    kyvernov1.Mutation
	user        string
	authChecker AuthChecker
}

// NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(m kyvernov1.Mutation, client dclient.Interface, user string) *Mutate {
	return &Mutate{
		mutation:    m,
		user:        user,
		authChecker: newAuthChecker(client, user),
	}
}

// Validate validates the 'mutate' rule
func (m *Mutate) Validate(ctx context.Context) (string, error) {
	if m.hasForEach() {
		if m.hasPatchStrategicMerge() || m.hasPatchesJSON6902() {
			return "foreach", fmt.Errorf("only one of `foreach`, `patchStrategicMerge`, or `patchesJson6902` is allowed")
		}

		return m.validateForEach("", m.mutation.ForEachMutation)
	}

	if m.hasPatchesJSON6902() && m.hasPatchStrategicMerge() {
		return "foreach", fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
	}

	if m.mutation.Targets != nil {
		if err := m.validateAuth(ctx, m.mutation.Targets); err != nil {
			return "targets", fmt.Errorf("auth check fails, additional privileges are required for the service account '%s': %v", m.user, err)
		}
	}
	return "", nil
}

func (m *Mutate) validateForEach(tag string, foreach []kyvernov1.ForEachMutation) (string, error) {
	for i, fe := range foreach {
		tag = tag + fmt.Sprintf("foreach[%d]", i)
		if fe.ForEachMutation != nil {
			if fe.Context != nil || fe.AnyAllConditions != nil || fe.PatchesJSON6902 != "" || fe.RawPatchStrategicMerge != nil {
				return tag, fmt.Errorf("a nested foreach cannot contain other declarations")
			}

			return m.validateNestedForEach(tag, fe.ForEachMutation)
		}

		psm := fe.GetPatchStrategicMerge()
		if (fe.PatchesJSON6902 == "" && psm == nil) || (fe.PatchesJSON6902 != "" && psm != nil) {
			return tag, fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
		}
	}

	return "", nil
}

func (m *Mutate) validateNestedForEach(tag string, j *v1.JSON) (string, error) {
	nestedForeach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](j)
	if err != nil {
		return tag, fmt.Errorf("invalid foreach syntax: %w", err)
	}

	return m.validateForEach(tag, nestedForeach)
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
		if !regex.IsVariable(target.Kind) {
			_, _, k, sub := kubeutils.ParseKindSelector(target.Kind)
			srcKey := k
			if sub != "" {
				srcKey = srcKey + "/" + sub
			}

			if ok, err := m.authChecker.CanIUpdate(ctx, strings.Join([]string{target.APIVersion, k}, "/"), target.Namespace, sub); err != nil {
				errs = append(errs, err)
			} else if !ok {
				errs = append(errs, fmt.Errorf("cannot %s/%s/%s in namespace %s", "update", target.APIVersion, srcKey, target.Namespace))
			}

			if ok, err := m.authChecker.CanIGet(ctx, strings.Join([]string{target.APIVersion, k}, "/"), target.Namespace, sub); err != nil {
				errs = append(errs, err)
			} else if !ok {
				errs = append(errs, fmt.Errorf("cannot %s/%s/%s in namespace %s", "get", target.APIVersion, srcKey, target.Namespace))
			}
		}
	}
	return multierr.Combine(errs...)
}
