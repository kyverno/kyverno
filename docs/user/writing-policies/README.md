# Writing Policies

This guide covers how to write policies in Kyverno.

> **Note:** Kyverno supports both traditional YAML-based policies and CEL-based policies.  
> As of Kyverno v1.17, policy authoring is transitioning toward CEL-based policies, and some legacy constructs (such as certain ClusterPolicy usage patterns) are being deprecated.  
> Traditional YAML-based policies may still be encountered in existing clusters, while CEL-based policies are recommended for new and more complex validation logic.

