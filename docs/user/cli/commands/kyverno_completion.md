## kyverno completion

Generate the autocompletion script for kyverno for the specified shell.

### Synopsis

Generate the autocompletion script for kyverno for the specified shell.
  
  Shell autocompletion enables tab completion for kyverno commands, subcommands, flags, and arguments.
  This significantly improves CLI usability by providing command suggestions and reducing typing.
  
  The generated script contains shell-specific functions that integrate with your shell's
  completion system to provide intelligent command completion when you press the Tab key.
  
  To enable autocompletion, source the generated script in your shell profile or save it
  to your shell's completion directory. See the examples below for shell-specific instructions.

  For more information visit https://kyverno.io/docs/kyverno-cli/#shell-autocompletion

```
kyverno completion [bash|zsh|fish|powershell]
```

### Examples

```
  # Generate and install bash completion (Linux)
  kyverno completion bash > /etc/bash_completion.d/kyverno

  # Generate and source bash completion for current session
  source <(kyverno completion bash)

  # Generate and install zsh completion
  kyverno completion zsh > "${fpath[1]}/_kyverno"

  # Generate and source zsh completion for current session
  source <(kyverno completion zsh)

  # Generate and install fish completion
  kyverno completion fish > ~/.config/fish/completions/kyverno.fish

  # Generate PowerShell completion
  kyverno completion powershell | Out-String | Invoke-Expression

  # To permanently enable PowerShell completion, add to your profile:
  kyverno completion powershell >> $PROFILE
```

### Options

```
  -h, --help   help for completion
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

