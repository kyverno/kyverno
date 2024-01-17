## kyverno jp parse

Parses jmespath expression and shows corresponding AST.

### Synopsis

Parses jmespath expression and shows corresponding AST.

  For more information visit https://kyverno.io/docs/kyverno-cli/#jp

```
kyverno jp parse [-f file|expression]... [flags]
```

### Examples

```
  # Parse expression
  kyverno jp parse 'request.object.metadata.name | truncate(@, `9`)'

  # Parse expression from a file
  kyverno jp parse -f my-file

  # Parse expression from stdin
  kyverno jp parse

  # Parse multiple expressionxs
  kyverno jp parse -f my-file1 -f my-file-2 'request.object.metadata.name | truncate(@, `9`)'

  # Cat into
  cat my-file | kyverno jp parse
```

### Options

```
  -f, --file strings   Read input from a JSON or YAML file instead of stdin
  -h, --help           help for parse
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

* [kyverno jp](kyverno_jp.md)	 - Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.

