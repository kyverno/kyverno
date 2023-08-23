package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/policy/common"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

// Generate provides implementation to validate 'generate' rule
type Generate struct {
	user string
	// rule to hold 'generate' rule specifications
	rule kyvernov1.Generation
	// authCheck to check access for operations
	authCheck Operations
	// logger
	log logr.Logger
}

// NewGenerateFactory returns a new instance of Generate validation checker
func NewGenerateFactory(client dclient.Interface, rule kyvernov1.Generation, user string, log logr.Logger) *Generate {
	g := Generate{
		user:      user,
		rule:      rule,
		authCheck: NewAuth(client, user, log),
		log:       log,
	}

	return &g
}

// Validate validates the 'generate' rule
func (g *Generate) Validate(ctx context.Context) (string, error) {
	rule := g.rule
	if rule.GetData() != nil && rule.Clone != (kyvernov1.CloneFrom{}) {
		return "", fmt.Errorf("only one of data or clone can be specified")
	}

	if rule.Clone != (kyvernov1.CloneFrom{}) && len(rule.CloneList.Kinds) != 0 {
		return "", fmt.Errorf("only one of clone or cloneList can be specified")
	}

	apiVersion, kind, name, namespace := rule.ResourceSpec.GetAPIVersion(), rule.ResourceSpec.GetKind(), rule.ResourceSpec.GetName(), rule.ResourceSpec.GetNamespace()

	if len(rule.CloneList.Kinds) == 0 {
		if name == "" {
			return "name", fmt.Errorf("name cannot be empty")
		}
		if kind == "" {
			return "kind", fmt.Errorf("kind cannot be empty")
		}
		if apiVersion == "" {
			return "apiVersion", fmt.Errorf("apiVersion cannot be empty")
		}
	} else {
		if name != "" {
			return "name", fmt.Errorf("with cloneList, generate.name. should not be specified")
		}
		if kind != "" {
			return "kind", fmt.Errorf("with cloneList, generate.kind. should not be specified")
		}
	}

	if rule.CloneList.Selector != nil {
		if wildcard.ContainsWildcard(rule.CloneList.Selector.String()) {
			return "selector", fmt.Errorf("wildcard characters `*/?` not supported")
		}
	}

	if target := rule.GetData(); target != nil {
		// TODO: is this required ?? as anchors can only be on pattern and not resource
		// we can add this check by not sure if its needed here
		if path, err := common.ValidatePattern(target, "/", nil); err != nil {
			return fmt.Sprintf("data.%s", path), fmt.Errorf("anchors not supported on generate resources: %v", err)
		}
	}

	// Kyverno generate-controller create/update/deletes the resources specified in generate rule of policy
	// kyverno uses SA 'kyverno' and has default ClusterRoles and ClusterRoleBindings
	// instructions to modify the RBAC for kyverno are mentioned at https://github.com/kyverno/kyverno/blob/master/documentation/installation.md
	// - operations required: create/update/delete/get
	// If kind and namespace contain variables, then we cannot resolve then so we skip the processing
	if len(rule.CloneList.Kinds) != 0 {
		for _, kind = range rule.CloneList.Kinds {
			gvk, sub := parseCloneKind(kind)
			if err := g.canIGenerate(ctx, gvk, namespace, sub); err != nil {
				return "", err
			}
		}
	} else {
		k, sub := kubeutils.SplitSubresource(kind)
		if err := g.canIGenerate(ctx, strings.Join([]string{apiVersion, k}, "/"), namespace, sub); err != nil {
			return "", err
		}
	}
	return "", nil
}

// canIGenerate returns a error if kyverno cannot perform operations
func (g *Generate) canIGenerate(ctx context.Context, gvk, namespace, subresource string) error {
	// Skip if there is variable defined
	authCheck := g.authCheck
	if !regex.IsVariable(gvk) {
		ok, err := authCheck.CanICreate(ctx, gvk, namespace, subresource)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s does not have permissions to 'create' resource %s/%s/%s. Grant proper permissions to the background controller", g.user, gvk, subresource, namespace)
		}

		ok, err = authCheck.CanIUpdate(ctx, gvk, namespace, subresource)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s does not have permissions to 'update' resource %s/%s/%s. Grant proper permissions to the background controller", g.user, gvk, subresource, namespace)
		}

		ok, err = authCheck.CanIGet(ctx, gvk, namespace, subresource)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s does not have permissions to 'get' resource %s/%s/%s. Grant proper permissions to the background controller", g.user, gvk, subresource, namespace)
		}

		ok, err = authCheck.CanIDelete(ctx, gvk, namespace, subresource)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s does not have permissions to 'delete' resource %s/%s/%s. Grant proper permissions to the background controller", g.user, gvk, subresource, namespace)
		}
	} else {
		g.log.V(2).Info("resource Kind uses variables, so cannot be resolved. Skipping Auth Checks.")
	}

	return nil
}

func parseCloneKind(gvks string) (gvk, sub string) {
	gv, ks := kubeutils.GetKindFromGVK(gvks)
	k, sub := kubeutils.SplitSubresource(ks)
	if !strings.Contains(gv, "*") {
		k = strings.Join([]string{gv, k}, "/")
	}
	return k, sub
}
