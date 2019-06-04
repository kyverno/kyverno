/*

Command gty-migrate-from-testify migrates packages from
testify/assert and testify/require to gotest.tools/assert.

	$ go get gotest.tools/assert/cmd/gty-migrate-from-testify

Usage:

	gty-migrate-from-testify [OPTIONS] PACKAGE [PACKAGE...]

See --help for full usage.


To run on all packages (including external test packages) use:

	go list \
		-f '{{.ImportPath}} {{if .XTestGoFiles}}{{"\n"}}{{.ImportPath}}_test{{end}}' \
		./... | xargs gty-migrate-from-testify

*/
package main
