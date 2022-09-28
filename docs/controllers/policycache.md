# Policy cache controller

## Attributes

This controller runs on all kyverno instances.

## Purpose

The policy cache controller watches instances of `Policy` and `ClusterPolicy` registered in the cluster and updates the policy cache accordingly.

The policy cache is used at admission time to lookup policies that need to be considered depending on the resource being processed.
