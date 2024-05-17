package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig           string
	namespace            string
	kinds                string
	clientRateLimitBurst int
	clientRateLimitQPS   float64
	replicas             int
	count                int
	delete               bool
)

func main() {
	var burst int = 100
	var qps float64 = 100
	flagset := flag.NewFlagSet("perf-testing", flag.ExitOnError)
	flagset.StringVar(&kubeconfig, "kubeconfig", "/root/.kube/config", "Path to a kubeconfig. Only required if out-of-cluster.")
	flagset.StringVar(&namespace, "namespace", "test", "Namespace to create the resource")
	flagset.StringVar(&kinds, "kinds", "", "comma separated string which takes resource kinds to be created")
	flagset.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", qps, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flagset.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", burst, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flagset.IntVar(&replicas, "replicas", 50, "Configure the replica number of the resource to be created")
	flagset.IntVar(&count, "count", 50, "Configure the total number of the resource to be created")
	flagset.BoolVar(&delete, "delete", false, "clean up resources")

	flagset.VisitAll(func(f *flag.Flag) {
		flag.CommandLine.Var(f.Value, f.Name, f.Usage)
	})
	flag.Parse()

	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("error creating client config: ", err)
		os.Exit(1)
	}

	clientConfig.Burst = clientRateLimitBurst
	clientConfig.QPS = float32(clientRateLimitQPS)
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		fmt.Println("error creating client set: ", err)
		os.Exit(1)
	}

	resourceKinds := strings.Split(kinds, ",")
	for _, kind := range resourceKinds {
		switch kind {
		case "pods":
			if delete {
				if err := client.CoreV1().Pods(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					fmt.Println("failed to delete the collection of pods: ", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			var wg sync.WaitGroup
			for i := 0; i < count; i++ {
				num := strconv.Itoa(i)
				wg.Add(1)
				go func(num string, wg *sync.WaitGroup) {
					pod := newPod(num)
					_, err = client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
					if err != nil {
						fmt.Println("failed to create the pod: ", err)
						// os.Exit(1)
					}
					wg.Done()
				}(num, &wg)

				fmt.Printf("created pod perf-testing-pod-%v\n", num)
			}
			wg.Wait()
		case "replicasets":
			if delete {
				if err := client.AppsV1().ReplicaSets(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					fmt.Println("failed to delete the collection of replicasets: ", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			for i := 0; i < count; i++ {
				num := strconv.Itoa(i)
				rs := newReplicaset(num)
				_, err = client.AppsV1().ReplicaSets(namespace).Create(context.TODO(), rs, metav1.CreateOptions{})
				if err != nil {
					fmt.Println("failed to create the replicaset: ", err)
					os.Exit(1)
				}
				fmt.Printf("created replicaset perf-testing-rs-%v\n", num)
			}
		case "deployments":
			if delete {
				if err := client.AppsV1().Deployments(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
					fmt.Println("failed to delete the collection of deployments: ", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			for i := 0; i < count; i++ {
				num := strconv.Itoa(i)
				deploy := newDeployment(num)
				_, err = client.AppsV1().Deployments(namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
				if err != nil {
					fmt.Println("failed to create the deployment: ", err)
					os.Exit(1)
				}
				fmt.Printf("created deployment perf-testing-deploy-%v\n", num)
			}
		}
	}
}

func newPod(i string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perf-testing-pod-" + i,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "perf-testing",
			},
		},
		Spec: newPodSpec(),
	}
}

func newReplicaset(i string) *appsv1.ReplicaSet {
	r := int32(replicas)
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perf-testing-rs" + i,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "perf-testing",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &r,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "perf-testing",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "perf-testing",
					},
				},
				Spec: newPodSpec(),
			},
		},
	}
}

func newDeployment(i string) *appsv1.Deployment {
	r := int32(replicas)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perf-testing-deploy-" + i,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "perf-testing",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "perf-testing",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "perf-testing",
					},
				},
				Spec: newPodSpec(),
			},
		},
	}
}

func newPodSpec() corev1.PodSpec {
	boolTrue := true
	boolFalse := false
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "nginx",
				Image: "nginx",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: &boolFalse,
					RunAsNonRoot:             &boolTrue,
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "kwok.x-k8s.io/node",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "type",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"kwok"},
								},
							},
						},
					},
				},
			},
		},
	}
}
