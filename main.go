package main

import (
    "runtime"
    "flag"
    "fmt"

    controller "nirmata/kube-policy/controller"
)

var (
    masterURL  string
    kubeconfig string
)

func main() {
    flag.Parse()

    controller, err := controller.NewController(masterURL, kubeconfig)
    if err != nil {
        fmt.Println("Error running Controller!")
    }

    err = controller.Run(runtime.NumCPU())
    if err != nil {
        fmt.Println("Error running Controller!")
    }
}

func init() {
    flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
    flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
