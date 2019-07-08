package utils

import (
	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//NewKubeInformerFactory returns a kubeinformer
func NewKubeInformerFactory(cfg *rest.Config) kubeinformers.SharedInformerFactory {
	// kubernetes client
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Errorf("error building kubernetes client: %s", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	return kubeInformerFactory
}
