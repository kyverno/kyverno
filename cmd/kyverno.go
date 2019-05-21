package main

import (
	"fmt"
	"os"

	kyverno "github.com/nirmata/kyverno/pkg/kyverno"
)

func main() {
	cmd := kyverno.NewDefaultKyvernoCommand()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
