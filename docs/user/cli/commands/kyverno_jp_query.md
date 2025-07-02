## kyverno jp query

Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.

### Synopsis

Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.

  For more information visit https://kyverno.io/docs/kyverno-cli/usage/jp/

```
kyverno jp query [-i input] [-q query|query]... [flags]
```

### Examples

```
  # Evaluate query
  kyverno jp query -i object.yaml 'request.object.metadata.name | truncate(@, `9`)'

  # Evaluate query
  kyverno jp query -i object.yaml -q query-file

  # Evaluate multiple queries
  kyverno jp query -i object.yaml -q query-file-1 -q query-file-2 'request.object.metadata.name | truncate(@, `9`)'

  # Cat query into
  cat query-file | kyverno jp query -i object.yaml

  # Cat object into
  cat object.yaml | kyverno jp query -q query-file
```

### Options

```
  -c, --compact         Produce compact JSON output that omits non essential whitespace
  -h, --help            help for query
  -i, --input string    Read input from a JSON or YAML file instead of stdin
  -q, --query strings   Read JMESPath expression from the specified file
  -u, --unquoted        If the final result is a string, it will be printed without quotes
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --kubeconfig string                Paths to a kubeconfig. Only required if out-of-cluster.
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

* [kyverno jp](kyverno_jp.md)	 - Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.

