package color

import (
	"strings"

	ec "github.com/kyverno/kyverno/ext/output/color"
)

func Policy(namespace, name string) string {
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			namespace = parts[0]
			name = parts[1]
		}
	}
	if namespace == "" {
		return ec.BoldFgCyan.Sprint(name)
	}
	return ec.BoldFgCyan.Sprint(namespace) + "/" + ec.BoldFgCyan.Sprint(name)
}

func Rule(name string) string {
	return ec.BoldFgCyan.Sprint(name)
}

func Resource(kind, namespace, name string) string {
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			namespace = parts[0]
			name = parts[1]
		}
	}
	if namespace == "" {
		return ec.BoldFgCyan.Sprint(kind) + "/" + ec.BoldFgCyan.Sprint(name)
	}
	return ec.BoldFgCyan.Sprint(namespace) + "/" + ec.BoldFgCyan.Sprint(kind) + "/" + ec.BoldFgCyan.Sprint(name)
}

func Excluded() string {
	return ec.BoldYellow.Sprint("Excluded")
}

func NotFound() string {
	return ec.BoldYellow.Sprint("Not found")
}

func ResultPass() string {
	return ec.BoldGreen.Sprint("Pass")
}

func ResultFail() string {
	return ec.BoldRed.Sprint("Fail")
}

func ResultWarn() string {
	return ec.BoldYellow.Sprint("Warn")
}

func ResultError() string {
	return ec.BoldRed.Sprint("Error")
}

func ResultSkip() string {
	return ec.BoldFgCyan.Sprint("Skip")
}

func InvalidPolicy() string {
	return ec.BoldYellow.Sprint("Invalid Policy")
}
