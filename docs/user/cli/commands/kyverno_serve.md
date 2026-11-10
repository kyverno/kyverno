## kyverno serve

Run the Kyverno CLI policy evaluation as a local HTTP server (experimental, developer preview).

### Synopsis

Run the Kyverno CLI policy evaluation as a local HTTP server (experimental, developer preview).
  
  Policies are loaded once at startup from local files or directories; startup fails if any
  policy cannot be loaded. Payloads are then evaluated with POST /v1alpha1/evaluate/{mode},
  where mode is Kubernetes, JSON, HTTP or Envoy, using the same evaluation path as "kyverno apply".
  With --allow-request-policies, POST /v1alpha1/evaluate also accepts an EvaluationRequest that
  carries its own policies, exceptions, context, parameters and operation.
  
  Endpoints:
    POST /v1alpha1/evaluate/{mode}  evaluate a raw JSON/YAML payload
    POST /v1alpha1/evaluate         evaluate an EvaluationRequest (payload + context [+ policies])
    GET  /v1alpha1/policies         list the loaded policies and the policy-set digest
    GET  /healthz, /readyz          health and readiness
  
  The v1alpha1 API is experimental and may change in any release without notice. The server
  has no authentication or TLS, never writes to a cluster, evaluates one request at a time, and
  is intended for local development and testing only. It listens on loopback by default.

  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.

  For more information visit https://kyverno.io/docs/kyverno-cli/#serve

```
kyverno serve [flags]
```

### Examples

```
  # Serve the policies in a directory on the default address (127.0.0.1:8080)
  KYVERNO_EXPERIMENTAL=true kyverno serve --policies ./policies

  # Evaluate a Kubernetes resource
  curl -s -X POST -H 'Content-Type: application/yaml' --data-binary @pod.yaml http://127.0.0.1:8080/v1alpha1/evaluate/Kubernetes

  # Serve only Envoy check requests on another port
  KYVERNO_EXPERIMENTAL=true kyverno serve --policies ./envoy-policies --mode Envoy --address 127.0.0.1:9000
```

### Options

```
      --address string           Address to listen on; defaults to loopback only (default "127.0.0.1:8080")
      --allow-request-policies   Accept policies inside POST /v1alpha1/evaluate requests (evaluated instead of --policies); --policies is optional when set
  -h, --help                     help for serve
      --http-timeout duration    Maximum time a policy HTTP call (CEL http.Get or http.Post) may wait for a response (default 30s)
      --mode strings             Payload modes to serve: Kubernetes, JSON, HTTP, Envoy (repeatable or comma-separated; default all)
      --policies strings         Path to policy files or directories to load at startup; required unless --allow-request-policies is set
```

### Options inherited from parent commands

```
      --add_dir_header                      If true, adds the file directory to the header of the log messages
      --alsologtostderr                     log to standard error as well as files (no effect when -logtostderr=true)
      --alsologtostderrthreshold severity   logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
      --kubeconfig string                   Paths to a kubeconfig. Only required if out-of-cluster.
      --legacy_stderr_threshold_behavior    If true, stderrthreshold is ignored when logtostderr=true (legacy behavior). If false, stderrthreshold is honored even when logtostderr=true (default true)
      --log_backtrace_at traceLocation      when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                      If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                     If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint              Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                         log to standard error instead of files (default true)
      --one_output                          If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                        If true, avoid header prefixes in the log messages
      --skip_log_headers                    If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity            logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true unless -legacy_stderr_threshold_behavior=false) (default 2)
  -v, --v Level                             number for the log level verbosity
      --vmodule moduleSpec                  comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [kyverno](kyverno.md)	 - Kubernetes Native Policy Management.

