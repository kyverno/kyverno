package internal

import (
	"flag"
	"io"
	"testing"
)

func TestRegistryFlags(t *testing.T) {
	// Flag registration mutates package globals, so these cases run serially.
	originalFlags := flag.CommandLine
	originalAllowlist, originalMode := privateRegistryAllowlist, privateRegistryEgressMode
	originalSecrets, originalHelpers := imagePullSecrets, registryCredentialHelpers
	originalInsecure := allowInsecureRegistry
	t.Cleanup(func() {
		flag.CommandLine = originalFlags
		privateRegistryAllowlist, privateRegistryEgressMode = originalAllowlist, originalMode
		imagePullSecrets, registryCredentialHelpers = originalSecrets, originalHelpers
		allowInsecureRegistry = originalInsecure
	})
	for _, tc := range []struct {
		name        string
		options     []ConfigurationOption
		credentials bool
	}{
		{name: "cleanup", options: []ConfigurationOption{WithRegistryEgress()}},
		{name: "full client", options: []ConfigurationOption{WithRegistryClient()}, credentials: true},
		{name: "both options", options: []ConfigurationOption{WithRegistryEgress(), WithRegistryClient()}, credentials: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tc.name, flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			config := NewConfiguration(tc.options...)
			initRegistryFlags(config)
			if config.UsesRegistryClient() != tc.credentials {
				t.Fatal("egress-only configuration must not enable Secret informers")
			}
			if err := flag.CommandLine.Parse([]string{"--privateRegistryEgressMode=enforce", "--privateRegistryAllowlist=10.20.0.0/16"}); err != nil {
				t.Fatal(err)
			}
			if privateRegistryEgressMode != "enforce" || privateRegistryAllowlist != "10.20.0.0/16" {
				t.Fatal("registry egress flags did not reach startup configuration")
			}
			for _, name := range []string{"imagePullSecrets", "registryCredentialHelpers", "allowInsecureRegistry"} {
				if registered := flag.CommandLine.Lookup(name) != nil; registered != tc.credentials {
					t.Fatalf("unexpected registration for %s: %v", name, registered)
				}
			}
		})
	}
}
