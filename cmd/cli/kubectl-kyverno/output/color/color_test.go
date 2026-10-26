package color

import (
	"testing"

	fatihcolor "github.com/fatih/color"
)

func TestInit(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
	})

	tests := []struct {
		name              string
		noColor           bool
		fatihNoColor      bool
		wantHeaderBgColor int
		wantHeaderFgColor int
	}{
		{
			name:              "color disabled by flag",
			noColor:           true,
			fatihNoColor:      false,
			wantHeaderBgColor: 0,
			wantHeaderFgColor: 0,
		},
		{
			name:              "color enabled in TTY",
			noColor:           false,
			fatihNoColor:      false,
			wantHeaderBgColor: 40, // tablewriter.BgBlackColor
			wantHeaderFgColor: 32, // tablewriter.FgGreenColor
		},
		{
			name:              "color suppressed in non-TTY",
			noColor:           false,
			fatihNoColor:      true,
			wantHeaderBgColor: 0,
			wantHeaderFgColor: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fatihcolor.NoColor = tt.fatihNoColor
			Init(tt.noColor)
			if HeaderBgColor != tt.wantHeaderBgColor {
				t.Errorf("HeaderBgColor = %v, want %v", HeaderBgColor, tt.wantHeaderBgColor)
			}
			if HeaderFgColor != tt.wantHeaderFgColor {
				t.Errorf("HeaderFgColor = %v, want %v", HeaderFgColor, tt.wantHeaderFgColor)
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
		Init(true)
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
		Init(true)
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
		Init(true)
	})

	t.Run("color disabled", func(t *testing.T) {
		fatihcolor.NoColor = true
		Init(true)
		if got := BoldGreen("green"); got != "green" {
			t.Errorf("BoldGreen() = %q, want %q", got, "green")
		}
		if got := BoldRed("red"); got != "red" {
			t.Errorf("BoldRed() = %q, want %q", got, "red")
		}
		if got := BoldYellow("yellow"); got != "yellow" {
			t.Errorf("BoldYellow() = %q, want %q", got, "yellow")
		}
		if got := BoldCyan("cyan"); got != "cyan" {
			t.Errorf("BoldCyan() = %q, want %q", got, "cyan")
		}
	})

	t.Run("color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		Init(false)
		if got := BoldGreen("green"); got != "[32;1mgreen[0;22m" {
			t.Errorf("BoldGreen() = %q, want %q", got, "[32;1mgreen[0;22m")
		}
		if got := BoldRed("red"); got != "[31;1mred[0;22m" {
			t.Errorf("BoldRed() = %q, want %q", got, "[31;1mred[0;22m")
		}
		if got := BoldYellow("yellow"); got != "[33;1myellow[0;22m" {
			t.Errorf("BoldYellow() = %q, want %q", got, "[33;1myellow[0;22m")
		}
		if got := BoldCyan("cyan"); got != "[36;1mcyan[0;22m" {
			t.Errorf("BoldCyan() = %q, want %q", got, "[36;1mcyan[0;22m")
		}
	})
}
