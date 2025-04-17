# Feature Flags Guidelines
## Introduction
Feature flags, also known as feature toggles, are a software development technique that allows developers to turn features on and off in a production environment without requiring a new deployment.

There are several ways to enable/disable features in Golang:
1. Feature Toggles: It provides a simple way to enable/disable features based on environment variables and/or command line arguments
2. Container arguments

## Feature Flags
The `toggle` package exists in pkg/toggle and can be used to define and manage a feature toggle for a new feature in Kyverno. Let's say we need to introduce a new feature toggle to enable/disable deferred loading. We can do it as follows:
1. Define a flag name, description, environment variable, and a default value for this new feature in the `toggle` package:
    ```
    EnableDeferredLoadingFlagName    = "enableDeferredLoading"
    EnableDeferredLoadingDescription = "enable deferred loading of context variables"
    enableDeferredLoadingEnvVar      = "FLAG_ENABLE_DEFERRED_LOADING"
    defaultEnableDeferredLoading     = true
    ``` 
2. Create a new toggle for the new feature using the `newToggle` method that takes both default value and environment variable as arguments:
    ```
    EnableDeferredLoading    = newToggle(defaultEnableDeferredLoading, enableDeferredLoadingEnvVar)
    ```
    
    At this point, we have an instance of `toggle` which will be used later to call `toggle.enabled()` of the feature toggle to execute code conditionally. 

3. Add a new method `EnableDeferredLoading() bool` in `Toggles` Interface at pkg/toggle/context.go to call the `enabled` method. It will be used later in Kyverno controllers:
   ```
   type Toggles interface {
       EnableDeferredLoading() bool
   }

   func (defaultToggles) EnableDeferredLoading() bool {
	   return EnableDeferredLoading.enabled()
   }
   ```

4. In the controller, we can use it as follows:
   
   ```
   flag.Func(toggle.EnableDeferredLoadingFlagName, toggle.EnableDeferredLoadingDescription, toggle.EnableDeferredLoading.Parse)
   ```

### Advantages
1. Feature toggles can be accessed globally. Its value can be checked anywhere in the code; there is no need to pass it as an argument among methods/functions.

2. Users can either enable or disable the feature by setting it as an argument to the container `--enableDeferredLoading=false` or setting its
environment variable `FLAG_ENABLE_DEFERRED_LOADING=0`

## Container Arguments
Container arguments can be used directly in the controller. Let's say we want to add a new container flag `--enable-feature`, we can do it as follows:
1. Create a variable for this new flag:
```
var(
    enableFeature bool
)
```

2. Define a bool flag with a specified name, default value, and usage:
```
flagset.BoolVar(&enableFeature, "enable-feature", true, "Set this flag to 'false' to ....")
```
