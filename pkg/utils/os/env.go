package os

import "os"

func GetEnvWithFallback(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
