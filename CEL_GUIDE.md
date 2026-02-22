# Kyverno CEL Library Writing Guide

## Table of Contents

- [Introduction](#introduction)
- [Important Concepts](#important-concepts)
  - [Function](#function)
  - [Overload](#overload)
  - [Receiver](#receiver)
  - [CompileOptions](#compileoptions)
  - [ProgramOptions](#programoptions)
- [General Rules](#general-rules)
  - [Casing](#casing)
  - [Namespacing](#namespacing)
  - [Overload Naming](#overload-naming)
  - [Versioning](#versioning)
- [Library Structural Rules](#library-structural-rules)
  - [Fields](#fields)
  - [Variable Definition](#variable-definition)
  - [Testing](#testing)

## Introduction

Kyverno relies heavily on custom CEL libraries for extending its functionality. Those libraries are written by many maintainers and contributors, hence there should be unifying standard for how those libraries should be written and their behavior.

This document is a summary of those rules and should serve as a guiding principle for both the library author and reviewer. Reviewers of pull requests should nudge the author to make their code comply with this document as much as possible.

As of the time of writing this document, not all the libraries in Kyverno adhere to these rules. There will be an effort from the maintainers to rectify this.

This document assumes Go programming proficiency and a basic understanding of CEL.

## Important Concepts

### Function

A function that can be included in a policy expression by the policy's author. For example, `resource.Get` and `time.now` are both functions. A single function is made known to the CEL compiler through a call to `cel.Function`, which returns a function that implements the `EnvOption` signature, defined as: `type EnvOption func(e *Env) (*Env, error)`. A function in CEL can have one or more overloads, which are passed as the second argument to `cel.Function`.

### Overload

A certain signature for a function. Some functions in CEL can be called with multiple signatures (sets of arguments) and they can all be valid. For example, `resource.List` can be called with 3 strings—apiVersion, kind, and namespace: `resource.List("apps/v1", "deployments", "default")`—or with 3 strings and a map (apiVersion, kind, namespace, label selector): `resource.List("apps/v1", "deployments", "default", {"app": "nginx"})`. An overload can be a `MemberOverload` (a function on a receiver) or an `Overload` (no receiver).

### Receiver

An entity or type on which a function is called, and usually maps to a Go type that contains Go method implementations that carry out the functionality desired by the CEL function. For example, `resource.List` is a function on a receiver called `resource` of the resource context type.

### CompileOptions

`CompileOptions` are a group of modifications that a library makes to an environment during compile time. They are represented by a function that all libraries must implement, for example: `func (c *lib) CompileOptions() []cel.EnvOption`. The env options may be variable definitions, type definitions, and most importantly, function definitions for the functions this library introduces.

### ProgramOptions

`ProgramOptions` are a group of modifications that a library makes to an environment during evaluation time. The prevalent example in Kyverno is libraries passing the evaluation-time values for variables previously defined in `CompileOptions`. This says, during evaluation you should use this Go HTTP client as the type to make requests with.

## General Rules

### Casing

All functions should follow a camel case syntax to follow the same convention as Kubernetes and have expressions that leverage k8s and Kyverno functions look congruent.

For example:

- `something.SomeFunctionCall`, `something.some_function_call`: invalid
- `something.someFunctionCall`: valid

### Namespacing

All functions must be namespaced. This is guaranteed for functions that have a receiver (methods). However, for functions that don't have a receiver, they must be registered in the CEL environment with a namespace followed by a dot to identify what library they belong to.

For example:

- `now()`: invalid
- `time.now()`: valid

The CEL compiler is able to resolve such functions correctly without mistaking it for a member call, i.e., interpret `time.now` as the full function name rather than `now` on a member called `time`.

### Overload Naming

Overload names are not visible to policy authors and can be considered an implementation detail, but we define a practice for them for general hygiene.

The name of the overload is the first argument passed to `cel.Overload`. The naming should be `funcname_arg1_arg2_arg3...`. For example, the resource library's list with no label selector overload should be `list_string_string_string`.

### Versioning

There are two versioning schemes going on inside the CEL space of Kyverno: the compiler versioning and the library versioning. A compiler of version x is using library y of version z, and so on for all libraries. Compiler versions get bumped when a Kyverno release is made that contains CEL changes. Any breaking change to a library constitutes a version bump in the library and of the compiler in the next release. The rollout should follow a deprecation plan (change introduced in a release, previous state deprecated in the next release, and deleted in the third). The types of breaking changes are:

- Renaming or removal of a function
- Removal of an overload
- Renaming or removal of a namespace
- Removal or renaming of a field from a struct with defined fields

Non-breaking changes require a compiler and library bump but without a versioning plan. Non-breaking changes are:

- New functions
- New overloads

## Library Structural Rules

### Fields

The library struct MUST have a field for version. If the library introduces an opaque type (a type whose Go representation is an interface), there needs to be a field that carries an interface in the library struct. The library struct can have any extra needed fields to carry out its task.

### Variable Definition

The library needs to define its compile-time variables in `CompileOptions` and must not outsource this definition to another entity. At eval time, the library needs to pass the value of its interface field as the concrete value for that variable. Here's an example from the HTTP library:

```go
type lib struct {
	httpIface ContextInterface // contains the definitions for Get, Post, etc.
	version   *version.Version
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("http", ContextType),
		...
	}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Globals(
			map[string]any{
				"http": l.httpIface,
			},
		),
	}
}
```

### Testing

Libraries must define an `impl_test` or `lib_test` file that tests the compiler compiling and evaluating an expression from the library. An additional Chainsaw test containing a policy that uses an expression containing a function from the library is optional but preferred.
