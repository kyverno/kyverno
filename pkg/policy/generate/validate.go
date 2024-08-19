package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/kyverno/kyverno/pkg/policy/common"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

// Generate provides implementation to validate 'generate' rule
type Generate struct {
	user        string
	rule        kyvernov1.Generation
	authChecker auth.AuthChecks
	log         logr.Logger
}

// NewGenerateFactory returns a new instance of Generate validation checker
func NewGenerateFactory(client dclient.Interface, rule kyvernov1.Generation, user string, log logr.Logger) *Generate {
	g := Generate{
		user:        user,
		rule:        rule,
		authChecker: auth.NewAuth(client, user, log),
		log:         log,
	}

	return &g
}

// Validate validates the 'generate' rule
func (g *Generate) Validate(ctx context.Context) (warnings []string, path string, err error) {
	rule := g.rule
	if rule.CloneList.Selector != nil {
		if wildcard.ContainsWildcard(rule.CloneList.Selector.String()) {
			return nil, "selector", fmt.Errorf("wildcard characters `*/?` not supported")
		}
	}

	if target := rule.GetData(); target != nil {
		// TODO: is this required ?? as anchors can only be on pattern and not resource
		// we can add this check by not sure if its needed here
		if path, err := common.ValidatePattern(target, "/", nil); err != nil {
			return nil, fmt.Sprintf("data.%s", path), fmt.Errorf("anchors not supported on generate resources: %v", err)
		}
	}

	// Kyverno generate-controller create/update/deletes the resources specified in generate rule of policy
	// kyverno uses SA 'kyverno' and has default ClusterRoles and ClusterRoleBindings
	// instructions to modify the RBAC for kyverno are mentioned at https://github.com/kyverno/kyverno/blob/master/documentation/installation.md
	// - operations required: create/update/delete/get
	// If kind and namespace contain variables, then we cannot resolve then so we skip the processing
	if rule.ForEachGeneration != nil {
		for _, forEach := range rule.ForEachGeneration {
			if err := g.validateAuth(ctx, forEach.GeneratePatterns); err != nil {
				return nil, "foreach", err
			}
		}
	} else {
		if err := g.validateAuth(ctx, rule.GeneratePatterns); err != nil {
			return nil, "", err
		}
	}
	return nil, "", nil
}

// validateAuth returns a error if kyverno cannot perform operations
func (g *Generate) validateAuth(ctx context.Context, generate kyvernov1.GeneratePatterns) error {
	if len(generate.CloneList.Kinds) != 0 {
		for _, kind := range generate.CloneList.Kinds {
			gvk, sub := parseCloneKind(kind)
			return g.canIGenerate(ctx, gvk, generate.Namespace, sub)
		}
	} else {
		k, sub := kubeutils.SplitSubresource(generate.Kind)
		return g.canIGenerate(ctx, strings.Join([]string{generate.APIVersion, k}, "/"), generate.Namespace, sub)
	}
	return nil
}

func (g *Generate) canIGenerate(ctx context.Context, gvk, namespace, subresource string) error {
	if regex.IsVariable(gvk) {
		g.log.V(2).Info("resource Kind uses variables; skipping authorization checks.")
		return nil
	}

	verbs := []string{"get", "create"}
	if g.rule.Synchronize {
		verbs = []string{"get", "create", "update", "delete"}
	}

	ok, msg, err := g.authChecker.CanI(ctx, verbs, gvk, namespace, "", subresource)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf(msg)
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
