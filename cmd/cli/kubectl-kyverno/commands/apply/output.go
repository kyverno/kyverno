package apply

import (
	"fmt"

	"github.com/spf13/cobra"
)

// resolveOutputFlags validates output options and warns about ignored flags.
func (c *ApplyCommandConfig) resolveOutputFlags(cmd *cobra.Command, table, detailedResults bool) (bool, error) {
	structured := c.PolicyReport || c.GenerateExceptions
	formatChanged := cmd.Flags().Changed("output-format")
	if c.OutputFormat == "table" {
		if structured {
			return false, fmt.Errorf("--output-format table cannot be used with --policy-report or --generate-exceptions")
		}
		table = true
	}

	warn := func(message string) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Warning:", message)
	}
	if structured && c.OutputFormat != "json" && c.OutputFormat != "yaml" {
		warn(fmt.Sprintf("unsupported output format %q; using yaml", c.OutputFormat))
	}
	if c.PolicyReport && c.GenerateExceptions {
		warn("--generate-exceptions is ignored when --policy-report is set")
	}
	if table && structured {
		warn("--table is ignored when --policy-report or --generate-exceptions is set")
	} else if formatChanged && c.OutputFormat != "table" && !structured {
		warn(fmt.Sprintf("--output-format %s has no effect without --policy-report or --generate-exceptions", c.OutputFormat))
	}
	if detailedResults && (!table || structured) {
		warn("--detailed-results has no effect without table output")
	}
	return table, nil
}
