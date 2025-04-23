package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"

	// "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	client := kubernetes.NewForConfigOrDie(config)
	groupResources, err := restmapper.GetAPIGroupResources(client.Discovery())
	if err != nil {
		panic(err)
	}
	bytes, err := json.MarshalIndent(groupResources, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}
