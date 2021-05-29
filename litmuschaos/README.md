# Integration of Kyverno with Litmus

Kyverno is a policy engine designed for Kubernetes. It can validate, mutate, and generate configurations using admission controls and background scans. Litmus provides a large number of experiments for testing containers, pods, nodes, as well as specific platforms and tools. The advantage of chaos engineering is that one can quickly figure out issues that other testing layers cannot easily capture. This can save a lot of time in the future, and will help to find the loopholes in the system and fix them.

# Experiments

| Experiment name  | Integrate LitmusChaos experiment - Pod CPU Hog |
| ------------- | ------------- |
| Test command  | `go test ./litmuschaos/pod_cpu_hog -v` |
| Goal  | Seeing how the overall application stack behaves when Kyverno pods experience CPU spikes either due to expected/undesired processes  |
| Expected result  | Kyverno is responding after running CPU Hog  |