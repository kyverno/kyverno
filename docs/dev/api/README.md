# Kyverno API

This document provides guidance on extending and maintaining the [Kyverno API](../../../api/kyverno/)

## Concepts
* https://kubernetes.io/docs/reference/using-api/api-concepts/ 
* https://kubernetes.io/docs/reference/using-api/ 
* https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/

## API Groups 

All Kyverno resources are currently defined in the `kyverno.io` API group. 

## API Versions

This `kyverno.io` has the following versions:
* v1apha1 (deprecated) 
* v1apha2 (deprecated) 
* v1beta1 (deprecated)
* v1 
* v2Beta1 
* 

The `v1` version is currently the preferred storage version, but is being deprecated. 

New types should not be added to `v1` but should be introduced as `v2alpha1` and then promoted as they stabilize.

## Stable References

Within the API, newer versions can reference older stable types, but not the other way around. For example, a `v1` resource should not refer to a `v2alpha1` type. However, a `v2alpha1` type can reference `v1` types.



