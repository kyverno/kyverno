package generate

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/policy/common"
)

// Generate provides implementation to validate 'generate' rule
type Generate struct {
	// rule to hold 'generate' rule specifications
	rule kyvernov1.Generation
	// authCheck to check access for operations
	authCheck Operations
	// logger
	log logr.Logger
}

// NewGenerateFactory returns a new instance of Generate validation checker
func NewGenerateFactory(client dclient.Interface, rule kyvernov1.Generation, log logr.Logger) *Generate {
	g := Generate{
		rule:      rule,
		authCheck: NewAuth(client, log),
		log:       log,
	}

	return &g
}

// Validate validates the 'generate' rule
func (g *Generate) Validate() (string, error) {
	rule := g.rule
	if rule.GetData() != nil && rule.Clone != (kyvernov1.CloneFrom{}) {
		return "", fmt.Errorf("only one of data or clone can be specified")
	}

	if rule.Clone != (kyvernov1.CloneFrom{}) && len(rule.CloneList.Kinds) != 0 {
		return "", fmt.Errorf("only one of clone or cloneList can be specified")
	}

	kind, name, namespace := rule.Kind, rule.Name, rule.Namespace

	if len(rule.CloneList.Kinds) == 0 {
		if name == "" {
			return "name", fmt.Errorf("name cannot be empty")
		}
		if kind == "" {
			return "kind", fmt.Errorf("kind cannot be empty")
		}
	}
	// Can I generate resource

	if !reflect.DeepEqual(rule.Clone, kyvernov1.CloneFrom{}) {
		if path, err := g.validateClone(rule.Clone, rule.CloneList, kind); err != nil {
			return fmt.Sprintf("clone.%s", path), err
		}
	}
	if target := rule.GetData(); target != nil {
		// TODO: is this required ?? as anchors can only be on pattern and not resource
		// we can add this check by not sure if its needed here
		if path, err := common.ValidatePattern(target, "/", []commonAnchors.IsAnchor{}); err != nil {
			return fmt.Sprintf("data.%s", path), fmt.Errorf("anchors not supported on generate resources: %v", err)
		}
	}

	// Kyverno generate-controller create/update/deletes the resources specified in generate rule of policy
	// kyverno uses SA 'kyverno-service-account' and has default ClusterRoles and ClusterRoleBindings
	// instructions to modify the RBAC for kyverno are mentioned at https://github.com/kyverno/kyverno/blob/master/documentation/installation.md
	// - operations required: create/update/delete/get
	// If kind and namespace contain variables, then we cannot resolve then so we skip the processing
	if err := g.canIGenerate(kind, namespace); err != nil {
		return "", err
	}
	return "", nil
}

func (g *Generate) validateClone(c kyvernov1.CloneFrom, cl kyvernov1.CloneList, kind string) (string, error) {
	if len(cl.Kinds) == 0 {
		if c.Name == "" {
			return "name", fmt.Errorf("name cannot be empty")
		}
	}

	namespace := c.Namespace
	// Skip if there is variable defined
	if !variables.IsVariable(kind) && !variables.IsVariable(namespace) {
		// GET
		ok, err := g.authCheck.CanIGet(kind, namespace)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("kyverno does not have permissions to 'get' resource %s/%s. Update permissions in ClusterRole 'kyverno:generate'", kind, namespace)
		}
	} else {
		g.log.V(4).Info("name & namespace uses variables, so cannot be resolved. Skipping Auth Checks.")
	}
	return "", nil
}

// canIGenerate returns a error if kyverno cannot perform operations
func (g *Generate) canIGenerate(kind, namespace string) error {
	// Skip if there is variable defined
	authCheck := g.authCheck
	if !variables.IsVariable(kind) && !variables.IsVariable(namespace) {
		// CREATE
		ok, err := authCheck.CanICreate(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'create' resource %s/%s. Update permissions in ClusterRole 'kyverno:generate'", kind, namespace)
		}
		// UPDATE
		ok, err = authCheck.CanIUpdate(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'update' resource %s/%s. Update permissions in ClusterRole 'kyverno:generate'", kind, namespace)
		}
		// GET
		ok, err = authCheck.CanIGet(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'get' resource %s/%s. Update permissions in ClusterRole 'kyverno:generate'", kind, namespace)
		}

		// DELETE
		ok, err = authCheck.CanIDelete(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'delete' resource %s/%s. Update permissions in ClusterRole 'kyverno:generate'", kind, namespace)
		}
	} else {
		g.log.V(4).Info("name & namespace uses variables, so cannot be resolved. Skipping Auth Checks.")
	}

	return nil
}
