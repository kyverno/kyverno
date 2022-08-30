package toggle

import (
	"os"
	"strconv"
)

const (
	AutogenInternalsFlagName     = "autogenInternals"
	AutogenInternalsDescription  = "Enables autogen internal policies. When this is 'true' policy rules should not be mutated."
	autogenInternalsEnvVar       = "FLAG_AUTOGEN_INTERNALS"
	defaultAutogenInternals      = true
	SplitPolicyReportFlagName    = "splitPolicyReport"
	SplitPolicyReportDescription = "Set the flag to 'true', to enable the split-up PolicyReports per policy."
	splitPolicyReportEnvVar      = "FLAG_SPLIT_POLICY_REPORT"
	defaultSplitPolicyReport     = false
)

var (
	AutogenInternals  = newToggle(defaultAutogenInternals, autogenInternalsEnvVar)
	SplitPolicyReport = newToggle(defaultSplitPolicyReport, splitPolicyReportEnvVar)
)

type Toggle interface {
	Enabled() bool
	Parse(string) error
}

type toggle struct {
	value        *bool
	defaultValue bool
	envVar       string
}

func newToggle(defaultValue bool, envVar string) *toggle {
	return &toggle{
		defaultValue: defaultValue,
		envVar:       envVar,
	}
}

func (t *toggle) Parse(in string) error {
	if value, err := getBool(in); err != nil {
		return err
	} else {
		t.value = value
		return nil
	}
}

func (t *toggle) Enabled() bool {
	if t.value != nil {
		return *t.value
	}
	if value, err := getBool(os.Getenv(t.envVar)); err == nil && value != nil {
		return *value
	}
	return t.defaultValue
}

func getBool(in string) (*bool, error) {
	if in == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(in)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
