# Integration of Kyverno with Litmus

Kyverno is a policy engine designed for Kubernetes. It can validate, mutate, and generate configurations using admission controls and background scans. Litmus provides a large number of experiments for testing containers, pods, nodes, as well as specific platforms and tools. The advantage of chaos engineering is that one can quickly figure out issues that other testing layers cannot easily capture. This can save a lot of time in the future, and will help to find the loopholes in the system and fix them.


## Steps to Execute LitmusChaos Experiment

### Prerequisites
- At first, ensure that the Kyverno is running by executing `kubectl get pods` in operator namespace.If not, install from [here](https://kyverno.io/docs/installation/)
- Install Litmus Chaos operator using `make install-litmus-chaos`. 
- We will change the base image soon so that the Litmuschaos tests can be run against the official images. For that, in [Dockerfile](https://github.com/kyverno/kyverno/blob/main/cmd/kyverno/Dockerfile) and [localDockerfile](https://github.com/kyverno/kyverno/blob/5dfd16ce44131c05c3867409f1edf9953e7b45c0/cmd/kyverno/localDockerfile) change `scratch` to `alpine` and execute both commands - `make docker-build-all-amd64` and `make docker-build-local-kyverno`. 
- Pull the Docker image with test-litmuschaos tag  ` docker pull ghcr.io/kyverno/kyverno:test-litmuschaos `.
- Restart the Kyverno pod so that new changes can be applied using `kubectl -n kyverno delete pod --all `.


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
