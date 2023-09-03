# kubectl-kyverno

<a href="https://kyverno.io" rel="kyverno.io">![logo](../../../img/Kyverno_Horizontal.png)</a>

This repository contains [Kyverno CLI](https://kyverno.io/docs/kyverno-cli/) source code.

The CLI can be used as a standalone tool or as a kubectl plugin.

## 📙 Documentation

👉 **[Installation](https://kyverno.io/docs/kyverno-cli/#building-and-installing-the-cli)**

👉 **[Installation](https://kyverno.io/docs/kyverno-cli/#cli-commands)**

👉 **[Reference docs](../../../docs/user/cli/kyverno.md)**

## 🔧 GitHub Action

You can install the Kyverno CLI in your GitHub workflows easily using the [kyverno-cli-installer](https://github.com/kyverno/action-install-cli) GitHub action.

Check the documentation in the [GitHub repository](https://github.com/kyverno/action-install-cli) or [GitHub marketplace](https://github.com/marketplace/actions/kyverno-cli-installer).

## 🙋‍♂️ Help

Use `kyverno --help` to list supported commands and their corresponding flags:

```shell
To enable experimental commands, KYVERNO_EXPERIMENTAL should be configured with true or 1.

Usage:
  kyverno [command]

Available Commands:
  apply       Applies policies on resources.
  completion  Generate the autocompletion script for the specified shell
  create      Provides a command-line interface to help with the creation of various Kyverno resources.
  docs        Generates documentation.
  help        Help about any command
  jp          Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.
  test        Run tests from directory.
  version     Shows current version of kyverno.

Flags:
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
  -h, --help                             help for kyverno
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one_output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

To enable experimental commands, `KYVERNO_EXPERIMENTAL` should be configured with true or 1.

## License

Copyright 2023, the Kyverno project. All rights reserved. Kyverno is licensed under the [Apache License 2.0](LICENSE).

Kyverno is a [Cloud Native Computing Foundation (CNCF) Incubating project](https://www.cncf.io/projects/) and was contributed by [Nirmata](https://nirmata.com/?utm_source=github&utm_medium=repository).
