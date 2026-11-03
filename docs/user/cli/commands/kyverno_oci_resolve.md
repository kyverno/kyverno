## kyverno oci resolve

Resolve a bundle tag to its immutable digest and print a digest-pinned reference you can pass to "kyverno test", "kyverno apply", "kyverno oci pull", or a Flux OCIRepository.

### Synopsis

Resolve a bundle tag to its immutable digest and print a digest-pinned reference you can pass to "kyverno test", "kyverno apply", "kyverno oci pull", or a Flux OCIRepository.
  The command fetches the manifest and config, checks that it is a Kyverno bundle, and prints exactly one line to stdout; it never downloads the content layer. Use --digest-only for the bare digest.

  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.

  For more information visit https://kyverno.io/docs/kyverno-cli/#oci

```
kyverno oci resolve [flags]
```

### Examples

```
  # Resolve a tag to its digest-pinned reference
  kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0

  # Run kyverno test against the resolved, digest-pinned bundle
  kyverno test oci://$(kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0)

  # Print only the bare digest
  kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0 --digest-only
```

### Options

```
      --digest-only    print only the digest (sha256:...) instead of the full digest-pinned reference
  -h, --help           help for resolve
  -i, --image string   image reference (tag or digest) of a pushed bundle to resolve
```

### Options inherited from parent commands

```
      --add_dir_header                      If true, adds the file directory to the header of the log messages
      --alsologtostderr                     log to standard error as well as files (no effect when -logtostderr=true)
      --alsologtostderrthreshold severity   logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
      --kubeconfig string                   Paths to a kubeconfig. Only required if out-of-cluster.
      --legacy_stderr_threshold_behavior    If true, stderrthreshold is ignored when logtostderr=true (legacy behavior). If false, stderrthreshold is honored even when logtostderr=true (default true)
      --log_backtrace_at traceLocation      when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                      If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                     If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint              Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                         log to standard error instead of files (default true)
      --one_output                          If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                        If true, avoid header prefixes in the log messages
      --skip_log_headers                    If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity            logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true unless -legacy_stderr_threshold_behavior=false) (default 2)
  -v, --v Level                             number for the log level verbosity
      --vmodule moduleSpec                  comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [kyverno oci](kyverno_oci.md)	 - Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.

