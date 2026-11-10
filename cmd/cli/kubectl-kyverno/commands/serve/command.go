package serve

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

// DefaultAddress listens on loopback only.
const DefaultAddress = "127.0.0.1:8080"

type options struct {
	policies             []string
	modes                []string
	address              string
	allowRequestPolicies bool
	httpTimeout          time.Duration
}

func Command() *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:          "serve",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return opts.run(ctx, cmd)
		},
	}
	cmd.Flags().StringSliceVar(&opts.policies, "policies", nil, "Path to policy files or directories to load at startup; required unless --allow-request-policies is set")
	cmd.Flags().StringSliceVar(&opts.modes, "mode", nil, "Payload modes to serve: Kubernetes, JSON, HTTP, Envoy (repeatable or comma-separated; default all)")
	cmd.Flags().StringVar(&opts.address, "address", DefaultAddress, "Address to listen on; defaults to loopback only")
	cmd.Flags().BoolVar(&opts.allowRequestPolicies, "allow-request-policies", false, "Accept policies inside POST /v1alpha1/evaluate requests (evaluated instead of --policies); --policies is optional when set")
	cmd.Flags().DurationVar(&opts.httpTimeout, "http-timeout", DefaultHTTPTimeout, "Maximum time a policy HTTP call (CEL http.Get or http.Post) may wait for a response")
	return cmd
}

func (o options) run(ctx context.Context, cmd *cobra.Command) error {
	modes, err := parseModes(o.modes)
	if err != nil {
		return err
	}
	if len(o.policies) == 0 && !o.allowRequestPolicies {
		return fmt.Errorf("--policies is required unless --allow-request-policies is set")
	}
	if o.httpTimeout <= 0 {
		return fmt.Errorf("--http-timeout must be positive")
	}
	boundPolicyHTTP(o.httpTimeout)
	server, err := newServerFromPaths(o.policies, Config{Modes: modes, AllowRequestPolicies: o.allowRequestPolicies})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if host, _, err := net.SplitHostPort(o.address); err == nil && !isLoopback(host) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: listening on non-loopback address %s; kyverno serve has no authentication or TLS and is meant for local development only\n", o.address)
	}
	return server.ListenAndServe(ctx, o.address, func(addr string) {
		count := 0
		if server.set != nil {
			count = server.set.PolicyCount()
		}
		fmt.Fprintf(out, "kyverno serve (experimental, API %s) loaded %d policies (digest %s), modes %v, request policies allowed: %t\n", APIVersion, count, server.digest, server.modes, server.allowRequestPolicies)
		fmt.Fprintf(out, "listening on http://%s\n", addr)
	})
}

func newServerFromPaths(paths []string, cfg Config) (*Server, error) {
	// the CRD processor is process-global; set it once before loading and evaluating
	data.InjectProcessor(data.NewCRDProcessor(nil))
	var set *evaluator.PolicySet
	if len(paths) > 0 {
		var err error
		if set, err = evaluator.LoadStrict(paths...); err != nil {
			return nil, err
		}
	}
	return NewServer(set, version.Version(), cfg)
}

func parseModes(values []string) ([]evaluator.Mode, error) {
	var modes []evaluator.Mode
	seen := map[evaluator.Mode]bool{}
	for _, v := range values {
		m, err := evaluator.ParseMode(v)
		if err != nil {
			return nil, err
		}
		if !seen[m] {
			seen[m] = true
			modes = append(modes, m)
		}
	}
	return modes, nil
}

func listen(ctx context.Context, address string) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	return ln, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
