package color

import (
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kataras/tablewriter"
)

func TestInit(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		Init(origNoColor)
	})

	tests := []struct {
		name    string
		noColor bool
	}{{
		name:    "color enabled",
		noColor: false,
	}, {
		name:    "color disabled",
		noColor: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.noColor)
			if tt.noColor {
				if HeaderBgColor != 0 || HeaderFgColor != 0 {
					t.Errorf("expected header colors to be cleared, got bg=%d fg=%d", HeaderBgColor, HeaderFgColor)
				}
			} else {
				if HeaderBgColor != tablewriter.BgBlackColor || HeaderFgColor != tablewriter.FgGreenColor {
					t.Errorf("expected header colors to be set, got bg=%d fg=%d", HeaderBgColor, HeaderFgColor)
				}
			}
		})
	}
}

func TestPolicy(t *testing.T) {
	Init(true)
	tests := []struct {
		name            string
		policyNamespace string
		policyName      string
		want            string
	}{{
		policyNamespace: "",
		policyName:      "policy",
		want:            "policy",
	}, {
		policyNamespace: "namespace",
		policyName:      "policy",
		want:            "namespace/policy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Policy(tt.policyNamespace, tt.policyName); got != tt.want {
				t.Errorf("Policy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRule(t *testing.T) {
	Init(true)
	tests := []struct {
		name     string
		ruleName string
		want     string
	}{{
		ruleName: "rule",
		want:     "rule",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Rule(tt.ruleName); got != tt.want {
				t.Errorf("Rule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource(t *testing.T) {
	Init(true)
	tests := []struct {
		name         string
		kind         string
		namespace    string
		resourceName string
		want         string
	}{{
		kind:         "Namespace",
		namespace:    "",
		resourceName: "resource",
		want:         "Namespace/resource",
	}, {
		kind:         "Pod",
		namespace:    "namespace",
		resourceName: "resource",
		want:         "namespace/Pod/resource",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resource(tt.kind, tt.namespace, tt.resourceName); got != tt.want {
				t.Errorf("Resource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Not found",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NotFound(); got != tt.want {
				t.Errorf("NotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultPass(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Pass",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultPass(); got != tt.want {
				t.Errorf("ResultPass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultFail(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Fail",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultFail(); got != tt.want {
				t.Errorf("ResultFail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultWarn(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Warn",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultWarn(); got != tt.want {
				t.Errorf("ResultWarn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultError(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Error",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultError(); got != tt.want {
				t.Errorf("ResultError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultSkip(t *testing.T) {
	Init(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Skip",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultSkip(); got != tt.want {
				t.Errorf("ResultSkip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionName(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		Init(origNoColor)
	})

	tests := []struct {
		name         string
		functionName string
		noColor      bool
		want         string
	}{{
		name:         "color disabled",
		functionName: "truncate",
		noColor:      true,
		want:         "truncate",
	}, {
		name:         "color enabled",
		functionName: "truncate",
		noColor:      false,
		want:         "\x1b[36;1mtruncate\x1b[0;22m",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fatihcolor.NoColor = tt.noColor
			Init(tt.noColor)
			if got := FunctionName(tt.functionName); got != tt.want {
				t.Errorf("FunctionName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFunctionNote(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		Init(origNoColor)
	})

	tests := []struct {
		name    string
		note    string
		noColor bool
		want    string
	}{{
		name:    "color disabled",
		note:    "sample note",
		noColor: true,
		want:    "sample note",
	}, {
		name:    "color enabled",
		note:    "sample note",
		noColor: false,
		want:    "\x1b[33;1msample note\x1b[0;22m",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fatihcolor.NoColor = tt.noColor
			Init(tt.noColor)
			if got := FunctionNote(tt.note); got != tt.want {
				t.Errorf("FunctionNote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBoldHelpers(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		Init(origNoColor)
	})

	t.Run("color disabled", func(t *testing.T) {
		fatihcolor.NoColor = true
		Init(true)

		assert := func(got, want string) {
			t.Helper()
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		}
		assert(BoldGreen(1), "1")
		assert(BoldRed(2), "2")
		assert(BoldYellow(3), "3")
		assert(BoldCyan(4), "4")
		assert(BoldYellow("Policies Skipped:"), "Policies Skipped:")
		assert(BoldRed("failed:"), "failed:")
	})

	t.Run("color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		Init(false)

		assert := func(got, want string) {
			t.Helper()
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		}
		assert(BoldGreen(1), "\x1b[32;1m1\x1b[0;22m")
		assert(BoldRed(2), "\x1b[31;1m2\x1b[0;22m")
		assert(BoldYellow(3), "\x1b[33;1m3\x1b[0;22m")
		assert(BoldCyan(4), "\x1b[36;1m4\x1b[0;22m")
		assert(BoldYellow("Policies Skipped:"), "\x1b[33;1mPolicies Skipped:\x1b[0;22m")
		assert(BoldRed("failed:"), "\x1b[31;1mfailed:\x1b[0;22m")
	})
}
