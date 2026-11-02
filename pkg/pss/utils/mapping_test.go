package utils

import (
	"reflect"
	"strings"
	"testing"
)

func TestPSSControlIDToName_ProcMountRestricted(t *testing.T) {
	if got := PSSControlIDToName("procMount_restricted"); got != "/proc Mount Type" {
		t.Fatalf("expected /proc Mount Type, got %q", got)
	}
	if len(PSS_controls["procMount_restricted"]) == 0 {
		t.Fatal("expected restricted fields for procMount_restricted")
	}
}

// Every restricted field defined for spec.containers must also be defined, with
// the same allowed values, for spec.initContainers and spec.ephemeralContainers,
// since the Pod Security Standards apply the same controls to all three.
// A copy-paste slip in one of the parallel entries otherwise goes unnoticed.
func TestPSSControls_ContainerKindsMatch(t *testing.T) {
	for control, fields := range PSS_controls {
		allowed := make(map[string][]interface{}, len(fields))
		for _, f := range fields {
			allowed[f.Path] = f.AllowedValues
		}
		for _, f := range fields {
			suffix, ok := strings.CutPrefix(f.Path, "spec.containers[*].")
			if !ok {
				continue
			}
			for _, kind := range []string{"spec.initContainers[*].", "spec.ephemeralContainers[*]."} {
				path := kind + suffix
				got, ok := allowed[path]
				if !ok {
					t.Errorf("%s: %s is restricted but %s is not", control, f.Path, path)
					continue
				}
				if !reflect.DeepEqual(got, f.AllowedValues) {
					t.Errorf("%s: %s allows %v, want %v as for %s", control, path, got, f.AllowedValues, f.Path)
				}
			}
		}
	}
}

func TestPSSControls_SELinuxOptionsEphemeralContainerPaths(t *testing.T) {
	paths := make(map[string]bool)
	for _, f := range PSS_controls["seLinuxOptions"] {
		paths[f.Path] = true
	}
	for _, path := range []string{
		"spec.ephemeralContainers[*].securityContext.seLinuxOptions.user",
		"spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
	} {
		if !paths[path] {
			t.Errorf("seLinuxOptions: missing restricted field %s", path)
		}
	}
}

func TestPSSControls_RunAsNonRootAllowedValues(t *testing.T) {
	want := []interface{}{true, nil}
	for _, path := range []string{
		"spec.containers[*].securityContext.runAsNonRoot",
		"spec.initContainers[*].securityContext.runAsNonRoot",
		"spec.ephemeralContainers[*].securityContext.runAsNonRoot",
	} {
		var found bool
		for _, f := range PSS_controls["runAsNonRoot"] {
			if f.Path != path {
				continue
			}
			found = true
			if !reflect.DeepEqual(f.AllowedValues, want) {
				t.Errorf("runAsNonRoot: %s allows %v, want %v", path, f.AllowedValues, want)
			}
		}
		if !found {
			t.Errorf("runAsNonRoot: missing restricted field %s", path)
		}
	}
}
