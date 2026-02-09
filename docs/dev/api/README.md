# Kyverno API

This document provides guidance on extending and maintaining the [Kyverno API](../../../api/kyverno/)

## Concepts
* https://kubernetes.io/docs/reference/using-api/api-concepts/ 
* https://kubernetes.io/docs/reference/using-api/ 
* https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/

## API Groups 

Kyverno uses multiple API groups:
- `kyverno.io` - Main Kyverno policies and resources
- `policies.kyverno.io` - CEL-based policies 
- `policyreport.io` - Policy reports
- `reports.kyverno.io` - Kyverno-specific reports

## API Versions

The `kyverno.io` API group has the following versions:
* v1beta1 
* v1 
* v2alpha1
* v2beta1

The `policies.kyverno.io` API group has:
* v1alpha1

The `policyreport.io` API group has:
* v1alpha2

The `reports.kyverno.io` API group has:
* v1 

The `v1` version in the `kyverno.io` API group is currently the preferred storage version, but is being deprecated. 

The goal is to eventually make v2 the preferred version and remove all v1* versions from the main `kyverno.io` API group.


## Adding a new Kind (Resource)

New types should not be added to `v1` in the `kyverno.io` API group but should be introduced as `v2alpha1` and then promoted as they stabilize. For CEL-based policies, new types should be added to the `policies.kyverno.io` API group.

## Adding a new attribute

New attributes can be added to existing resources without impacting compatibility. They do not require a new version.

## Deleting an attribute

Attributes cannot be deleted in a version. They should be marked for deprecation and removed after 3 minor releases.

## Modifying an attribute

Attributes cannot be modified in a version. The existing attribute should be marked for deprecation and a new attribute should be added following version compatibility guidelines.


## Stable References

Within the API, newer versions can reference older stable types, but not the other way around. For example, a `v1` resource should not refer to a `v2alpha1` type. However, a `v2alpha1` type can reference `v1` types.



