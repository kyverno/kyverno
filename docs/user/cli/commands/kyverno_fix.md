## kyverno fix

Fix inconsistencies and deprecated usage of Kyverno resources.

### Synopsis

Fix inconsistencies and deprecated usage of Kyverno resources.
  
  The fix command provides a command-line interface to fix inconsistencies and deprecated usage of Kyverno resources.
  It can be used to fix Kyverno test files.

  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.

```
kyverno fix [flags]
```

### Examples

```
  # Fix Kyverno test files
  KYVERNO_EXPERIMENTAL=true kyverno fix test . --save
```

### Options

```
  -h, --help   help for fix
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

* [kyverno](kyverno.md)	 - Kubernetes Native Policy Management.
* [kyverno fix policy](kyverno_fix_policy.md)	 - Fix inconsistencies and deprecated usage in Kyverno policy files.
* [kyverno fix test](kyverno_fix_test.md)	 - Fix inconsistencies and deprecated usage in Kyverno test files.

