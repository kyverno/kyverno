## kyverno apply

Applies policies on resources.

### Synopsis

Applies policies on resources.

  For more information visit https://kyverno.io/docs/kyverno-cli/usage/apply/

```
kyverno apply [flags]
```

### Examples

```
  # Display evaluation results as a table
  kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --output-format table

  # Save mutated/generated resources to a directory
  kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --output-dir /path/to/results/

  # Save mutated/generated resources to a file
  kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --output-file /path/to/results.yaml

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

* **<a id="audit-warn"></a>--audit-warn**: Treat audit-only ValidatingPolicy and ImageValidatingPolicy violations as warnings (counted as warn; by default exit 0)
* **<a id="batch-size"></a>--batch-size int**: Number of resources to fetch per API call (default 100)
* **<a id="cluster"></a>-c, --cluster**: Checks if policies should be applied to cluster in the current context
* **<a id="cluster-wide-resources"></a>--cluster-wide-resources**: If set to true, will apply policies to cluster-wide resources
* **<a id="concurrent"></a>--concurrent int**: Number of concurrent workers for resource loading (default 1)
* **<a id="context"></a>--context string**: The name of the kubeconfig context to use
* **<a id="context-file"></a>--context-file string**: File containing context data for CEL policies
* **<a id="continue-on-error"></a>--continue-on-error**: Continue processing despite resource loading errors (default true)
* **<a id="continue-on-fail"></a>--continue-on-fail**: If set to true, will continue to apply policies on the next resource upon failure to apply to the current resource instead of exiting out
* **<a id="crd-paths"></a>--crd-paths strings**: List of paths to CRD files to be used for apply command
* **<a id="detailed-results"></a>--detailed-results**: If set to true, display detailed results
* **<a id="envoy-payload"></a>--envoy-payload strings**: Path to Envoy check request payload files (JSON)
* **<a id="exception"></a>-e, --exception strings**: Policy exception to be considered when evaluating policies against resources
* **<a id="exceptions"></a>--exceptions strings**: Policy exception to be considered when evaluating policies against resources
* **<a id="exceptions-with-resources"></a>--exceptions-with-resources**: Evaluate policy exceptions from the resources path
* **<a id="exceptions-within-policies"></a>--exceptions-within-policies**: Evaluate policy exceptions from the policies path
* **<a id="exceptions-within-resources"></a>--exceptions-within-resources**: Evaluate policy exceptions from the resources path
* **<a id="explain"></a>--explain**: Print how each ValidatingPolicy, MutatingPolicy, GeneratingPolicy, ImageValidatingPolicy and DeletingPolicy reached its result: whether it applied, its match conditions, variables, the images it checked, and its validations, the mutations it ran, the generate entries it ran or its delete conditions. The trace prints the values the expressions read, including resource fields and variables, so a policy that reads a Secret's data prints that data; treat the output as sensitive
* **<a id="generate-exceptions"></a>--generate-exceptions**: Generate policy exceptions for each violation
* **<a id="generated-exception-ttl"></a>--generated-exception-ttl duration**: Default TTL for generated exceptions (default 720h0m0s)
* **<a id="git-branch"></a>-b, --git-branch string**: test git repository branch
* **<a id="help"></a>-h, --help**: help for apply
* **<a id="http-payload"></a>--http-payload strings**: Path to HTTP check request payload files (JSON)
* **<a id="json"></a>--json strings**: Path to JSON payload files
* **<a id="kubeconfig"></a>--kubeconfig string**: path to kubeconfig file with authorization and master location information
* **<a id="namespace"></a>-n, --namespace string**: Optional Policy parameter passed with cluster flag
* **<a id="output-dir"></a>--output-dir string**: Directory for mutated/generated resources
* **<a id="output-file"></a>--output-file string**: File for mutated/generated resources
* **<a id="output-format"></a>--output-format string**: Output format: json or yaml for policy reports/exceptions, or table for evaluation results (default "yaml")
* **<a id="parameter-resource"></a>--parameter-resource strings**: Path to resource files that act as ValidatingAdmissionPolicy/MutatingAdmissionPolicy parameters
* **<a id="password"></a>--password string**: Password for connecting to git repository
* **<a id="policy-report"></a>-p, --policy-report**: Generates policy report when passed (default policyviolation)
* **<a id="registry"></a>--registry**: If set to true, access the image registry using local docker credentials to populate external data
* **<a id="remove-color"></a>--remove-color**: Remove any color from output
* **<a id="resource"></a>-r, --resource strings**: Path to resource files
* **<a id="resources"></a>--resources strings**: Path to resource files
* **<a id="set"></a>-s, --set strings**: Variables that are required
* **<a id="show-performance"></a>--show-performance**: Show resource loading performance metrics
* **<a id="stdin"></a>-i, --stdin**: Optional mutate policy parameter to pipe directly through to kubectl
* **<a id="table"></a>-t, --table**: Show results in table format
* **<a id="target-resource"></a>--target-resource strings**: Path to individual files containing target resources files for policies that have mutate existing
* **<a id="target-resources"></a>--target-resources strings**: Path to a directory containing target resources files for policies that have mutate existing
* **<a id="userinfo"></a>-u, --userinfo string**: Admission Info including Roles, Cluster Roles and Subjects
* **<a id="username"></a>--username string**: Username for connecting to git repository
* **<a id="values-file"></a>-f, --values-file string**: File containing values for policy variables
* **<a id="warn-exit-code"></a>--warn-exit-code int**: Set the exit code for warnings; if failures or errors are found, will exit 1
* **<a id="warn-no-pass"></a>--warn-no-pass**: Specify if warning exit code should be raised if no objects satisfied a policy; can be used together with --warn-exit-code flag
* **<a id="warnings-as-errors"></a>--warnings-as-errors**: Treat deprecation warnings as errors

### Options inherited from parent commands

* **<a id="add_dir_header"></a>--add\_dir\_header**: If true, adds the file directory to the header of the log messages
* **<a id="alsologtostderr"></a>--alsologtostderr**: log to standard error as well as files (no effect when -logtostderr=true)
* **<a id="alsologtostderrthreshold"></a>--alsologtostderrthreshold severity**: logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
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

