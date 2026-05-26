package color

import (
	"strings"
	"testing"
)

const testText = "test"

func TestInit(t *testing.T) {
	tests := []struct {
		name        string
		noColor     bool
		expectColor bool
	}{
		{
			name:        "noColor -> no color",
			noColor:     true,
			expectColor: false,
		},
		{
			name:        "color -> color",
			noColor:     false,
			expectColor: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			forceAndReset(t, tc.expectColor)

			coloredText := BoldGreen.Sprint(testText)

			if tc.expectColor {
				if coloredText == testText {
					t.Errorf("Expected color output")
				}
				if !strings.Contains(coloredText, "\x1b[") {
					t.Errorf("Expected ANSI color codes in output")
				}
				if !strings.HasSuffix(coloredText, "\x1b[0m") {
					t.Errorf("Expected output to end with ANSI reset code")
				}
				if !strings.Contains(coloredText, testText) {
					t.Errorf("Expected output to contain the original text")
				}
			} else {
				if coloredText != testText {
					t.Errorf("Expected no color output")
				}
			}
		})
	}
}

func forceAndReset(t *testing.T, enable bool) {
	t.Helper()
	originalColorState := Enabled()
	t.Cleanup(func() {
		Force(originalColorState)
	})
	Force(enable)
}
