Use these scripts to prepare the controller for work.
All these scripts should be launched from the root folder of the project, for example:
`scripts/compile-image.sh`

### compile-image.sh ###
Compiles the project to go executable, generates docker image and pushes it to the repo. Has no arguments.

### generate-server-cert.sh ###
Generates TLS certificate and key that used by webhook server. Example:
`scripts/generate-server-cert.sh --service=kube-policy-svc --namespace=my_namespace --serverIp=192.168.10.117`
* `--service` identifies the service for in-cluster webhook server. Do not specify it if you plan to run webhook server outside the cluster.
* `--namespace` identifies the namespace for in-cluster webhook server. Default value is "default".
* `--serverIp` is the IP of master node, it can be found in `~/.kube/config`: clusters.cluster[0].server. **The default is hardcoded value**, so you should explicitly specify it.

### deploy-controller.sh ###
Prepares controller for current environment in 1 of 2 possible modes: free (local) and in-cluster. Usage:
`scripts/deploy-controller.sh --namespace=my_namespace --serverIp=192.168.10.117`
* --namespace identifies the namespace for in-cluster webhook server. Do not specify it if you plan to run webhook server outside the cluster.
* --serverIp is the IP of master node, means the same as for `generate-server-cert.sh`.

### test-web-hook.sh ###
Quickly creates and deletes test config map. If your webhook server is running, you should see the corresponding output from it. Use this script after `deploy-controller.sh`.

### update-codegen.sh ###
Generates additional code for controller object. You should resolve all dependencies before using it, see main Readme for details.
