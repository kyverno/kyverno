package os

import (
	"fmt"
	"os"
)

func GetEnvWithFallback(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func MustGetEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	panic(fmt.Sprintf("environment variable `%s` is required.", name))
}
