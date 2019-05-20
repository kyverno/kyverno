package cmd

import (
	"io"
	"os"

	"github.com/nirmata/kube-policy/pkg/kyverno/apply"
	"github.com/spf13/cobra"
)

// NewDefaultKyvernoCommand ...
func NewDefaultKyvernoCommand() *cobra.Command {
	return NewKyvernoCommand(os.Stdin, os.Stdout, os.Stderr)
}

// NewKyvernoCommand returns the new kynerno command
func NewKyvernoCommand(in io.Reader, out, errout io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   "kyverno",
		Short: "kyverno manages native policies of Kubernetes",
	}

	cmds.AddCommand(apply.NewCmdApply(in, out, errout))
	return cmds
}
