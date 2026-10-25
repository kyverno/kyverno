package auth_test

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/policy/mutate"
	"github.com/kyverno/kyverno/pkg/policy/validate"
	"github.com/stretchr/testify/assert"
)

// TestConstructorCompatibility checks the original function types as an external consumer.
func TestConstructorCompatibility(t *testing.T) {
	var newAuth func(dclient.Interface, string, logr.Logger) *auth.Auth = auth.NewAuth
	var newGenerate func(dclient.Interface, *kyvernov1.Rule, string, string, logr.Logger) *generate.Generate = generate.NewGenerateFactory
	var newMutate func(*kyvernov1.Rule, dclient.Interface, bool, string, string) *mutate.Mutate = mutate.NewMutateFactory
	var newValidate func(*kyvernov1.Rule, dclient.Interface, bool, string) *validate.Validate = validate.NewValidateFactory

	rule := &kyvernov1.Rule{}
	assert.Equal(t, "test-user", newAuth(nil, "test-user", logr.Discard()).User())
	assert.NotNil(t, newGenerate(nil, rule, "test-user", "", logr.Discard()))
	assert.NotNil(t, newMutate(rule, nil, true, "", ""))
	assert.NotNil(t, newValidate(rule, nil, true, ""))
}
