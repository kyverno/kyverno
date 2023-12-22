## kyverno create

Helps with the creation of various Kyverno resources.

### Synopsis

Helps with the creation of various Kyverno resources.

```
kyverno create [flags]
```

### Examples

```
  # Create metrics config file
  kyverno create metrics-config -i ns-included-1 -i ns-included-2 -e ns-excluded

  # Create test file
  kyverno create test -p policy.yaml -r resource.yaml -f values.yaml --pass policy-name,rule-name,resource-name,resource-namespace,resource-kind

  # Create user info file
  kyverno create user-info -u molybdenum@somecorp.com -g basic-user -c admin

  # Create values file
  kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo
```

### Options

```
  -h, --help   help for create
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
* [kyverno create exception](kyverno_create_exception.md)	 - Create a Kyverno policy exception file.
* [kyverno create metrics-config](kyverno_create_metrics-config.md)	 - Create a Kyverno metrics-config file.
* [kyverno create test](kyverno_create_test.md)	 - Create a Kyverno test file.
* [kyverno create user-info](kyverno_create_user-info.md)	 - Create a Kyverno user-info file.
* [kyverno create values](kyverno_create_values.md)	 - Create a Kyverno values file.

