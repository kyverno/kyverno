## kyverno oci validate

Validate a bundle directory or file, or a pushed OCI image, without pushing anything. By default, local validate succeeds exactly when "kyverno oci push" would reach its upload step; --kyverno-version and --strict both change that contract (see below).

### Synopsis

Validate a bundle directory or file, or a pushed OCI image, without pushing anything. By default, local validate succeeds exactly when "kyverno oci push" would reach its upload step; --kyverno-version and --strict both change that contract (see below).
  Local mode (a directory or file argument) runs every check push runs up to the upload step: no network call is made.
  Reference mode (--image) fetches a pushed image, re-runs the same strict checks over it, and can fail on content "kyverno oci pull" would accept, such as an exception referencing a policy outside the bundle; use pull to extract such a bundle instead.
  --kyverno-version compares the bundle's kyvernoVersion constraint against the given version and fails on a mismatch; a plain push or validate only warns, since they compare against the running CLI's own version.
  --strict additionally fails on conditions push and plain validate only warn about, such as an expired exception or a compensating control ignored for its policy's kind.

  For more information visit https://kyverno.io/docs/kyverno-cli/#oci

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

* **<a id="help"></a>-h, --help**: help for validate
* **<a id="image"></a>-i, --image string**: image reference of a pushed bundle to validate
* **<a id="kyverno-version"></a>--kyverno-version string**: Kyverno version to check the bundle's declared kyvernoVersion constraint against; an unsatisfied constraint fails
* **<a id="strict"></a>--strict**: treat warnings (such as an expired exception) as failures

### Options inherited from parent commands

* **<a id="add_dir_header"></a>--add\_dir\_header**: If true, adds the file directory to the header of the log messages
* **<a id="alsologtostderr"></a>--alsologtostderr**: log to standard error as well as files (no effect when -logtostderr=true)
* **<a id="alsologtostderrthreshold"></a>--alsologtostderrthreshold severity**: logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
* **<a id="kubeconfig"></a>--kubeconfig string**: Paths to a kubeconfig. Only required if out-of-cluster.
* **<a id="legacy_stderr_threshold_behavior"></a>--legacy\_stderr\_threshold\_behavior**: If true, stderrthreshold is ignored when logtostderr=true (legacy behavior). If false, stderrthreshold is honored even when logtostderr=true (default true)
* **<a id="log_backtrace_at"></a>--log\_backtrace\_at traceLocation**: when logging hits line file:N, emit a stack trace (default :0)
* **<a id="log_dir"></a>--log\_dir string**: If non-empty, write log files in this directory (no effect when -logtostderr=true)
* **<a id="log_file"></a>--log\_file string**: If non-empty, use this log file (no effect when -logtostderr=true)
* **<a id="log_file_max_size"></a>--log\_file\_max\_size uint**: Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
* **<a id="logtostderr"></a>--logtostderr**: log to standard error instead of files (default true)
* **<a id="one_output"></a>--one\_output**: If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
* **<a id="skip_headers"></a>--skip\_headers**: If true, avoid header prefixes in the log messages
* **<a id="skip_log_headers"></a>--skip\_log\_headers**: If true, avoid headers when opening log files (no effect when -logtostderr=true)
* **<a id="stderrthreshold"></a>--stderrthreshold severity**: logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true unless -legacy_stderr_threshold_behavior=false) (default 2)
* **<a id="v"></a>-v, --v Level**: number for the log level verbosity
* **<a id="vmodule"></a>--vmodule moduleSpec**: comma-separated list of pattern=N settings for file-filtered logging

### SEE ALSO

* [kyverno oci](kyverno_oci.md)	 - Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.

