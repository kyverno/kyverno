package report

import (
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

func TestNewReportingConfig(t *testing.T) {
	tests := []struct {
		name              string
		allowedRuleStatus []string
		items             []string
		wantValidate      bool
		wantMutate        bool
		wantMutateExist   bool
		wantImageVerify   bool
		wantGenerate      bool
	}{{
		name:              "all report types enabled",
		allowedRuleStatus: []string{"pass", "fail"},
		items:             []string{"validate", "mutate", "mutateExisting", "imageVerify", "generate"},
		wantValidate:      true,
		wantMutate:        true,
		wantMutateExist:   true,
		wantImageVerify:   true,
		wantGenerate:      true,
	}, {
		name:              "only validate enabled",
		allowedRuleStatus: []string{"pass"},
		items:             []string{"validate"},
		wantValidate:      true,
		wantMutate:        false,
		wantMutateExist:   false,
		wantImageVerify:   false,
		wantGenerate:      false,
	}, {
		name:              "no report types enabled",
		allowedRuleStatus: []string{},
		items:             []string{},
		wantValidate:      false,
		wantMutate:        false,
		wantMutateExist:   false,
		wantImageVerify:   false,
		wantGenerate:      false,
	}, {
		name:              "mutate and generate enabled",
		allowedRuleStatus: []string{"fail", "error"},
		items:             []string{"mutate", "generate"},
		wantValidate:      false,
		wantMutate:        true,
		wantMutateExist:   false,
		wantImageVerify:   false,
		wantGenerate:      true,
	}, {
		name:              "only imageVerify enabled",
		allowedRuleStatus: []string{"warn"},
		items:             []string{"imageVerify"},
		wantValidate:      false,
		wantMutate:        false,
		wantMutateExist:   false,
		wantImageVerify:   true,
		wantGenerate:      false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global state for each test case
			ReportingCfg = nil
			cfg := NewReportingConfig(tt.allowedRuleStatus, tt.items...)
			assert.NotNil(t, cfg)
			assert.Equal(t, tt.wantValidate, cfg.ValidateReportsEnabled())
			assert.Equal(t, tt.wantMutate, cfg.MutateReportsEnabled())
			assert.Equal(t, tt.wantMutateExist, cfg.MutateExistingReportsEnabled())
			assert.Equal(t, tt.wantImageVerify, cfg.ImageVerificationReportsEnabled())
			assert.Equal(t, tt.wantGenerate, cfg.GenerateReportsEnabled())
		})
	}
	// Reset global after all tests
	ReportingCfg = nil
}

func TestIsStatusAllowed(t *testing.T) {
	tests := []struct {
		name              string
		allowedRuleStatus []string
		checkStatus       engineapi.RuleStatus
		want              bool
	}{{
		name:              "pass is allowed",
		allowedRuleStatus: []string{"pass", "fail"},
		checkStatus:       engineapi.RuleStatusPass,
		want:              true,
	}, {
		name:              "fail is allowed",
		allowedRuleStatus: []string{"pass", "fail"},
		checkStatus:       engineapi.RuleStatusFail,
		want:              true,
	}, {
		name:              "warn is not allowed",
		allowedRuleStatus: []string{"pass", "fail"},
		checkStatus:       engineapi.RuleStatusWarn,
		want:              false,
	}, {
		name:              "error is allowed when configured",
		allowedRuleStatus: []string{"error"},
		checkStatus:       engineapi.RuleStatusError,
		want:              true,
	}, {
		name:              "skip is allowed when configured",
		allowedRuleStatus: []string{"skip"},
		checkStatus:       engineapi.RuleStatusSkip,
		want:              true,
	}, {
		name:              "no statuses allowed",
		allowedRuleStatus: []string{},
		checkStatus:       engineapi.RuleStatusPass,
		want:              false,
	}, {
		name:              "all statuses allowed - check pass",
		allowedRuleStatus: []string{"pass", "fail", "warn", "error", "skip"},
		checkStatus:       engineapi.RuleStatusPass,
		want:              true,
	}, {
		name:              "unknown status string is ignored",
		allowedRuleStatus: []string{"unknown", "pass"},
		checkStatus:       engineapi.RuleStatusPass,
		want:              true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ReportingCfg = nil
			cfg := NewReportingConfig(tt.allowedRuleStatus, "validate")
			got := cfg.IsStatusAllowed(tt.checkStatus)
			assert.Equal(t, tt.want, got)
		})
	}
	ReportingCfg = nil
}

func TestNewReportingConfig_ReturnsCachedInstance(t *testing.T) {
	ReportingCfg = nil
	cfg1 := NewReportingConfig([]string{"pass"}, "validate")
	cfg2 := NewReportingConfig([]string{"fail"}, "mutate")
	// Second call should return cached instance (cfg1)
	assert.Equal(t, cfg1, cfg2)
	assert.True(t, cfg2.ValidateReportsEnabled())
	assert.False(t, cfg2.MutateReportsEnabled())
	ReportingCfg = nil
}
