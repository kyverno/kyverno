package toggle

import (
	"os"
	"strconv"
	"strings"
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
	// enable http calls in namespaced policies (CVE-2026-4789)
	AllowHTTPInNamespacedPoliciesFlagName    = "allowHTTPInNamespacedPolicies"
	AllowHTTPInNamespacedPoliciesDescription = "Set to 'true' to enable CEL http.Get/Post in namespaced policies. Disabled by default due to SSRF risk (CVE-2026-4789); enable only when combined with restrictive network policies."
	allowHTTPInNamespacedPoliciesEnvVar      = "FLAG_ENABLE_HTTP_IN_NAMESPACED_POLICIES"
	defaultAllowHTTPInNamespacedPolicies     = false
	// http blocklist / allowlist flag names
	HTTPBlocklistFlagName    = "httpBlocklist"
	HTTPBlocklistDescription = "Comma-separated list of CIDR ranges or hostnames blocked from CEL http.Get/Post calls. Overrides the built-in defaults when set. Example: '10.0.0.0/8,metadata.google.internal'."
	httpBlocklistEnvVar      = "FLAG_HTTP_BLOCKLIST"
	HTTPAllowlistFlagName    = "httpAllowlist"
	HTTPAllowlistDescription = "Comma-separated list of URL prefixes (scheme+host[+path]) permitted in CEL http.Get/Post calls. When set, only matching URLs are allowed. Example: 'https://api.example.com,https://webhook.corp/v1/'."
	httpAllowlistEnvVar      = "FLAG_HTTP_ALLOWLIST"
)

// defaultHTTPBlocklist mirrors DefaultBlockedCIDRs + DefaultBlockedHosts from
// github.com/kyverno/sdk/cel/libs/http. Duplicated here to avoid an import cycle.
var defaultHTTPBlocklist = []string{
	"127.0.0.0/8",
	"::1/128",
	"169.254.0.0/16",
	"fe80::/10",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
	"100.64.0.0/10",
	"metadata.google.internal",
	"metadata.internal",
}

var (
	ProtectManagedResources           = newToggle(defaultProtectManagedResources, protectManagedResourcesEnvVar)
	ForceFailurePolicyIgnore          = newToggle(defaultForceFailurePolicyIgnore, forceFailurePolicyIgnoreEnvVar)
	EnableDeferredLoading             = newToggle(defaultEnableDeferredLoading, enableDeferredLoadingEnvVar)
	GenerateValidatingAdmissionPolicy = newToggle(defaultGenerateValidatingAdmissionPolicy, generateValidatingAdmissionPolicyEnvVar)
	GenerateMutatingAdmissionPolicy   = newToggle(defaultGenerateMutatingAdmissionPolicy, generateMutatingAdmissionPolicyEnvVar)
	DumpMutatePatches                 = newToggle(defaultDumpMutatePatches, dumpMutatePatchesEnvVar)
	AutogenV2                         = newToggle(defaultAutogenV2, autogenV2EnvVar)
	AllowHTTPInNamespacedPolicies     = newToggle(defaultAllowHTTPInNamespacedPolicies, allowHTTPInNamespacedPoliciesEnvVar)
	HTTPBlocklist                     = newStringSliceFlag(defaultHTTPBlocklist, httpBlocklistEnvVar)
	HTTPAllowlist                     = newStringSliceFlag(nil, httpAllowlistEnvVar)
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

// StringSliceFlag is a flag that holds a comma-separated list of string values.
// It follows the same precedence as Toggle: CLI flag > environment variable > default.
type StringSliceFlag struct {
	value        []string
	hasValue     bool
	defaultValue []string
	envVar       string
}

func newStringSliceFlag(defaultValue []string, envVar string) *StringSliceFlag {
	return &StringSliceFlag{
		defaultValue: defaultValue,
		envVar:       envVar,
	}
}

// Parse is used as the callback for flagset.Func(...). The input is a comma-separated list of values.
func (f *StringSliceFlag) Parse(in string) error {
	f.value = splitAndTrim(in)
	f.hasValue = true
	return nil
}

// Reset clears any parsed flag value, returning the flag to its unset state so that
// env var and default precedence apply again. Intended for use in tests.
func (f *StringSliceFlag) Reset() {
	f.value = nil
	f.hasValue = false
}

// Values returns the current slice value following CLI > env var > default precedence.
func (f *StringSliceFlag) Values() []string {
	if f.hasValue {
		return f.value
	}
	if env, ok := os.LookupEnv(f.envVar); ok {
		return splitAndTrim(env)
	}
	return f.defaultValue
}

func splitAndTrim(in string) []string {
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
