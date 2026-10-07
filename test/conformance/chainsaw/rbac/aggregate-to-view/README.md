## Description

This test verifies that kyverno view cluster roles exist in the cluster and are labelled correctly to be aggregated to the `view` cluster role. It also verifies that no view role exists for `updaterequests`, as these embed the full admission request.
