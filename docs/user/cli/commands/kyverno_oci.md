## kyverno oci

Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.

### Synopsis

Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.

  For more information visit https://kyverno.io/docs/kyverno-cli/#oci

```
kyverno oci [flags]
```

### Examples

```
  # Push a policy bundle from a directory to an OCI image
  kyverno oci push policies -i ghcr.io/acme/baseline:v1.0.0

  # Pull a pushed bundle into a directory
  kyverno oci pull policies -i ghcr.io/acme/baseline:v1.0.0

  # Validate a pushed bundle without pulling it
  kyverno oci validate -i ghcr.io/acme/baseline:v1.0.0

  # Inspect a pushed bundle's metadata and Kyverno compatibility
  kyverno oci inspect -i ghcr.io/acme/baseline:v1.0.0

  # Resolve a tag to its digest-pinned reference
  kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0
```

### Options

* **<a id="help"></a>-h, --help**: help for oci

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

* [kyverno](kyverno.md)	 - Kubernetes Native Policy Management.
* [kyverno oci inspect](kyverno_oci_inspect.md)	 - Show a pushed bundle's resolved digest, resource index, sets, source and revision metadata, manifest annotations, and Kyverno compatibility.
* [kyverno oci pull](kyverno_oci_pull.md)	 - Pulls policie(s) that are included in an OCI image from OCI registry and saves them to a local directory.
* [kyverno oci push](kyverno_oci_push.md)	 - Push policie(s) that are included in an OCI image to OCI registry.
* [kyverno oci resolve](kyverno_oci_resolve.md)	 - Resolve a bundle tag to its immutable digest and print a digest-pinned reference you can pass to "kyverno test", "kyverno apply", "kyverno oci pull", or a Flux OCIRepository.
* [kyverno oci validate](kyverno_oci_validate.md)	 - Validate a bundle directory or file, or a pushed OCI image, without pushing anything. By default, local validate succeeds exactly when "kyverno oci push" would reach its upload step; --kyverno-version and --strict both change that contract (see below).

