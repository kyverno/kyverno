# Logging

Canonical doc: [docs/dev/logging/logging.md](../../dev/logging/logging.md). This file is a pointer, not a copy —
edit rules there.

Quick summary: `logr` API surface, `zerologr` backend for app logs, `klogr` for client-go/Kubernetes library logs.
Default verbosity 2. L0 = errors (with stack trace), L2 = startup info / policy-application results, L3 = variable
evaluation / intermediate decisions, L4+ = deep debugging. `logging.ControllerLogger("name")` sets level 3 for
controller loggers specifically. Gotcha: call depth must be set correctly relative to `logging.Setup()` — see the
canonical doc before wrapping the logger.
