package wildcard

import (
	"github.com/gobwas/glob"
)

func Match(pattern, name string) bool {
	var g glob.Glob
	g = glob.MustCompile(pattern)
	return g.Match(name)
}
