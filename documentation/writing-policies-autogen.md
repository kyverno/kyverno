<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Auto-Generation for Pod Controllers*</small>

# Auto Generating Rules for Pod Controllers

Writing policies on pods helps address all pod creation flows. However, when pod cotrollers are used pod level policies result in errors not being reported when the pod controller object is created. 

Kyverno solves this issue by supporting automatic generation of policy rules for pod controllers from a rule written for a pod.

This auto-generation behavior is controlled by the `pod-policies.kyverno.io/autogen-controllers` annotation. 

By default, Kyverno inserts an annotation `pod-policies.kyverno.io/autogen-controllers=all`, to generate an additional rule that is applied to pod controllers: DaemonSet, Deployment, Job, StatefulSet.
 
You can change the annotation `pod-policies.kyverno.io/autogen-controllers` to customize the target pod controllers for the auto-generated rules. For example, Kyverno generates a rule for a `Deployment` if the annotation of policy is defined as `pod-policies.kyverno.io/autogen-controllers=Deployment`. 

When a `name` or `labelSelector` is specified in the match / exclude block, Kyverno skips generating pod controllers rule as these filters may not be applicable to pod controllers.
 
To disable auto-generating rules for pod controllers set `pod-policies.kyverno.io/autogen-controllers`  to the value `none`.

<small>*Read Next >> [Background Processing](/documentation/writing-policies-background.md)*</small>

