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
	// enable deferred context loading
	EnableDeferredLoadingFlagName    = "enableDeferredLoading"
	EnableDeferredLoadingDescription = "enable deferred loading of context variables"
	enableDeferredLoadingEnvVar      = "FLAG_ENABLE_DEFERRED_LOADING"
	defaultEnableDeferredLoading     = true
	// generate validating admission policies
	GenerateValidatingAdmissionPolicyFlagName    = "generateValidatingAdmissionPolicy"
	GenerateValidatingAdmissionPolicyDescription = "Set the flag to 'false', to disable the generation of ValidatingAdmissionPolicies."
	generateValidatingAdmissionPolicyEnvVar      = "FLAG_GENERATE_VALIDATING_ADMISSION_POLICY"
	defaultGenerateValidatingAdmissionPolicy     = true
	// generate mutating admission policies
	GenerateMutatingAdmissionPolicyFlagName    = "generateMutatingAdmissionPolicy"
	GenerateMutatingAdmissionPolicyDescription = "Set the flag to 'true', to generate mutating admission policies."
	generateMutatingAdmissionPolicyEnvVar      = "FLAG_GENERATE_MUTATING_ADMISSION_POLICY"
	defaultGenerateMutatingAdmissionPolicy     = false
	// dump mutate patches
	DumpMutatePatchesFlagName    = "dumpPatches"
	DumpMutatePatchesDescription = "Set the flag to 'true', to dump mutate patches."
	dumpMutatePatchesEnvVar      = "FLAG_DUMP_PATCHES"
	defaultDumpMutatePatches     = false
	// select autogen
	AutogenV2FlagName    = "autogenV2"
	AutogenV2Description = "Set the flag to 'true', to enable autogen v2."
	autogenV2EnvVar      = "FLAG_AUTOGEN_V2"
	defaultAutogenV2     = false
)

var (
	ProtectManagedResources           = newToggle(defaultProtectManagedResources, protectManagedResourcesEnvVar)
	ForceFailurePolicyIgnore          = newToggle(defaultForceFailurePolicyIgnore, forceFailurePolicyIgnoreEnvVar)
	EnableDeferredLoading             = newToggle(defaultEnableDeferredLoading, enableDeferredLoadingEnvVar)
	GenerateValidatingAdmissionPolicy = newToggle(defaultGenerateValidatingAdmissionPolicy, generateValidatingAdmissionPolicyEnvVar)
	GenerateMutatingAdmissionPolicy   = newToggle(defaultGenerateMutatingAdmissionPolicy, generateMutatingAdmissionPolicyEnvVar)
	DumpMutatePatches                 = newToggle(defaultDumpMutatePatches, dumpMutatePatchesEnvVar)
	AutogenV2                         = newToggle(defaultAutogenV2, autogenV2EnvVar)
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
