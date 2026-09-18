package generate

import (
	"context"
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/auth/fake"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	var err error
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "(podSelector)": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var genRule kyverno.Generation
	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}

	rawGenerate = []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker = NewFakeGenerate(genRule)
	if _, _, err := checker.Validate(context.TODO(), nil); err != nil {
		assert.Assert(t, err != nil)
	}
}


// Test_NewGenerateValidator_OfflineMode verifies that offline (mock) mode succeeds
// for a valid generate rule without any cluster access, and that it uses fake auth.
func Test_NewGenerateValidator_OfflineMode(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-offline",
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{
					Kind: "ConfigMap",
					Name: "test-cm",
				},
			},
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:         rule,
		Client:       nil,
		Mock:         true,
		Logger:       logging.GlobalLogger(),
		BackgroundSA: "system:serviceaccount:kyverno:kyverno-background-controller",
		AdmissionSA:  "system:serviceaccount:kyverno:kyverno",
		ReportsSA:    "",
	})

	assert.Equal(t, 1, len(v.steps))
	_, isFake := v.steps[0].g.authChecker.(*fake.FakeAuth)
	assert.Assert(t, isFake, "offline mode must use fake auth checker")

	warnings, _, err := v.Validate(context.TODO(), nil)
	assert.NilError(t, err)
	assert.Equal(t, 0, len(warnings))
}

// Test_NewGenerateValidator_OfflineStructuralError verifies that offline mode still
// surfaces structural errors (e.g. CELPreconditions on a generate rule).
func Test_NewGenerateValidator_OfflineStructuralError(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-offline-structural",
		CELPreconditions: []admissionregistrationv1.MatchCondition{
			{Name: "check", Expression: "true"},
		},
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Kind: "ConfigMap"},
			},
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:   rule,
		Mock:   true,
		Logger: logging.GlobalLogger(),
	})

	_, _, err := v.Validate(context.TODO(), nil)
	assert.ErrorContains(t, err, "celPrecondition can only be used with validate.cel")
}

// Test_NewGenerateValidator_OnlineSynchronize_StepCount verifies that an online
// synchronize rule produces two steps: admission-SA first, then background-SA.
func Test_NewGenerateValidator_OnlineSynchronize_StepCount(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-sync",
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Kind: "ConfigMap"},
			},
			Synchronize: true,
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:         rule,
		Client:       nil, // not called at construction time
		Mock:         false,
		Logger:       logging.GlobalLogger(),
		BackgroundSA: "system:serviceaccount:kyverno:kyverno-background-controller",
		AdmissionSA:  "system:serviceaccount:kyverno:kyverno",
		ReportsSA:    "",
	})

	assert.Equal(t, 2, len(v.steps))
	assert.DeepEqual(t, []string{"list", "get"}, v.steps[0].verbs)
	assert.Assert(t, v.steps[1].verbs == nil, "background step should have nil verbs")
}

// Test_NewGenerateValidator_OnlineNonSynchronize_StepCount verifies that a non-synchronize
// online rule produces exactly one background-SA step.
func Test_NewGenerateValidator_OnlineNonSynchronize_StepCount(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-nosync",
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Kind: "ConfigMap"},
			},
			Synchronize: false,
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:         rule,
		Client:       nil,
		Mock:         false,
		Logger:       logging.GlobalLogger(),
		BackgroundSA: "system:serviceaccount:kyverno:kyverno-background-controller",
		AdmissionSA:  "system:serviceaccount:kyverno:kyverno",
		ReportsSA:    "",
	})

	assert.Equal(t, 1, len(v.steps))
	assert.Assert(t, v.steps[0].verbs == nil)
}

// Test_NewGenerateValidator_ReportsAuthPresent verifies that when ReportsSA is set in
// online mode, the background step includes a non-nil reports auth checker.
func Test_NewGenerateValidator_ReportsAuthPresent(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-reports-present",
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Kind: "ConfigMap"},
			},
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:         rule,
		Client:       nil,
		Mock:         false,
		Logger:       logging.GlobalLogger(),
		BackgroundSA: "system:serviceaccount:kyverno:kyverno-background-controller",
		AdmissionSA:  "",
		ReportsSA:    "system:serviceaccount:kyverno:kyverno-reports-controller",
	})

	assert.Equal(t, 1, len(v.steps))
	assert.Assert(t, v.steps[0].g.authCheckerReports != nil,
		"reports auth checker must be non-nil when ReportsSA is provided")
}

// Test_NewGenerateValidator_ReportsAuthAbsent verifies that when ReportsSA is empty in
// online mode, the background step has a nil reports auth checker so no reports checks run.
func Test_NewGenerateValidator_ReportsAuthAbsent(t *testing.T) {
	rule := &kyverno.Rule{
		Name: "test-reports-absent",
		Generation: &kyverno.Generation{
			GeneratePattern: kyverno.GeneratePattern{
				ResourceSpec: kyverno.ResourceSpec{Kind: "ConfigMap"},
			},
		},
	}

	v := NewGenerateValidator(GenerateConfig{
		Rule:         rule,
		Client:       nil,
		Mock:         false,
		Logger:       logging.GlobalLogger(),
		BackgroundSA: "system:serviceaccount:kyverno:kyverno-background-controller",
		AdmissionSA:  "",
		ReportsSA:    "",
	})

	assert.Equal(t, 1, len(v.steps))
	assert.Assert(t, v.steps[0].g.authCheckerReports == nil,
		"reports auth checker must be nil when ReportsSA is empty")
}
