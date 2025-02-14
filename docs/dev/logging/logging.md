# Logging Guideline

Logging in Kyverno follows a structured approach using `logr`, with `zerologr` for structured logging and `klogr` for Kubernetes client logs, ensuring consistency and standardization.

## **Log Levels**
- **Default log level:** `2`
- Can be modified via command-line argument: `-v=<level_int>`.
- Uses **zerologr** for structured logging (verbose logs) and **klogr** for client logs, both implemented through `logr` (Go's built-in logging interface).

> :warning: Initially, call depth applied to the logger does not get set. When `logging.Setup` is called in `main`, `globalLog` is switched to the real logger, meaning all loggers created after `logging.Setup` will work correctly if the underlying sink supports it.

## **Identifiers**
- `WithName`: `logging.WithName("setup")` → Adds "setup" as a prefix.
- `WithValues`: `logging.WithValues("key", "value")` → Adds key-value pairs to logs.
- `ControllerLogger`: `logging.ControllerLogger("name")` → Creates a logger for controllers, setting a log level of 3.

## **Error Logging (L0)**
```bash
logging.Error(err, "failed due to this error", "key", "value")
logging.Info("failed due to this error", err)

```
- Prints a **stack trace** where the error was defined for deeper debugging.
- **Fatal logs** do not exist; the caller must use `os.Exit()` explicitly.

## **Startup Info & Policy Application Results (L2)**
```bash
logging.V(2).Info("starting controller", "workers", c.workers)
logging.V(2).Info("setup metrics", "otel", otel, "port", metricsPort)
```
- Logs providing information about startup processes or policy application results.

## **Variable Evaluation Logs (L3)**
```bash
logging.V(3).Info("evaluating variable", "value", someVariable)
logging.V(3).Info("failed to fetch UR, falling back", "reason", err.Error())
```
- Logs related to dynamic evaluations, variable processing, or intermediate policy decisions.

## **Debugging Logs (L4 & Above)**
```bash
logging.V(4).Info("fetching downstream resource", "APIVersion", generatePattern.GetAPIVersion())
logging.V(4).Info("ForceFailurePolicyIgnore is enabled, all policies with failures will be set to Ignore")
```
- Detailed logs useful for debugging execution paths.
- Shutdown messages should also be **logged at L4**.

---
