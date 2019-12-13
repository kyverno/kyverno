# Disallow Docker socket bind mount

The Docker socket bind mount allows access to the 
Docker daemon on the node. This access can be used for privilege escalation and 
to manage containers outside of Kubernetes, and hence should not be allowed.  

## Policy YAML 

[disallow_docker_sock_mount.yaml](best_practices/disallow_docker_sock_mount.yaml) 

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-docker-sock-mount
spec:
  rules:
  - name: validate-docker-sock-mount
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Use of the Docker Unix socket is not allowed"
      pattern:
        spec:
          =(volumes):
            =(hostPath):
              path: "!/var/run/docker.sock"
````
