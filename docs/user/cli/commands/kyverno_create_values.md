## kyverno create values

Create a Kyverno values file.

### Synopsis

Create a Kyverno values file.

```
kyverno create values [flags]
```

### Examples

```
  # Create values file
  kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo
```

### Options

```
  -g, --global strings            Global value
  -h, --help                      help for values
  -n, --ns-selector stringArray   Namespace selector
  -o, --output string             Output path (uses standard console output if not set)
      --resource stringArray      Policy resource values
      --rule stringArray          Policy rule values
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

