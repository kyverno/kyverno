# Logging Guideline

Logging is done using `logging` package in the codebase. The logger relies on sigs.k8s.io/controller-runtime/pkg/log that provides methods to define logs.

Default log level is 2. Can be modified by adding command-line argument `-v=<level_int>`.



The `globalLog` variable in logging package is a reference to logr.Logger from controller-runtime/pkg/log.


> :warning:
> Initially, call depth applied to the logger do not get set. When logging.Setup is called in main, globalLog is switched to the real logger, in turn, all loggers created after logging.Setup won't be subject to the call depth limitation and will work if the underlying sink supports it.


### Identifiers:

```
WithName: logging.WithName("setup") : adds "setup" as prefix.

WithValues: logging.WithValues("key","value") : adds key-value pairs.

ControllerLogger: logging.ControllerLogger("name") : creates a logger to be used by controllers. it sets a log level of 3. 
```


### Error:
```
logging.Error(err,"failed due to this error","key","value")
```

Prints the stack trace where the error was defined for more details. There are no methods for fatal, os.Exit() needs to be called by the caller.

Info:
```
logging.Info("information message","key","value")

logging.V(4).Info("level log4","log_level","4") : prints the log information based on the defined verbosity.
```

