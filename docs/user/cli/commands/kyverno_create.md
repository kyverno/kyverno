## kyverno create

Helps with the creation of various Kyverno resources.

### Synopsis

Helps with the creation of various Kyverno resources.
  
  Note: "kyverno create exception" (which scaffolded a legacy kyverno.io/v2 PolicyException) was removed
  in 1.20 along with legacy policy execution; there is no scaffolding replacement. See the
  policies.kyverno.io PolicyException docs at https://kyverno.io/docs/guides/exceptions/
  for the CEL PolicyException shape.

  For more information visit https://kyverno.io/docs/kyverno-cli/#create

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

* **<a id="help"></a>-h, --help**: help for create

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
* [kyverno create cluster-role](kyverno_create_cluster-role.md)	 - Create an aggregated role for given resource types
* [kyverno create metrics-config](kyverno_create_metrics-config.md)	 - Create a Kyverno metrics-config file.
* [kyverno create test](kyverno_create_test.md)	 - Create a Kyverno test file.
* [kyverno create user-info](kyverno_create_user-info.md)	 - Create a Kyverno user-info file.
* [kyverno create values](kyverno_create_values.md)	 - Create a Kyverno values file.

