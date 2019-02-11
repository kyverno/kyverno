package main

import (
	"time"
	"flag"
	"fmt"
	
	"k8s.io/sample-controller/pkg/signals"
	"k8s.io/client-go/tools/clientcmd"

	clientset "nirmata/kube-policy/pkg/client/clientset/versioned"
	informers "nirmata/kube-policy/pkg/client/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()

	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error building example clientset: %v\n", err)
	}

	fmt.Println("Hello from Policy Controller!")

	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	controller := NewController(exampleClient, exampleInformerFactory.Nirmata().V1alpha1().Policies())
	exampleInformerFactory.Start(stopCh)
	if err = controller.Run(4, stopCh); err != nil {
		fmt.Println("Error running Controller!")
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
