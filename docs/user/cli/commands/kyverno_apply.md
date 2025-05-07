## kyverno apply

Applies policies on resources.

### Synopsis

Applies policies on resources.

  For more information visit https://kyverno.io/docs/kyverno-cli/#apply

```
kyverno apply [flags]
```

### Examples

```
  # Apply on a resource
  kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2

  # Apply on a folder of resources
  kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resources/

  # Apply on a cluster
  kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster

  # Apply policies from a gitSourceURL on a cluster
  kyverno apply https://github.com/kyverno/policies/openshift/ --git-branch main --cluster

  # Apply single policy with variable on single resource
  kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --set <variable1>=<value1>,<variable2>=<value2>

  # Apply multiple policy with variable on multiple resource
  kyverno apply /path/to/policy1.yaml /path/to/policy2.yaml --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml -f /path/to/value.yaml
```

### Options

```
      --audit-warn                         If set to true, will flag audit policies as warnings instead of failures
  -c, --cluster                            Checks if policies should be applied to cluster in the current context
      --context string                     The name of the kubeconfig context to use
      --continue-on-fail                   If set to true, will continue to apply policies on the next resource upon failure to apply to the current resource instead of exiting out
      --detailed-results                   If set to true, display detailed results
  -e, --exception strings                  Policy exception to be considered when evaluating policies against resources
      --exceptions strings                 Policy exception to be considered when evaluating policies against resources
      --exceptions-with-resources          Evaluate policy exceptions from the resources path
      --generate-exceptions                Generate policy exceptions for each violation
      --generated-exception-ttl duration   Default TTL for generated exceptions (default 720h0m0s)
  -b, --git-branch string                  test git repository branch
  -h, --help                               help for apply
      --kubeconfig string                  path to kubeconfig file with authorization and master location information
  -n, --namespace string                   Optional Policy parameter passed with cluster flag
  -o, --output string                      Prints the mutated/generated resources in provided file/directory
  -p, --policy-report                      Generates policy report when passed (default policyviolation)
      --registry                           If set to true, access the image registry using local docker credentials to populate external data
      --remove-color                       Remove any color from output
  -r, --resource strings                   Path to resource files
      --resources strings                  Path to resource files
  -s, --set strings                        Variables that are required
  -i, --stdin                              Optional mutate policy parameter to pipe directly through to kubectl
  -t, --table                              Show results in table format
  -u, --userinfo string                    Admission Info including Roles, Cluster Roles and Subjects
  -f, --values-file string                 File containing values for policy variables
      --warn-exit-code int                 Set the exit code for warnings; if failures or errors are found, will exit 1
      --warn-no-pass                       Specify if warning exit code should be raised if no objects satisfied a policy; can be used together with --warn-exit-code flag
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

