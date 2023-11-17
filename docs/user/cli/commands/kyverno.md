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

```
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
  -h, --help                             help for kyverno
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one_output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [kyverno apply](kyverno_apply.md)	 - Applies policies on resources.
* [kyverno completion](kyverno_completion.md)	 - Generate the autocompletion script for the specified shell
* [kyverno create](kyverno_create.md)	 - Helps with the creation of various Kyverno resources.
* [kyverno docs](kyverno_docs.md)	 - Generates reference documentation.
* [kyverno fix](kyverno_fix.md)	 - Fix inconsistencies and deprecated usage of Kyverno resources.
* [kyverno jp](kyverno_jp.md)	 - Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.
* [kyverno oci](kyverno_oci.md)	 - Pulls/pushes images that include policie(s) from/to OCI registries.
* [kyverno test](kyverno_test.md)	 - Run tests from a local filesystem or a remote git repository.
* [kyverno version](kyverno_version.md)	 - Prints the version of Kyverno CLI.

