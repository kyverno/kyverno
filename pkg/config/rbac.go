package config

import (
	"strings"
)

func parseRbac(list string) []string {
	return strings.Split(list, ",")
}
