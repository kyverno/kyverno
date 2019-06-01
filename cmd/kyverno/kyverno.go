package main

import (
	goflag "flag"
	"fmt"
	"os"

	"github.com/nirmata/kyverno/pkg/config"
	kyverno "github.com/nirmata/kyverno/pkg/kyverno"
	flag "github.com/spf13/pflag"
)

func init() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	config.LogDefaultFlags()
	flag.Parse()
}
func main() {
	cmd := kyverno.NewDefaultKyvernoCommand()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
