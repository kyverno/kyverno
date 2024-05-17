## kyverno create exception

Create a Kyverno policy exception file.

### Synopsis

Create a Kyverno policy exception file.

```
kyverno create exception [name] [flags]
```

### Examples

```
  # Create a policy exception file
  kyverno create exception my-exception --namespace my-ns --policy-rules "policy,rule-1,rule-2" --any "kind=Pod,kind=Deployment,name=test-*"
```

### Options

```
      --all stringArray                                        List of resource filters
      --any stringArray                                        List of resource filters
  -b, --background                                             Set to false when policy shouldn't be considered in background scans (default true)
  -h, --help                                                   help for exception
      --namespace string                                       Policy exception namespace
  -o, --output string                                          Output path (uses standard console output if not set)
      --policy-rules --policy-rules=policy,rule-1,rule-2,...   Policy name, followed by rule names (--policy-rules=policy,rule-1,rule-2,...)
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
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

* [kyverno create](kyverno_create.md)	 - Helps with the creation of various Kyverno resources.

