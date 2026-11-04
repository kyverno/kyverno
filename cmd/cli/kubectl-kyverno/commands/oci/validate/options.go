package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/pkg/version"
)

type options struct {
	imageRef      string
	kyvernoTarget string
	strict        bool
}

// validate enforces D1's mutual-exclusion rule: exactly one of a positional path or --image.
func (o options) validate(args []string) error {
	hasPath := len(args) == 1
	hasImage := o.imageRef != ""
	switch {
	case hasPath && hasImage:
		return errors.New("specify either a bundle directory or file, or --image, not both")
	case !hasPath && !hasImage:
		return errors.New("a bundle directory, file, or --image reference is required")
	}
	if o.kyvernoTarget != "" {
		if _, ok := bundle.NormalizeKyvernoVersion(o.kyvernoTarget); !ok {
			return fmt.Errorf("--kyverno-version %q is not a release version", o.kyvernoTarget)
		}
	}
	return nil
}

// execute runs local or reference mode depending on which of args/--image was set, then applies
// the warning/strict/target contract shared by both: warnings go to stderr, --strict promotes any
// warning to a failure, and an explicit --kyverno-version target makes an Unsatisfied comparison
// a hard failure instead of a warning.
func (o options) execute(ctx context.Context, args []string, keychain authn.Keychain) error {
	var (
		b          *bundle.Bundle
		constraint string
		source     string
	)

	warnCount := 0
	warn := func(msg string) {
		warnCount++
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	}

	if len(args) == 1 {
		dir := args[0]
		source = dir
		assembled, err := bundle.Assemble(dir)
		if err != nil {
			return fmt.Errorf("assembling bundle from %s: %w", dir, err)
		}
		b = assembled

		if err := bundle.ValidateWithOptions(b, bundle.Options{Warn: warn}); err != nil {
			return err
		}

		d, err := bundle.LoadDescriptor(dir, b.Documents)
		if err != nil {
			return err
		}
		if d != nil {
			constraint = d.KyvernoVersion
		}
		// Write runs so validate stops at the same place push would, including resolveCreated's
		// SOURCE_DATE_EPOCH check and assignSets; the resulting image is discarded (no network).
		if _, err := bundle.Write(b, d); err != nil {
			return fmt.Errorf("writing bundle image: %w", err)
		}
	} else {
		img, ref, err := pull.FetchImage(ctx, o.imageRef, keychain)
		if err != nil {
			return err
		}
		source = ref.Name()

		tmp, err := os.MkdirTemp("", "kyverno-oci-validate-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		defer os.RemoveAll(tmp)

		read, err := bundle.Read(img, tmp)
		if err != nil {
			return err
		}
		b = read

		if err := bundle.ValidateWithOptions(b, bundle.Options{Warn: warn}); err != nil {
			return err
		}

		configBytes, err := img.RawConfigFile()
		if err != nil {
			return fmt.Errorf("reading image config: %w", err)
		}
		var cfg bundle.Config
		if err := json.Unmarshal(configBytes, &cfg); err != nil {
			return fmt.Errorf("decoding image config: %w", err)
		}
		constraint = cfg.KyvernoVersion
	}

	target := o.kyvernoTarget
	hardFailOnUnsatisfied := target != ""
	if target == "" {
		target = version.Version()
	}
	if constraint != "" {
		c, err := bundle.CheckKyvernoVersion(constraint, target)
		if err != nil {
			return err
		}
		if c == bundle.CompatibilityUnsatisfied && hardFailOnUnsatisfied {
			return fmt.Errorf("bundle declares kyvernoVersion %q, which Kyverno %s does not satisfy", constraint, target)
		}
		if text, isWarning := bundle.DescribeCompatibility(c, constraint, target); text != "" {
			// text already carries its own "Warning:"/"Note:" prefix (DescribeCompatibility), unlike
			// bundle.Options.Warn callers elsewhere, which receive a bare message; print it as-is and
			// only bump warnCount for the warning case so --strict still promotes it.
			fmt.Fprintln(os.Stderr, text)
			if isWarning {
				warnCount++
			}
		}
	}

	if o.strict && warnCount > 0 {
		return fmt.Errorf("validation produced %d warning(s) and --strict is set", warnCount)
	}

	n := len(b.Documents)
	msg := fmt.Sprintf("Validated %d resource(s) from %s", n, source)
	if warnCount > 0 {
		msg = fmt.Sprintf("%s (%d warning(s))", msg, warnCount)
	}
	fmt.Println(msg)
	return nil
}
