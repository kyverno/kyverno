## Description

This test creates a policy to deny the creation of ephemeral containers.
The policy is targeting `Pod/ephemeralcontainers` and calls `kubectl debug`, the call is expected to fail.
