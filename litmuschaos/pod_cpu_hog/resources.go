package e2e

// Namespace Description
var LitmusChaosnamespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test-litmus
`)

// Litmus Chaos Service Account
var ChaosServiceAccountYaml = []byte(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-cpu-hog-sa
  namespace: test-litmus
  labels:
    name: pod-cpu-hog-sa
    app.kubernetes.io/part-of: litmus
`)

var ChaosRoleYaml = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-cpu-hog-sa
  namespace: test-litmus
  labels:
    name: pod-cpu-hog-sa
    app.kubernetes.io/part-of: litmus
rules:
- apiGroups: [""]
  resources: ["pods","events"]
  verbs: ["create","list","get","patch","update","delete","deletecollection"]
- apiGroups: [""]
  resources: ["pods/exec","pods/log","replicationcontrollers"]
  verbs: ["create","list","get"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create","list","get","delete","deletecollection"]
- apiGroups: ["apps"]
  resources: ["deployments","statefulsets","daemonsets","replicasets"]
  verbs: ["list","get"]
- apiGroups: ["apps.openshift.io"]
  resources: ["deploymentconfigs"]
  verbs: ["list","get"]
- apiGroups: ["argoproj.io"]
  resources: ["rollouts"]
  verbs: ["list","get"]
- apiGroups: ["litmuschaos.io"]
  resources: ["chaosengines","chaosexperiments","chaosresults"]
  verbs: ["create","list","get","patch","update"]
`)

var ChaosRoleBindingYaml = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pod-cpu-hog-sa
  namespace: test-litmus
  labels:
    name: pod-cpu-hog-sa
    app.kubernetes.io/part-of: litmus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: pod-cpu-hog-sa
subjects:
- kind: ServiceAccount
  name: pod-cpu-hog-sa
  namespace: test-litmus
`)

// Pod CPU Hog Experiment
var PodCPUHogExperimentYaml = []byte(`
apiVersion: litmuschaos.io/v1alpha1
description:
  message: |
    Injects cpu consumption on pods belonging to an app deployment
kind: ChaosExperiment
metadata:
  name: pod-cpu-hog
  labels:
    name: pod-cpu-hog
    app.kubernetes.io/part-of: litmus
    app.kubernetes.io/component: chaosexperiment
    app.kubernetes.io/version: 1.13.3
spec:
  definition:
    scope: Namespaced
    permissions:
      - apiGroups:
          - ""
          - "batch"
          - "apps"
          - "apps.openshift.io"
          - "argoproj.io"
          - "litmuschaos.io"
        resources:
          - "jobs"
          - "pods"
          - "pods/log"
          - "events"
          - "replicationcontrollers"
          - "deployments"
          - "statefulsets"
          - "daemonsets"
          - "replicasets"
          - "deploymentconfigs"
          - "rollouts"
          - "pods/exec"
          - "chaosengines"
          - "chaosexperiments"
          - "chaosresults"
        verbs:
          - "create"
          - "list"
          - "get"
          - "patch"
          - "update"
          - "delete"
          - "deletecollection"
    image: "litmuschaos/go-runner:1.13.3"
    imagePullPolicy: Always
    args:
    - -c
    - ./experiments -name pod-cpu-hog
    command:
    - /bin/bash
    env:
    - name: TOTAL_CHAOS_DURATION
      value: '60'

    ## Number of CPU cores to stress
    - name: CPU_CORES
      value: '1'

    ## Percentage of total pods to target
    - name: PODS_AFFECTED_PERC
      value: ''

    ## Period to wait before and after injection of chaos in sec
    - name: RAMP_TIME
      value: ''

    ## env var that describes the library used to execute the chaos
    ## default: litmus. Supported values: litmus, pumba    
    - name: LIB
      value: 'litmus'

    ## It is used in pumba lib only    
    - name: LIB_IMAGE
      value: 'litmuschaos/go-runner:1.13.3'  

    ## It is used in pumba lib only    
    - name: STRESS_IMAGE
      value: 'alexeiled/stress-ng:latest-ubuntu'  

    # provide the socket file path
    # it is used in pumba lib
    - name: SOCKET_PATH
      value: '/var/run/docker.sock'      

    - name: TARGET_PODS
      value: ''

    ## it defines the sequence of chaos execution for multiple target pods
    ## supported values: serial, parallel
    - name: SEQUENCE
      value: 'parallel'
      
    labels:
      name: pod-cpu-hog
      app.kubernetes.io/part-of: litmus
      app.kubernetes.io/component: experiment-job
      app.kubernetes.io/version: 1.13.3

`)

// ChaosEngine Manifest
var ChaosEngineYaml = []byte(`
apiVersion: litmuschaos.io/v1alpha1
kind: ChaosEngine
metadata:
  name: kind-chaos
  namespace: test-litmus
spec:
  # It can be active/stop
  engineState: 'active'
  appinfo:
    appns: 'kyverno'
    applabel: 'app.kubernetes.io/name=kyverno'
    appkind: 'deployment'
  chaosServiceAccount: pod-cpu-hog-sa
  # It can be delete/retain
  jobCleanUpPolicy: 'delete'
  experiments:
    - name: pod-cpu-hog
      spec:
        components:
          env:
            #number of cpu cores to be consumed
            #verify the resources the app has been launched with
            - name: CPU_CORES
              value: '1'

            - name: TOTAL_CHAOS_DURATION
              value: '60' # in seconds
`)

// install disallow_cri_sock_mount
var DisallowAddingCapabilitiesYaml = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-add-capabilities
  annotations:
    policies.kyverno.io/category: Pod Security Standards (Baseline)
    policies.kyverno.io/severity: medium
    policies.kyverno.io/subject: Pod
    policies.kyverno.io/description: >-
      Capabilities permit privileged actions without giving full root access.
      Adding capabilities beyond the default set must not be allowed.
spec:
  validationFailureAction: enforce
  background: true
  rules:
    - name: capabilities
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: >-
          Adding of additional capabilities beyond the default set is not allowed.
          The fields spec.containers[*].securityContext.capabilities.add and 
          spec.initContainers[*].securityContext.capabilities.add must be empty.
        pattern:
          spec:
            containers:
              - =(securityContext):
                  =(capabilities):
                    X(add): null
            =(initContainers):
              - =(securityContext):
                  =(capabilities):
                    X(add): null

`)

// disallow_cri_sock_mount Resource
var KyvernoTestResourcesYaml = []byte(`
apiVersion: v1
kind: Pod
metadata:
 name: add-new-capabilities
spec:
 containers:
   - name: add-new-capabilities
     image: "ubuntu:18.04"
     command:
       - /bin/sleep
       - "300"
     securityContext:
       capabilities:
         add:
           - NET_ADMIN
`)
