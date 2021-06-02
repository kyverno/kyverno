package data

// k8s version 1.20.2
const APIResourceLists = `
[
  {
    "kind": "APIResourceList",
    "groupVersion": "v1",
    "resources": [
      {
        "name": "bindings",
        "singularName": "",
        "namespaced": true,
        "kind": "Binding",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "componentstatuses",
        "singularName": "",
        "namespaced": false,
        "kind": "ComponentStatus",
        "verbs": [
          "get",
          "list"
        ],
        "shortNames": [
          "cs"
        ]
      },
      {
        "name": "configmaps",
        "singularName": "",
        "namespaced": true,
        "kind": "ConfigMap",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "cm"
        ],
        "storageVersionHash": "qFsyl6wFWjQ="
      },
      {
        "name": "endpoints",
        "singularName": "",
        "namespaced": true,
        "kind": "Endpoints",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ep"
        ],
        "storageVersionHash": "fWeeMqaN/OA="
      },
      {
        "name": "events",
        "singularName": "",
        "namespaced": true,
        "kind": "Event",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ev"
        ],
        "storageVersionHash": "r2yiGXH7wu8="
      },
      {
        "name": "limitranges",
        "singularName": "",
        "namespaced": true,
        "kind": "LimitRange",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "limits"
        ],
        "storageVersionHash": "EBKMFVe6cwo="
      },
      {
        "name": "namespaces",
        "singularName": "",
        "namespaced": false,
        "kind": "Namespace",
        "verbs": [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ns"
        ],
        "storageVersionHash": "Q3oi5N2YM8M="
      },
      {
        "name": "namespaces/finalize",
        "singularName": "",
        "namespaced": false,
        "kind": "Namespace",
        "verbs": [
          "update"
        ]
      },
      {
        "name": "namespaces/status",
        "singularName": "",
        "namespaced": false,
        "kind": "Namespace",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "nodes",
        "singularName": "",
        "namespaced": false,
        "kind": "Node",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "no"
        ],
        "storageVersionHash": "XwShjMxG9Fs="
      },
      {
        "name": "nodes/proxy",
        "singularName": "",
        "namespaced": false,
        "kind": "NodeProxyOptions",
        "verbs": [
          "create",
          "delete",
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "nodes/status",
        "singularName": "",
        "namespaced": false,
        "kind": "Node",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "persistentvolumeclaims",
        "singularName": "",
        "namespaced": true,
        "kind": "PersistentVolumeClaim",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "pvc"
        ],
        "storageVersionHash": "QWTyNDq0dC4="
      },
      {
        "name": "persistentvolumeclaims/status",
        "singularName": "",
        "namespaced": true,
        "kind": "PersistentVolumeClaim",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "persistentvolumes",
        "singularName": "",
        "namespaced": false,
        "kind": "PersistentVolume",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "pv"
        ],
        "storageVersionHash": "HN/zwEC+JgM="
      },
      {
        "name": "persistentvolumes/status",
        "singularName": "",
        "namespaced": false,
        "kind": "PersistentVolume",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "pods",
        "singularName": "",
        "namespaced": true,
        "kind": "Pod",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "po"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "xPOwRZ+Yhw8="
      },
      {
        "name": "pods/attach",
        "singularName": "",
        "namespaced": true,
        "kind": "PodAttachOptions",
        "verbs": [
          "create",
          "get"
        ]
      },
      {
        "name": "pods/binding",
        "singularName": "",
        "namespaced": true,
        "kind": "Binding",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "pods/eviction",
        "singularName": "",
        "namespaced": true,
        "group": "policy",
        "version": "v1beta1",
        "kind": "Eviction",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "pods/exec",
        "singularName": "",
        "namespaced": true,
        "kind": "PodExecOptions",
        "verbs": [
          "create",
          "get"
        ]
      },
      {
        "name": "pods/log",
        "singularName": "",
        "namespaced": true,
        "kind": "Pod",
        "verbs": [
          "get"
        ]
      },
      {
        "name": "pods/portforward",
        "singularName": "",
        "namespaced": true,
        "kind": "PodPortForwardOptions",
        "verbs": [
          "create",
          "get"
        ]
      },
      {
        "name": "pods/proxy",
        "singularName": "",
        "namespaced": true,
        "kind": "PodProxyOptions",
        "verbs": [
          "create",
          "delete",
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "pods/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Pod",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "podtemplates",
        "singularName": "",
        "namespaced": true,
        "kind": "PodTemplate",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "LIXB2x4IFpk="
      },
      {
        "name": "replicationcontrollers",
        "singularName": "",
        "namespaced": true,
        "kind": "ReplicationController",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "rc"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "Jond2If31h0="
      },
      {
        "name": "replicationcontrollers/scale",
        "singularName": "",
        "namespaced": true,
        "group": "autoscaling",
        "version": "v1",
        "kind": "Scale",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "replicationcontrollers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "ReplicationController",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "resourcequotas",
        "singularName": "",
        "namespaced": true,
        "kind": "ResourceQuota",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "quota"
        ],
        "storageVersionHash": "8uhSgffRX6w="
      },
      {
        "name": "resourcequotas/status",
        "singularName": "",
        "namespaced": true,
        "kind": "ResourceQuota",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "secrets",
        "singularName": "",
        "namespaced": true,
        "kind": "Secret",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "S6u1pOWzb84="
      },
      {
        "name": "serviceaccounts",
        "singularName": "",
        "namespaced": true,
        "kind": "ServiceAccount",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "sa"
        ],
        "storageVersionHash": "pbx9ZvyFpBE="
      },
      {
        "name": "serviceaccounts/token",
        "singularName": "",
        "namespaced": true,
        "group": "authentication.k8s.io",
        "version": "v1",
        "kind": "TokenRequest",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "services",
        "singularName": "",
        "namespaced": true,
        "kind": "Service",
        "verbs": [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "svc"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "0/CO1lhkEBI="
      },
      {
        "name": "services/proxy",
        "singularName": "",
        "namespaced": true,
        "kind": "ServiceProxyOptions",
        "verbs": [
          "create",
          "delete",
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "services/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Service",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]

  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "apiregistration.k8s.io/v1",
    "resources": [
      {
        "name": "apiservices",
        "singularName": "",
        "namespaced": false,
        "kind": "APIService",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "C+s2HXXP47k="
      },
      {
        "name": "apiservices/status",
        "singularName": "",
        "namespaced": false,
        "kind": "APIService",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "apiregistration.k8s.io/v1beta1",
    "resources": [
      {
        "name": "apiservices",
        "singularName": "",
        "namespaced": false,
        "kind": "APIService",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "C+s2HXXP47k="
      },
      {
        "name": "apiservices/status",
        "singularName": "",
        "namespaced": false,
        "kind": "APIService",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "apps/v1",
    "resources": [
      {
        "name": "controllerrevisions",
        "singularName": "",
        "namespaced": true,
        "kind": "ControllerRevision",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "85nkx63pcBU="
      },
      {
        "name": "daemonsets",
        "singularName": "",
        "namespaced": true,
        "kind": "DaemonSet",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ds"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "dd7pWHUlMKQ="
      },
      {
        "name": "daemonsets/status",
        "singularName": "",
        "namespaced": true,
        "kind": "DaemonSet",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "deployments",
        "singularName": "",
        "namespaced": true,
        "kind": "Deployment",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "deploy"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "8aSe+NMegvE="
      },
      {
        "name": "deployments/scale",
        "singularName": "",
        "namespaced": true,
        "group": "autoscaling",
        "version": "v1",
        "kind": "Scale",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "deployments/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Deployment",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "replicasets",
        "singularName": "",
        "namespaced": true,
        "kind": "ReplicaSet",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "rs"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "P1RzHs8/mWQ="
      },
      {
        "name": "replicasets/scale",
        "singularName": "",
        "namespaced": true,
        "group": "autoscaling",
        "version": "v1",
        "kind": "Scale",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "replicasets/status",
        "singularName": "",
        "namespaced": true,
        "kind": "ReplicaSet",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "statefulsets",
        "singularName": "",
        "namespaced": true,
        "kind": "StatefulSet",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "sts"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "H+vl74LkKdo="
      },
      {
        "name": "statefulsets/scale",
        "singularName": "",
        "namespaced": true,
        "group": "autoscaling",
        "version": "v1",
        "kind": "Scale",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "statefulsets/status",
        "singularName": "",
        "namespaced": true,
        "kind": "StatefulSet",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "events.k8s.io/v1",
    "resources": [
      {
        "name": "events",
        "singularName": "",
        "namespaced": true,
        "kind": "Event",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ev"
        ],
        "storageVersionHash": "r2yiGXH7wu8="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "events.k8s.io/v1beta1",
    "resources": [
      {
        "name": "events",
        "singularName": "",
        "namespaced": true,
        "kind": "Event",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ev"
        ],
        "storageVersionHash": "r2yiGXH7wu8="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "authentication.k8s.io/v1",
    "resources": [
      {
        "name": "tokenreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "TokenReview",
        "verbs": [
          "create"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "authentication.k8s.io/v1beta1",
    "resources": [
      {
        "name": "tokenreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "TokenReview",
        "verbs": [
          "create"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "authorization.k8s.io/v1",
    "resources": [
      {
        "name": "localsubjectaccessreviews",
        "singularName": "",
        "namespaced": true,
        "kind": "LocalSubjectAccessReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "selfsubjectaccessreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SelfSubjectAccessReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "selfsubjectrulesreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SelfSubjectRulesReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "subjectaccessreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SubjectAccessReview",
        "verbs": [
          "create"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "authorization.k8s.io/v1beta1",
    "resources": [
      {
        "name": "localsubjectaccessreviews",
        "singularName": "",
        "namespaced": true,
        "kind": "LocalSubjectAccessReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "selfsubjectaccessreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SelfSubjectAccessReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "selfsubjectrulesreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SelfSubjectRulesReview",
        "verbs": [
          "create"
        ]
      },
      {
        "name": "subjectaccessreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SubjectAccessReview",
        "verbs": [
          "create"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "autoscaling/v1",
    "resources": [
      {
        "name": "horizontalpodautoscalers",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "hpa"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "oQlkt7f5j/A="
      },
      {
        "name": "horizontalpodautoscalers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "autoscaling/v2beta1",
    "resources": [
      {
        "name": "horizontalpodautoscalers",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "hpa"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "oQlkt7f5j/A="
      },
      {
        "name": "horizontalpodautoscalers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "autoscaling/v2beta2",
    "resources": [
      {
        "name": "horizontalpodautoscalers",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "hpa"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "oQlkt7f5j/A="
      },
      {
        "name": "horizontalpodautoscalers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HorizontalPodAutoscaler",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "batch/v1",
    "resources": [
      {
        "name": "jobs",
        "singularName": "",
        "namespaced": true,
        "kind": "Job",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "mudhfqk/qZY="
      },
      {
        "name": "jobs/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Job",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "batch/v1beta1",
    "resources": [
      {
        "name": "cronjobs",
        "singularName": "",
        "namespaced": true,
        "kind": "CronJob",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "cj"
        ],
        "categories": [
          "all"
        ],
        "storageVersionHash": "h/JlFAZkyyY="
      },
      {
        "name": "cronjobs/status",
        "singularName": "",
        "namespaced": true,
        "kind": "CronJob",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "certificates.k8s.io/v1",
    "resources": [
      {
        "name": "certificatesigningrequests",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "csr"
        ],
        "storageVersionHash": "UQh3YTCDIf0="
      },
      {
        "name": "certificatesigningrequests/approval",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "certificatesigningrequests/status",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "certificates.k8s.io/v1beta1",
    "resources": [
      {
        "name": "certificatesigningrequests",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "csr"
        ],
        "storageVersionHash": "UQh3YTCDIf0="
      },
      {
        "name": "certificatesigningrequests/approval",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "certificatesigningrequests/status",
        "singularName": "",
        "namespaced": false,
        "kind": "CertificateSigningRequest",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "networking.k8s.io/v1",
    "resources": [
      {
        "name": "ingressclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "IngressClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "6upRfBq0FOI="
      },
      {
        "name": "ingresses",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ing"
        ],
        "storageVersionHash": "ZOAfGflaKd0="
      },
      {
        "name": "ingresses/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "networkpolicies",
        "singularName": "",
        "namespaced": true,
        "kind": "NetworkPolicy",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "netpol"
        ],
        "storageVersionHash": "YpfwF18m1G8="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "networking.k8s.io/v1beta1",
    "resources": [
      {
        "name": "ingressclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "IngressClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "6upRfBq0FOI="
      },
      {
        "name": "ingresses",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ing"
        ],
        "storageVersionHash": "ZOAfGflaKd0="
      },
      {
        "name": "ingresses/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "groupVersion": "extensions/v1beta1",
    "resources": [
      {
        "name": "ingresses",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "ing"
        ],
        "storageVersionHash": "ZOAfGflaKd0="
      },
      {
        "name": "ingresses/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Ingress",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "policy/v1beta1",
    "resources": [
      {
        "name": "poddisruptionbudgets",
        "singularName": "",
        "namespaced": true,
        "kind": "PodDisruptionBudget",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "pdb"
        ],
        "storageVersionHash": "6BGBu0kpHtk="
      },
      {
        "name": "poddisruptionbudgets/status",
        "singularName": "",
        "namespaced": true,
        "kind": "PodDisruptionBudget",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "podsecuritypolicies",
        "singularName": "",
        "namespaced": false,
        "kind": "PodSecurityPolicy",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "psp"
        ],
        "storageVersionHash": "khBLobUXkqA="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "rbac.authorization.k8s.io/v1",
    "resources": [
      {
        "name": "clusterrolebindings",
        "singularName": "",
        "namespaced": false,
        "kind": "ClusterRoleBinding",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "48tpQ8gZHFc="
      },
      {
        "name": "clusterroles",
        "singularName": "",
        "namespaced": false,
        "kind": "ClusterRole",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "bYE5ZWDrJ44="
      },
      {
        "name": "rolebindings",
        "singularName": "",
        "namespaced": true,
        "kind": "RoleBinding",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "eGsCzGH6b1g="
      },
      {
        "name": "roles",
        "singularName": "",
        "namespaced": true,
        "kind": "Role",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "7FuwZcIIItM="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "rbac.authorization.k8s.io/v1beta1",
    "resources": [
      {
        "name": "clusterrolebindings",
        "singularName": "",
        "namespaced": false,
        "kind": "ClusterRoleBinding",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "48tpQ8gZHFc="
      },
      {
        "name": "clusterroles",
        "singularName": "",
        "namespaced": false,
        "kind": "ClusterRole",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "bYE5ZWDrJ44="
      },
      {
        "name": "rolebindings",
        "singularName": "",
        "namespaced": true,
        "kind": "RoleBinding",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "eGsCzGH6b1g="
      },
      {
        "name": "roles",
        "singularName": "",
        "namespaced": true,
        "kind": "Role",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "7FuwZcIIItM="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "storage.k8s.io/v1",
    "resources": [
      {
        "name": "csidrivers",
        "singularName": "",
        "namespaced": false,
        "kind": "CSIDriver",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "Z7aeXSiaYTw="
      },
      {
        "name": "csinodes",
        "singularName": "",
        "namespaced": false,
        "kind": "CSINode",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "Pe62DkZtjuo="
      },
      {
        "name": "storageclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "StorageClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "sc"
        ],
        "storageVersionHash": "K+m6uJwbjGY="
      },
      {
        "name": "volumeattachments",
        "singularName": "",
        "namespaced": false,
        "kind": "VolumeAttachment",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "tJx/ezt6UDU="
      },
      {
        "name": "volumeattachments/status",
        "singularName": "",
        "namespaced": false,
        "kind": "VolumeAttachment",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "storage.k8s.io/v1beta1",
    "resources": [
      {
        "name": "csidrivers",
        "singularName": "",
        "namespaced": false,
        "kind": "CSIDriver",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "Z7aeXSiaYTw="
      },
      {
        "name": "csinodes",
        "singularName": "",
        "namespaced": false,
        "kind": "CSINode",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "Pe62DkZtjuo="
      },
      {
        "name": "storageclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "StorageClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "sc"
        ],
        "storageVersionHash": "K+m6uJwbjGY="
      },
      {
        "name": "volumeattachments",
        "singularName": "",
        "namespaced": false,
        "kind": "VolumeAttachment",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "tJx/ezt6UDU="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "admissionregistration.k8s.io/v1",
    "resources": [
      {
        "name": "mutatingwebhookconfigurations",
        "singularName": "",
        "namespaced": false,
        "kind": "MutatingWebhookConfiguration",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "yxW1cpLtfp8="
      },
      {
        "name": "validatingwebhookconfigurations",
        "singularName": "",
        "namespaced": false,
        "kind": "ValidatingWebhookConfiguration",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "P9NhrezfnWE="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "admissionregistration.k8s.io/v1beta1",
    "resources": [
      {
        "name": "mutatingwebhookconfigurations",
        "singularName": "",
        "namespaced": false,
        "kind": "MutatingWebhookConfiguration",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "yxW1cpLtfp8="
      },
      {
        "name": "validatingwebhookconfigurations",
        "singularName": "",
        "namespaced": false,
        "kind": "ValidatingWebhookConfiguration",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "P9NhrezfnWE="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "apiextensions.k8s.io/v1",
    "resources": [
      {
        "name": "customresourcedefinitions",
        "singularName": "",
        "namespaced": false,
        "kind": "CustomResourceDefinition",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "crd",
          "crds"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "jfWCUB31mvA="
      },
      {
        "name": "customresourcedefinitions/status",
        "singularName": "",
        "namespaced": false,
        "kind": "CustomResourceDefinition",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "apiextensions.k8s.io/v1beta1",
    "resources": [
      {
        "name": "customresourcedefinitions",
        "singularName": "",
        "namespaced": false,
        "kind": "CustomResourceDefinition",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "crd",
          "crds"
        ],
        "categories": [
          "api-extensions"
        ],
        "storageVersionHash": "jfWCUB31mvA="
      },
      {
        "name": "customresourcedefinitions/status",
        "singularName": "",
        "namespaced": false,
        "kind": "CustomResourceDefinition",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "scheduling.k8s.io/v1",
    "resources": [
      {
        "name": "priorityclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "PriorityClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "pc"
        ],
        "storageVersionHash": "1QwjyaZjj3Y="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "scheduling.k8s.io/v1beta1",
    "resources": [
      {
        "name": "priorityclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "PriorityClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "shortNames": [
          "pc"
        ],
        "storageVersionHash": "1QwjyaZjj3Y="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "coordination.k8s.io/v1",
    "resources": [
      {
        "name": "leases",
        "singularName": "",
        "namespaced": true,
        "kind": "Lease",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "/sY7hl8ol1U="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "coordination.k8s.io/v1beta1",
    "resources": [
      {
        "name": "leases",
        "singularName": "",
        "namespaced": true,
        "kind": "Lease",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "/sY7hl8ol1U="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "node.k8s.io/v1",
    "resources": [
      {
        "name": "runtimeclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "RuntimeClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "8nMHWqj34s0="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "node.k8s.io/v1beta1",
    "resources": [
      {
        "name": "runtimeclasses",
        "singularName": "",
        "namespaced": false,
        "kind": "RuntimeClass",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "8nMHWqj34s0="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "discovery.k8s.io/v1beta1",
    "resources": [
      {
        "name": "endpointslices",
        "singularName": "",
        "namespaced": true,
        "kind": "EndpointSlice",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "Nx3SIv6I0mE="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "flowcontrol.apiserver.k8s.io/v1beta1",
    "resources": [
      {
        "name": "flowschemas",
        "singularName": "",
        "namespaced": false,
        "kind": "FlowSchema",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "9bSnTLYweJ0="
      },
      {
        "name": "flowschemas/status",
        "singularName": "",
        "namespaced": false,
        "kind": "FlowSchema",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "prioritylevelconfigurations",
        "singularName": "",
        "namespaced": false,
        "kind": "PriorityLevelConfiguration",
        "verbs": [
          "create",
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "update",
          "watch"
        ],
        "storageVersionHash": "BFVwf8eYnsw="
      },
      {
        "name": "prioritylevelconfigurations/status",
        "singularName": "",
        "namespaced": false,
        "kind": "PriorityLevelConfiguration",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "kyverno.io/v1",
    "resources": [
      {
        "name": "clusterpolicies",
        "singularName": "clusterpolicy",
        "namespaced": false,
        "kind": "ClusterPolicy",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "cpol"
        ],
        "storageVersionHash": "uhKMxCLP2EM="
      },
      {
        "name": "clusterpolicies/status",
        "singularName": "",
        "namespaced": false,
        "kind": "ClusterPolicy",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "policies",
        "singularName": "policy",
        "namespaced": true,
        "kind": "Policy",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "pol"
        ],
        "storageVersionHash": "vgwy0+LsB2g="
      },
      {
        "name": "policies/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Policy",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "generaterequests",
        "singularName": "generaterequest",
        "namespaced": true,
        "kind": "GenerateRequest",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "gr"
        ],
        "storageVersionHash": "TeMup732PSY="
      },
      {
        "name": "generaterequests/status",
        "singularName": "",
        "namespaced": true,
        "kind": "GenerateRequest",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "kyverno.io/v1alpha1",
    "resources": [
      {
        "name": "reportchangerequests",
        "singularName": "reportchangerequest",
        "namespaced": true,
        "kind": "ReportChangeRequest",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "rcr"
        ],
        "storageVersionHash": "vIx0JC9u2UM="
      },
      {
        "name": "clusterreportchangerequests",
        "singularName": "clusterreportchangerequest",
        "namespaced": false,
        "kind": "ClusterReportChangeRequest",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "crcr"
        ],
        "storageVersionHash": "joW3CYySVD4="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "telemetry.istio.io/v1alpha1",
    "resources": [
      {
        "name": "telemetries",
        "singularName": "telemetry",
        "namespaced": true,
        "kind": "Telemetry",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "telemetry"
        ],
        "categories": [
          "istio-io",
          "telemetry-istio-io"
        ],
        "storageVersionHash": "d44J/hig0n8="
      },
      {
        "name": "telemetries/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Telemetry",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "wgpolicyk8s.io/v1alpha1",
    "resources": [
      {
        "name": "clusterpolicyreports",
        "singularName": "clusterpolicyreport",
        "namespaced": false,
        "kind": "ClusterPolicyReport",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "cpolr"
        ],
        "storageVersionHash": "jpUwkNR0RFs="
      },
      {
        "name": "policyreports",
        "singularName": "policyreport",
        "namespaced": true,
        "kind": "PolicyReport",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "polr"
        ],
        "storageVersionHash": "lh+/wBaRsd0="
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "networking.istio.io/v1beta1",
    "resources": [
      {
        "name": "gateways",
        "singularName": "gateway",
        "namespaced": true,
        "kind": "Gateway",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "gw"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "/mjmih7j52A="
      },
      {
        "name": "gateways/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Gateway",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "serviceentries",
        "singularName": "serviceentry",
        "namespaced": true,
        "kind": "ServiceEntry",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "se"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "tY6rFaYqFTs="
      },
      {
        "name": "serviceentries/status",
        "singularName": "",
        "namespaced": true,
        "kind": "ServiceEntry",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "workloadentries",
        "singularName": "workloadentry",
        "namespaced": true,
        "kind": "WorkloadEntry",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "we"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "ZyZMEVHubgI="
      },
      {
        "name": "workloadentries/status",
        "singularName": "",
        "namespaced": true,
        "kind": "WorkloadEntry",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "destinationrules",
        "singularName": "destinationrule",
        "namespaced": true,
        "kind": "DestinationRule",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "dr"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "RTFbwVKZLVo="
      },
      {
        "name": "destinationrules/status",
        "singularName": "",
        "namespaced": true,
        "kind": "DestinationRule",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "sidecars",
        "singularName": "sidecar",
        "namespaced": true,
        "kind": "Sidecar",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "nU5Up7P+Lx0="
      },
      {
        "name": "sidecars/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Sidecar",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "virtualservices",
        "singularName": "virtualservice",
        "namespaced": true,
        "kind": "VirtualService",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "vs"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "OQjiBwfKPL4="
      },
      {
        "name": "virtualservices/status",
        "singularName": "",
        "namespaced": true,
        "kind": "VirtualService",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "networking.istio.io/v1alpha3",
    "resources": [
      {
        "name": "workloadgroups",
        "singularName": "workloadgroup",
        "namespaced": true,
        "kind": "WorkloadGroup",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "wg"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "tPPh8TtfDAQ="
      },
      {
        "name": "workloadgroups/status",
        "singularName": "",
        "namespaced": true,
        "kind": "WorkloadGroup",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "sidecars",
        "singularName": "sidecar",
        "namespaced": true,
        "kind": "Sidecar",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "nU5Up7P+Lx0="
      },
      {
        "name": "sidecars/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Sidecar",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "virtualservices",
        "singularName": "virtualservice",
        "namespaced": true,
        "kind": "VirtualService",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "vs"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "OQjiBwfKPL4="
      },
      {
        "name": "virtualservices/status",
        "singularName": "",
        "namespaced": true,
        "kind": "VirtualService",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "envoyfilters",
        "singularName": "envoyfilter",
        "namespaced": true,
        "kind": "EnvoyFilter",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "cC04AzOKdkE="
      },
      {
        "name": "envoyfilters/status",
        "singularName": "",
        "namespaced": true,
        "kind": "EnvoyFilter",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "gateways",
        "singularName": "gateway",
        "namespaced": true,
        "kind": "Gateway",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "gw"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "/mjmih7j52A="
      },
      {
        "name": "gateways/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Gateway",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "serviceentries",
        "singularName": "serviceentry",
        "namespaced": true,
        "kind": "ServiceEntry",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "se"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "tY6rFaYqFTs="
      },
      {
        "name": "serviceentries/status",
        "singularName": "",
        "namespaced": true,
        "kind": "ServiceEntry",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "workloadentries",
        "singularName": "workloadentry",
        "namespaced": true,
        "kind": "WorkloadEntry",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "we"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "ZyZMEVHubgI="
      },
      {
        "name": "workloadentries/status",
        "singularName": "",
        "namespaced": true,
        "kind": "WorkloadEntry",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "destinationrules",
        "singularName": "destinationrule",
        "namespaced": true,
        "kind": "DestinationRule",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "dr"
        ],
        "categories": [
          "istio-io",
          "networking-istio-io"
        ],
        "storageVersionHash": "RTFbwVKZLVo="
      },
      {
        "name": "destinationrules/status",
        "singularName": "",
        "namespaced": true,
        "kind": "DestinationRule",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "kustomize.toolkit.fluxcd.io/v1beta1",
    "resources": [
      {
        "name": "kustomizations",
        "singularName": "kustomization",
        "namespaced": true,
        "kind": "Kustomization",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "ks"
        ],
        "storageVersionHash": "OUKv7t9A3G4="
      },
      {
        "name": "kustomizations/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Kustomization",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "notification.toolkit.fluxcd.io/v1beta1",
    "resources": [
      {
        "name": "receivers",
        "singularName": "receiver",
        "namespaced": true,
        "kind": "Receiver",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "vYnkEiRiL40="
      },
      {
        "name": "receivers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Receiver",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "providers",
        "singularName": "provider",
        "namespaced": true,
        "kind": "Provider",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "z8f1NmxfWgI="
      },
      {
        "name": "providers/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Provider",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "alerts",
        "singularName": "alert",
        "namespaced": true,
        "kind": "Alert",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "R+8Re3cbWcQ="
      },
      {
        "name": "alerts/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Alert",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "security.istio.io/v1beta1",
    "resources": [
      {
        "name": "peerauthentications",
        "singularName": "peerauthentication",
        "namespaced": true,
        "kind": "PeerAuthentication",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "pa"
        ],
        "categories": [
          "istio-io",
          "security-istio-io"
        ],
        "storageVersionHash": "0IW8zQBF4Qc="
      },
      {
        "name": "peerauthentications/status",
        "singularName": "",
        "namespaced": true,
        "kind": "PeerAuthentication",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "authorizationpolicies",
        "singularName": "authorizationpolicy",
        "namespaced": true,
        "kind": "AuthorizationPolicy",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "categories": [
          "istio-io",
          "security-istio-io"
        ],
        "storageVersionHash": "djwd/cy0e/E="
      },
      {
        "name": "authorizationpolicies/status",
        "singularName": "",
        "namespaced": true,
        "kind": "AuthorizationPolicy",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "requestauthentications",
        "singularName": "requestauthentication",
        "namespaced": true,
        "kind": "RequestAuthentication",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "ra"
        ],
        "categories": [
          "istio-io",
          "security-istio-io"
        ],
        "storageVersionHash": "mr+dytYVwr8="
      },
      {
        "name": "requestauthentications/status",
        "singularName": "",
        "namespaced": true,
        "kind": "RequestAuthentication",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "source.toolkit.fluxcd.io/v1beta1",
    "resources": [
      {
        "name": "helmcharts",
        "singularName": "helmchart",
        "namespaced": true,
        "kind": "HelmChart",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "8ObbA6cw8Ew="
      },
      {
        "name": "helmcharts/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HelmChart",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "buckets",
        "singularName": "bucket",
        "namespaced": true,
        "kind": "Bucket",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "BCjTQxWiRt4="
      },
      {
        "name": "buckets/status",
        "singularName": "",
        "namespaced": true,
        "kind": "Bucket",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "helmrepositories",
        "singularName": "helmrepository",
        "namespaced": true,
        "kind": "HelmRepository",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "eR2pCs+OX/8="
      },
      {
        "name": "helmrepositories/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HelmRepository",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      },
      {
        "name": "gitrepositories",
        "singularName": "gitrepository",
        "namespaced": true,
        "kind": "GitRepository",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "storageVersionHash": "tU3NG5lZ/ZI="
      },
      {
        "name": "gitrepositories/status",
        "singularName": "",
        "namespaced": true,
        "kind": "GitRepository",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "helm.toolkit.fluxcd.io/v2beta1",
    "resources": [
      {
        "name": "helmreleases",
        "singularName": "helmrelease",
        "namespaced": true,
        "kind": "HelmRelease",
        "verbs": [
          "delete",
          "deletecollection",
          "get",
          "list",
          "patch",
          "create",
          "update",
          "watch"
        ],
        "shortNames": [
          "hr"
        ],
        "storageVersionHash": "08YCiPNyiSQ="
      },
      {
        "name": "helmreleases/status",
        "singularName": "",
        "namespaced": true,
        "kind": "HelmRelease",
        "verbs": [
          "get",
          "patch",
          "update"
        ]
      }
    ]
  },
  {
    "kind": "APIResourceList",
    "apiVersion": "v1",
    "groupVersion": "metrics.k8s.io/v1beta1",
    "resources": [
      {
        "name": "nodes",
        "singularName": "",
        "namespaced": false,
        "kind": "NodeMetrics",
        "verbs": [
          "get",
          "list"
        ]
      },
      {
        "name": "pods",
        "singularName": "",
        "namespaced": true,
        "kind": "PodMetrics",
        "verbs": [
          "get",
          "list"
        ]
      }
    ]
  }
]
`
