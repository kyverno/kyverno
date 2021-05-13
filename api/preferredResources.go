package data

// k8s version 1.20.2
const PreferredAPIResourceLists = `
[
  {
    "groupVersion": "v1",
    "resources": [
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
        "name": "bindings",
        "singularName": "",
        "namespaced": true,
        "kind": "Binding",
        "verbs": [
          "create"
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "apiregistration.k8s.io/v1beta1",
    "resources": null
  },
  {
    "groupVersion": "apps/v1",
    "resources": [
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
      }
    ]
  },
  {
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
    "groupVersion": "events.k8s.io/v1beta1",
    "resources": null
  },
  {
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
    "groupVersion": "authentication.k8s.io/v1beta1",
    "resources": null
  },
  {
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
        "name": "subjectaccessreviews",
        "singularName": "",
        "namespaced": false,
        "kind": "SubjectAccessReview",
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
      }
    ]
  },
  {
    "groupVersion": "authorization.k8s.io/v1beta1",
    "resources": null
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "autoscaling/v2beta1",
    "resources": null
  },
  {
    "groupVersion": "autoscaling/v2beta2",
    "resources": null
  },
  {
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "certificates.k8s.io/v1beta1",
    "resources": null
  },
  {
    "groupVersion": "networking.k8s.io/v1",
    "resources": [
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
      },
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
      }
    ]
  },
  {
    "groupVersion": "networking.k8s.io/v1beta1",
    "resources": null
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "policy/v1beta1",
    "resources": [
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
      },
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "rbac.authorization.k8s.io/v1beta1",
    "resources": null
  },
  {
    "groupVersion": "storage.k8s.io/v1",
    "resources": [
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
      }
    ]
  },
  {
    "groupVersion": "storage.k8s.io/v1beta1",
    "resources": null
  },
  {
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
    "groupVersion": "admissionregistration.k8s.io/v1beta1",
    "resources": null
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "apiextensions.k8s.io/v1beta1",
    "resources": null
  },
  {
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
    "groupVersion": "scheduling.k8s.io/v1beta1",
    "resources": null
  },
  {
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
    "groupVersion": "coordination.k8s.io/v1beta1",
    "resources": null
  },
  {
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
    "groupVersion": "node.k8s.io/v1beta1",
    "resources": null
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "kyverno.io/v1",
    "resources": [
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
      }
    ]
  },
  {
    "groupVersion": "kyverno.io/v1alpha1",
    "resources": [
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
      },
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "wgpolicyk8s.io/v1alpha1",
    "resources": [
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
      },
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
      }
    ]
  },
  {
    "groupVersion": "networking.istio.io/v1beta1",
    "resources": [
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "notification.toolkit.fluxcd.io/v1beta1",
    "resources": [
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
      }
    ]
  },
  {
    "groupVersion": "security.istio.io/v1beta1",
    "resources": [
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
      }
    ]
  },
  {
    "groupVersion": "source.toolkit.fluxcd.io/v1beta1",
    "resources": [
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
      }
    ]
  },
  {
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
      }
    ]
  },
  {
    "groupVersion": "metrics.k8s.io/v1beta1",
    "resources": [
      {
        "name": "pods",
        "singularName": "",
        "namespaced": true,
        "kind": "PodMetrics",
        "verbs": [
          "get",
          "list"
        ]
      },
      {
        "name": "nodes",
        "singularName": "",
        "namespaced": false,
        "kind": "NodeMetrics",
        "verbs": [
          "get",
          "list"
        ]
      }
    ]
  }
]
`
