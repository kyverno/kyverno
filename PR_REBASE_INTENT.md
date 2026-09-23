# Rebase Legacy Policy PRs to release-1.19

## Background

Kyverno v1.19 is the last release that supports ClusterPolicy, Policy, and CleanupPolicy types.

We want to remove all legacy policy code from `main` but retain the CRDs till v1.21 to detect old types and allow a safe rollback.

Once we remove the legacy engine and code, all open PRs (<https://github.com/kyverno/kyverno/pulls>) that reference the old code will show a conflict.

Hence, we need to rebase them to v1.19 and merge them there.

## Workflow

1. For each open PR, check if it makes changes to legacy policy type code, new CEL policy type code, or both:
   1. If it is only for legacy policy type code, merge to release-1.19
   2. If it is only for new CEL policy type, keep the merge to main
   3. If it's for both, either split to two PRs or keep to main

## New PRs

We will need some way to check new PRs and make sure they are against the right branch.
