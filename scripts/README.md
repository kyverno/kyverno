Use these scripts to prepare the controller for work.
All these scripts should be launched from the root folder of the project, for example:
`scripts/compile-image.sh`

### compile-image.sh ###
Compiles the project to go executable, generates docker image and pushes it to the repo. Has no arguments.

### generate-server-cert.sh ###
Generates TLS certificate and key that used by webhook server. Example:
`scripts/generate-server-cert.sh --service=kube-policy-svc --namespace=my_namespace --serverIp=192.168.10.117`
* `--service` identifies the service for in-cluster webhook server. Do not specify it if you plan to run webhook server outside the cluster, or cpecify 'localhost' if you want to run controller locally.
* `--namespace` identifies the namespace for in-cluster webhook server. Do not specify it if you plan to run controller locally.
* `--serverIp` is the IP of master node, it can be found in `~/.kube/config`: clusters.cluster[0].server. You should explicitly specify it.

### deploy-controller.sh ###
Prepares controller for free (local) or in-cluster use. Uses `generate-server-cert.sh` inside and has the same parameters with almost same meaning:
* `--service` - the name of the service which will be created for the controller. Use 'localhost' value to deploy controller locally. The default is 'kube-policu-svc'
* `--namespace` - the target namespace to deploy the controller. Do not specify it if you want to depoloy controller locally.
* `--serverIp` means the same as for `generate-server-cert.sh`
Examples:
`scripts/deploy-controller.sh --service=my-kube-policy --namespace=my_namespace --serverIp=192.168.10.117` - deploy controller to the cluster with master node '192.168.10.117' to the namespace 'my_namespace' as a service 'my-kube-policy'
`scripts/deploy-controller.sh --service=localhost --serverIp=192.168.10.117` - deploy controller locally for usage in cluster with mnaster node at '192.168.10.117'

### test-web-hook.sh ###
Quickly creates and deletes test config map. If your webhook server is running, you should see the corresponding output from it. Use this script after `deploy-controller.sh`.

### update-codegen.sh ###
Generates additional code for controller object. You should resolve all dependencies before using it, see main Readme for details.
