# Integration of Kyverno with Litmus

Kyverno is a policy engine designed for Kubernetes. It can validate, mutate, and generate configurations using admission controls and background scans. Litmus provides a large number of experiments for testing containers, pods, nodes, as well as specific platforms and tools. The advantage of chaos engineering is that one can quickly figure out issues that other testing layers cannot easily capture. This can save a lot of time in the future, and will help to find the loopholes in the system and fix them.


## Steps to Execute LitmusChaos Experiment

### Prerequisites
 * Ensure that Kubernetes Version > 1.15
 * Ensure that the Kyverno is running by executing `kubectl get pods` in operator namespace (typically, `kyverno`). If not, install from [here](https://kyverno.io/docs/installation/).
* Update Kyverno Deployment to use `ghcr.io/kyverno/kyverno:test-litmuschaos` image. Note that this image is built specifically to run Litmuschaos experiments per this request,  [CHAOS_KILL_COMMAND](https://docs.litmuschaos.io/docs/pod-cpu-hog/#prepare-chaosengine). The official Kyverno images will adopt this soon.
 * Ensure that the Litmus Chaos Operator is running by executing `kubectl get pods` in operator namespace (typically, `litmus`). If not, install from [here](https://docs.litmuschaos.io/docs/getstarted/#install-litmus).


### Running experiment
Aftr setting up the docker images, for running a LitmusChaos experiment following steps need to be followed - 
- Firstly, exicute ` eval export E2E="ok" `
- Run the Chaos Experiment Test Command - ` go test ./litmuschaos/pod_cpu_hog -v `.

The test passes if the enforce policy shows it's expected behaviour. 

# Experiments

| Experiment name  | LitmusChaos experiment - Pod CPU Hog |
| :-------------: | ------------- |
| Test command  | ` go test ./litmuschaos/pod_cpu_hog -v ` |
| Goal  | Seeing how the overall application stack behaves when Kyverno pods experience CPU spikes either due to expected/undesired processes  |
| Performed tests |  <li> Deploy enforce policy. </li><li>Run the chaos test to consume CPU resources on the application container. </li><li> Verify the enforce policy behaviour.  </li></li>|
| Expected result  | Kyverno pod is responding after running Pod CPU Hog Experiment |
