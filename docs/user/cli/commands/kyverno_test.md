## kyverno test

Run tests from a local filesystem or a remote git repository.

### Synopsis

Run tests from a local filesystem or a remote git repository.
  
  The test command provides a facility to test resources against policies by comparing expected results,
  declared ahead of time in a test manifest file, to actual results reported by Kyverno.
  
  Users provide the path to the folder containing a kyverno-test.yaml file where the location could be
  on a local filesystem or a remote git repository.

  For more information visit https://kyverno.io/docs/kyverno-cli/usage/test/

```
kyverno test [local folder or git repository]... [flags]
```

### Examples

```
  # Test a git repository containing Kyverno test cases
  kyverno test https://github.com/kyverno/policies/pod-security --git-branch main

  # Test a local folder containing test cases
  kyverno test .

  # Test some specific test cases out of many test cases in a local folder
  kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"
```

### Options

```
      --detailed-results            If set to true, display detailed results
      --fail-only                   If set to true, display all the failing test only as output for the test command
  -f, --file-name string            Test filename (default "kyverno-test.yaml")
  -b, --git-branch string           Test github repository branch
  -h, --help                        help for test
  -o, --output-format string        Specifies the output format (json, yaml, markdown, junit)
      --registry                    If set to true, access the image registry using local docker credentials to populate external data
      --remove-color                Remove any color from output
      --require-tests               If set to true, return an error if no tests are found
  -t, --test-case-selector string   Filter test cases to run (default "policy=*,rule=*,resource=*")
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

* [kyverno](kyverno.md)	 - Kubernetes Native Policy Management.

