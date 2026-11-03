package inspect

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/pkg/version"
)

type options struct {
	imageRef      string
	output        string
	kyvernoTarget string
}

func (o options) validate() error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if o.output != "" && o.output != "json" && o.output != "yaml" {
		return fmt.Errorf("invalid output format: %s (supported formats: json, yaml)", o.output)
	}
	if o.kyvernoTarget != "" {
		if _, ok := bundle.NormalizeKyvernoVersion(o.kyvernoTarget); !ok {
			return fmt.Errorf("--kyverno-version %q is not a release version", o.kyvernoTarget)
		}
	}
	return nil
}

// execute fetches imageRef's manifest and config (D2's read path: pull.FetchDescriptor, the
// shared index guard, bundle.ReadMetadata — never the content layer) and renders a Report. out
// carries the result (text, or -o json|yaml); errOut carries text mode's annotation/config
// mismatch warnings, which structured mode instead carries in Report.Warnings with nothing
// written to errOut, so a script reading stdout alone never sees stray diagnostic lines.
func (o options) execute(ctx context.Context, out, errOut io.Writer, keychain authn.Keychain) error {
	desc, ref, err := pull.FetchDescriptor(ctx, o.imageRef, keychain)
	if err != nil {
		return err
	}
	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("loading image from manifest: %v", err)
	}

	md, err := bundle.ReadMetadata(img)
	if err != nil {
		return err
	}

	target := o.kyvernoTarget
	if target == "" {
		target = version.Version()
	}

	report, warnings := buildReport(ref, desc, md, target)

	if o.output == "" {
		for _, w := range warnings {
			fmt.Fprintf(errOut, "Warning: %s\n", w)
		}
		return writeText(out, report, md.Config)
	}

	switch o.output {
	case "json":
		return writeJSON(out, report)
	case "yaml":
		return writeYAML(out, report)
	default:
		return fmt.Errorf("invalid output format: %s (supported formats: json, yaml)", o.output)
	}
}
