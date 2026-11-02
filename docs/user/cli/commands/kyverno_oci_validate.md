## kyverno oci validate

Validate a bundle directory or file, or a pushed OCI image, reporting exactly what "kyverno oci push" would reject, without pushing anything.

### Synopsis

Validate a bundle directory or file, or a pushed OCI image, reporting exactly what "kyverno oci push" would reject, without pushing anything.
  Local mode (a directory or file argument) runs every check push runs up to the upload step: no network call is made.
  Reference mode (--image) fetches a pushed image, re-runs the same strict checks over it, and can fail on content "kyverno oci pull" would accept, such as an exception referencing a policy outside the bundle; use pull to extract such a bundle instead.

  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.

  For more information visit https://kyverno.io/docs/kyverno-cli/#pushing

```
kyverno oci validate [dir or file] [flags]
```

### Examples

```
  # Validate a local bundle directory before pushing it
  kyverno oci validate ./bundle

  # Validate a local bundle, failing on warnings such as an expired exception
  kyverno oci validate ./bundle --strict

  # Validate a pushed bundle against a specific target Kyverno version
  kyverno oci validate -i <imgref> --kyverno-version 1.20.0
```

### Options

```
  -h, --help                     help for validate
  -i, --image string             image reference of a pushed bundle to validate
      --kyverno-version string   Kyverno version to check the bundle's declared kyvernoVersion constraint against; an unsatisfied constraint fails
      --strict                   treat warnings (such as an expired exception) as failures
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

* [kyverno oci](kyverno_oci.md)	 - Pulls/pushes images that include policie(s) from/to OCI registries.

