package fuzz

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type BypassChecker struct {
	ResourceType  string
	ShouldBlock   func(*corev1.Pod) (bool, error)
	ClusterPolicy *kyvernov1.ClusterPolicy
}

var (
	cp1  *kyvernov1.ClusterPolicy
	cp2  *kyvernov1.ClusterPolicy
	cp3  *kyvernov1.ClusterPolicy
	cp4  *kyvernov1.ClusterPolicy
	cp5  *kyvernov1.ClusterPolicy
	cp6  *kyvernov1.ClusterPolicy
	cp7  *kyvernov1.ClusterPolicy
	cp8  *kyvernov1.ClusterPolicy
	cp9  *kyvernov1.ClusterPolicy
	cp10 *kyvernov1.ClusterPolicy
	cp11 *kyvernov1.ClusterPolicy

	mi2048 resource.Quantity

	Policies map[int]*BypassChecker

	k8sKinds = map[int]string{
		0:  "Config",
		1:  "ConfigMap",
		2:  "CronJob",
		3:  "DaemonSet",
		4:  "Deployment",
		5:  "EndpointSlice",
		6:  "Ingress",
		7:  "Job",
		8:  "LimitRange",
		9:  "List",
		10: "NetworkPolicy",
		11: "PersistentVolume",
		12: "PersistentVolumeClaim",
		13: "Pod",
		14: "ReplicaSet",
		15: "ReplicationController",
		16: "RuntimeClass",
		17: "Secret",
		18: "Service",
		19: "StorageClass",
		20: "VolumeSnapshot",
		21: "VolumeSnapshotClass",
		22: "VolumeSnapshotContent",
	}

	kindToVersion = map[string]string{
		"Config":                "v1",
		"ConfigMap":             "v1",
		"CronJob":               "batch/v1",
		"DaemonSet":             "apps/v1",
		"Deployment":            "apps/v1",
		"EndpointSlice":         "discovery.k8s.io/v1",
		"Ingress":               "networking.k8s.io/v1",
		"Job":                   "batch/v1",
		"LimitRange":            "v1",
		"List":                  "v1",
		"NetworkPolicy":         "networking.k8s.io/v1",
		"PersistentVolume":      "v1",
		"PersistentVolumeClaim": "v1",
		"Pod":                   "v1",
		"ReplicaSet":            "apps/v1",
		"ReplicationController": "v1",
		"RuntimeClass":          "node.k8s.io/v1",
		"Secret":                "v1",
		"Service":               "v1",
		"StorageClass":          "storage.k8s.io/v1",
		"VolumeSnapshot":        "snapshot.storage.k8s.io/v1",
		"VolumeSnapshotClass":   "snapshot.storage.k8s.io/v1",
		"VolumeSnapshotContent": "snapshot.storage.k8s.io/v1",
	}

	LatestImageTagPolicy = []byte(`{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-image"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-tag",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "An image tag is required",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"image": "*:*"
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "validate-latest",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "If the image has 'latest' tag then imagePullPolicy must be 'Always'",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"(image)": "*latest",
								"imagePullPolicy": "Always"
							 }
						  ]
					   }
					}
				 }
			  }
		   ]
		}
	 }
	`)

	EqualityHostpathPolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-host-path"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-host-path",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "Host path '/var/lib/' is not allowed",
					"pattern": {
					   "spec": {
						  "volumes": [
							 {
								"=(hostPath)": {
								   "path": "!/var/lib"
								}
							 }
						  ]
					   }
					}
				 }
			  }
		   ]
		}
	 }
	 `)
	SecurityContextPolicy = []byte(`{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						  "=(securityContext)": {
							 "runAsNonRoot": true
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }`)

	ContainerNamePolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "fuzzPolicy"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"=(containers)": [
					  {
						"name": "nginx"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	PodExistencePolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-secaas-k8s"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"^(containers)": [
					  {
						"name": "nginx"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	  }
		 `)

	HostPathCannotExistPolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "validate-host-path"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate-host-path",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"message": "Host path is not allowed",
				"pattern": {
				  "spec": {
					"volumes": [
					  {
						"name": "*",
						"X(hostPath)": null
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	  }
	 `)
	NamespaceCannotBeEmptyOrDefaultPolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-namespace"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "check-default-namespace",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "A namespace is required",
					"anyPattern": [
					   {
						  "metadata": {
							 "namespace": "?*"
						  }
					   },
					   {
						  "metadata": {
							 "namespace": "!default"
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }
	`)

	HostnetworkAndPortNotAllowedPolicy = []byte(`
	{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-host-network-port"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-host-network-port",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "Host network and port are not allowed",
					"pattern": {
					   "spec": {
						  "hostNetwork": false,
						  "containers": [
							 {
								"name": "*",
								"ports": [
								   {
									  "hostPort": null
								   }
								]
							 }
						  ]
					   }
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	SupplementalGroupsShouldBeHigherThanZeroPolicy = []byte(`{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						  "=(supplementalGroups)": ">0"
					   }
					}
				 }
			  }
		   ]
		}
	 }	 `)

	SupplementalGroupsShouldBeBetween = []byte(`{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						"=(supplementalGroups)": [
							">0 & <100001"
						  ]
					   }
					}
				 }
			  }
		   ]
		}
	 }	 `)

	ShouldHaveMoreMemoryThanFirstContainer = []byte(`{
		"apiVersion": "kyvernov1.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
				{
					"name": "validate-host-network-port",
					"match": {
						"resources": {
					   		"kinds": [
						  		"Pod"
					   		]
						}
				 	},
				 	"validate": {
						"message": "Host network and port are not allowed",
						"pattern": {
					   		"spec":{
								"containers":[
									{
										"name":"*",
										"resources":{
											"requests":{
												"memory":"$(<=/spec/containers/0/resources/limits/memory)"
											},
											"limits":{
												"memory":"2048Mi"
											}
										}
									}
								]
							}
						}
				 	}
			  	}
		   	]
		}
	}	 `)
)

func InitFuzz() {
	mi2048, _ = resource.ParseQuantity("2048Mi")
	cp1 = &kyvernov1.ClusterPolicy{}
	err := json.Unmarshal(ContainerNamePolicy, cp1)
	if err != nil {
		panic(err)
	}
	bpc1 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockContainerName,
		ClusterPolicy: cp1,
	}
	cp2 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(LatestImageTagPolicy, cp2)
	if err != nil {
		panic(err)
	}
	bpc2 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockImageTag,
		ClusterPolicy: cp2,
	}
	cp3 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(SecurityContextPolicy, cp3)
	if err != nil {
		panic(err)
	}
	bpc3 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockSecurityPolicy,
		ClusterPolicy: cp3,
	}
	cp4 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(EqualityHostpathPolicy, cp4)
	if err != nil {
		panic(err)
	}
	bpc4 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockEquality,
		ClusterPolicy: cp4,
	}
	cp5 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(PodExistencePolicy, cp5)
	if err != nil {
		panic(err)
	}
	bpc5 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockContainerNameExistenceAnchor,
		ClusterPolicy: cp5,
	}
	cp6 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(HostPathCannotExistPolicy, cp6)
	if err != nil {
		panic(err)
	}
	bpc6 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfHostPathExists,
		ClusterPolicy: cp6,
	}
	cp7 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(NamespaceCannotBeEmptyOrDefaultPolicy, cp7)
	if err != nil {
		panic(err)
	}
	bpc7 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfNamespaceIsEmptyOrDefault,
		ClusterPolicy: cp7,
	}
	cp8 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(HostnetworkAndPortNotAllowedPolicy, cp8)
	if err != nil {
		panic(err)
	}
	bpc8 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfHostnetworkOrPortAreSpecified,
		ClusterPolicy: cp8,
	}
	cp9 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(SupplementalGroupsShouldBeHigherThanZeroPolicy, cp9)
	if err != nil {
		panic(err)
	}
	bpc9 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfSupplementalGroupsExistAndAreLessThanZero,
		ClusterPolicy: cp9,
	}
	cp10 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(SupplementalGroupsShouldBeBetween, cp10)
	if err != nil {
		panic(err)
	}
	bpc10 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfSupplementalGroupsExistAndIsNotBetween,
		ClusterPolicy: cp10,
	}
	cp11 = &kyvernov1.ClusterPolicy{}
	err = json.Unmarshal(ShouldHaveMoreMemoryThanFirstContainer, cp11)
	if err != nil {
		panic(err)
	}
	bpc11 := &BypassChecker{
		ResourceType:  "Pod",
		ShouldBlock:   ShouldBlockIfLessMemoryThanFirstContainer,
		ClusterPolicy: cp11,
	}

	Policies = make(map[int]*BypassChecker)
	Policies[0] = bpc1
	Policies[1] = bpc2
	Policies[2] = bpc3
	Policies[3] = bpc4
	Policies[4] = bpc5
	Policies[5] = bpc6
	Policies[6] = bpc7
	Policies[7] = bpc8
	Policies[8] = bpc9
	Policies[9] = bpc10
	Policies[10] = bpc11
}

func ShouldBlockIfLessMemoryThanFirstContainer(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("No containers found")
	}
	containers := pod.Spec.Containers

	if len(containers) < 2 {
		return false, nil
	}

	var container0MemLimit resource.Quantity
	container0 := containers[0]

	fieldName := "Resources"
	value := reflect.ValueOf(container0)
	field := value.FieldByName(fieldName)

	if !field.IsValid() {
		// field is not specied, so fail
		return true, nil
	}

	fieldName = "Limits"
	value = reflect.ValueOf(container0.Resources)
	field = value.FieldByName(fieldName)

	if !field.IsValid() {
		// field is not specied, so fail
		return true, nil
	}

	if limit, ok := container0.Resources.Limits[corev1.ResourceMemory]; ok {
		container0MemLimit = limit
	}

	for i, container := range containers {
		if i > 0 {
			fieldName := "Resources"
			value := reflect.ValueOf(container)
			field := value.FieldByName(fieldName)

			if !field.IsValid() {
				// field is not specied, so fail
				return true, nil
			}

			fieldName = "Limits"
			value = reflect.ValueOf(container.Resources)
			field = value.FieldByName(fieldName)

			if !field.IsValid() {
				// field is not specied, so fail
				return true, nil
			}

			if limit, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				if !limit.Equal(mi2048) {
					return true, nil
				}
			} else {
				return true, nil
			}

			fieldName = "Requests"
			value = reflect.ValueOf(container.Resources)
			field = value.FieldByName(fieldName)

			if !field.IsValid() {
				// field is not specied, so fail
				return true, nil
			}

			if limit, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				smallerThanOrEqual := limit.Cmp(container0MemLimit)
				if smallerThanOrEqual == -1 || smallerThanOrEqual == 0 {
					return true, nil
				}
			} else {
				return false, nil
			}
		}
	}
	return false, nil
}

func ShouldBlockIfSupplementalGroupsExistAndIsNotBetween(pod *corev1.Pod) (bool, error) {
	if pod.Spec.SecurityContext != nil {
		if len(pod.Spec.SecurityContext.SupplementalGroups) != 0 {
			for _, sg := range pod.Spec.SecurityContext.SupplementalGroups {
				if sg <= 0 || sg >= 100001 {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func ShouldBlockIfSupplementalGroupsExistAndAreLessThanZero(pod *corev1.Pod) (bool, error) {
	if pod.Spec.SecurityContext != nil {
		if len(pod.Spec.SecurityContext.SupplementalGroups) != 0 {
			for _, sg := range pod.Spec.SecurityContext.SupplementalGroups {
				if sg <= 0 {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func ShouldBlockIfHostnetworkOrPortAreSpecified(pod *corev1.Pod) (bool, error) {
	if pod.Spec.SecurityContext != nil {
		fieldName := "HostNetwork"
		value := reflect.ValueOf(pod.Spec.SecurityContext)
		field := value.Elem().FieldByName(fieldName)
		if field.IsValid() {
			// field is specified but cannot be according to the policy
			return true, nil
		}
	}

	if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("No containers found")
	}
	containers := pod.Spec.Containers

	for _, container := range containers {
		for _, port := range container.Ports {
			fieldName := "HostPort"
			value := reflect.ValueOf(port)
			field := value.FieldByName(fieldName)

			if field.IsValid() {
				// field is specified but cannot be according to the policy
				return true, nil
			}
		}
	}
	return false, nil
}

func ShouldBlockIfNamespaceIsEmptyOrDefault(pod *corev1.Pod) (bool, error) {
	if len(pod.ObjectMeta.Namespace) == 0 {
		return true, nil
	}

	if pod.ObjectMeta.Namespace == "default" {
		return true, nil
	}
	return false, nil
}

func ShouldBlockContainerName(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("No containers found")
	}
	containers := pod.Spec.Containers

	for _, container := range containers {
		if container.Name != "nginx" {
			return true, nil
		}
	}
	return false, nil
}

func ShouldBlockContainerNameExistenceAnchor(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("No containers found")
	}
	containers := pod.Spec.Containers

	for _, container := range containers {
		if container.Name == "nginx" {
			return false, nil
		}
	}
	return true, nil
}

func ShouldBlockImageTag(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("No containers found")
	}
	containers := pod.Spec.Containers

	for _, container := range containers {
		split := strings.Split(container.Image, ":")
		if len(split) != 2 {
			return true, nil
		}
		if _, ok := strings.CutSuffix(container.Image, "latest"); ok {
			if container.ImagePullPolicy != "Always" {
				return true, nil
			}
		}
	}
	return false, nil
}

func ShouldBlockEquality(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Volumes == nil || len(pod.Spec.Volumes) == 0 {
		return false, fmt.Errorf("No volumes found")
	}
	volumes := pod.Spec.Volumes

	for _, volume := range volumes {
		if volume.VolumeSource.HostPath != nil {
			if volume.VolumeSource.HostPath.Path == "/var/lib" {
				return true, nil
			}
		}
	}
	return false, nil
}

func ShouldBlockIfHostPathExists(pod *corev1.Pod) (bool, error) {
	if pod.Spec.Volumes == nil || len(pod.Spec.Volumes) == 0 {
		return false, fmt.Errorf("No volumes found")
	}
	volumes := pod.Spec.Volumes

	for _, volume := range volumes {
		if volume.VolumeSource.HostPath != nil {
			return true, nil
		}
	}
	return false, nil
}

// if there is a security policy, then RunAsNonRoot must be true
func ShouldBlockSecurityPolicy(pod *corev1.Pod) (bool, error) {
	if pod.Spec.SecurityContext == nil {
		return false, nil
	}

	securityContext := pod.Spec.SecurityContext

	if securityContext.RunAsNonRoot == nil {
		return true, nil
	}

	if !*securityContext.RunAsNonRoot {
		return true, nil
	}

	return false, nil
}
