package toggle

import (
	"os"
	"strconv"
)

const (
	// protect managed resource
	ProtectManagedResourcesFlagName    = "protectManagedResources"
	ProtectManagedResourcesDescription = "Set the flag to 'true', to enable managed resources protection."
	protectManagedResourcesEnvVar      = "FLAG_PROTECT_MANAGED_RESOURCES"
	defaultProtectManagedResources     = false
	// force failure policy ignore
	ForceFailurePolicyIgnoreFlagName    = "forceFailurePolicyIgnore"
	ForceFailurePolicyIgnoreDescription = "Set the flag to 'true', to force set Failure Policy to 'ignore'."
	forceFailurePolicyIgnoreEnvVar      = "FLAG_FORCE_FAILURE_POLICY_IGNORE"
	defaultForceFailurePolicyIgnore     = false
	// disable deferred context loading
	DeferredLoadingFlagName    = "deferredLoading"
	DeferredLoadingDescription = "defer context variable loading until first use"
	deferredLoadingEnvVar      = "FLAG_DEFERRED_LOADING"
	defaultDeferredLoading     = true
)

var (
	ProtectManagedResources  = newToggle(defaultProtectManagedResources, protectManagedResourcesEnvVar)
	ForceFailurePolicyIgnore = newToggle(defaultForceFailurePolicyIgnore, forceFailurePolicyIgnoreEnvVar)
	DeferredLoading          = newToggle(defaultDeferredLoading, deferredLoadingEnvVar)
)

type ToggleFlag interface {
	Parse(string) error
}

type Toggle interface {
	ToggleFlag
	enabled() bool
}

type toggle struct {
	value        *bool
	defaultValue bool
	envVar       string
}

func newToggle(defaultValue bool, envVar string) Toggle {
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

func (t *toggle) enabled() bool {
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
