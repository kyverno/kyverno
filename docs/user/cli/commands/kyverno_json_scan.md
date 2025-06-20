## kyverno json scan

Runs tests against any json compatible payloads/policies.

### Synopsis

Runs tests against any json compatible payloads/policies.

  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.

```
kyverno json scan [flags]
```

### Options

```
  -h, --help                  help for scan
      --labels strings        Labels selectors for policies
      --output string         Output format (text or json) (default "text")
      --payload string        Path to payload (json or yaml file)
      --policy strings        Path to kyverno-json policies
      --pre-process strings   JMESPath expression used to pre process payload
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

* [kyverno json](kyverno_json.md)	 - Runs tests against any json compatible payloads/policies.

