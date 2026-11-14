## kyverno

Kubernetes Native Policy Management.

### Synopsis

Kubernetes Native Policy Management.
  
  The Kyverno CLI provides a command-line interface to work with Kyverno resources.
  It can be used to validate and test policy behavior to resources prior to adding them to a cluster.
  
  The Kyverno CLI comes with additional commands to help creating and manipulating various Kyverno resources.
  
  NOTE: To enable experimental commands, environment variable "KYVERNO_EXPERIMENTAL" should be set true or 1.

  For more information visit https://kyverno.io/docs/kyverno-cli

```
kyverno [flags]
```

### Options

* **<a id="add_dir_header"></a>--add\_dir\_header**: If true, adds the file directory to the header of the log messages
* **<a id="alsologtostderr"></a>--alsologtostderr**: log to standard error as well as files (no effect when -logtostderr=true)
* **<a id="alsologtostderrthreshold"></a>--alsologtostderrthreshold severity**: logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
* **<a id="help"></a>-h, --help**: help for kyverno
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

* [kyverno apply](kyverno_apply.md)	 - Applies policies on resources.
* [kyverno completion](kyverno_completion.md)	 - Generate the autocompletion script for kyverno for the specified shell.
* [kyverno create](kyverno_create.md)	 - Helps with the creation of various Kyverno resources.
* [kyverno docs](kyverno_docs.md)	 - Generates reference documentation.
* [kyverno fix](kyverno_fix.md)	 - Fix inconsistencies and deprecated usage of Kyverno resources.
* [kyverno jp](kyverno_jp.md)	 - Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.
* [kyverno migrate](kyverno_migrate.md)	 - Migrate one or more resources to the stored version.
* [kyverno oci](kyverno_oci.md)	 - Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.
* [kyverno test](kyverno_test.md)	 - Run tests from a local filesystem or a remote git repository.
* [kyverno version](kyverno_version.md)	 - Prints the version of Kyverno CLI.

