package resourcecache

import (
	// "fmt"
	"k8s.io/client-go/tools/cache"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
	// handlers := cache.ResourceEventHandlerFuncs{
	//     AddFunc: func(obj interface{}) {
	// 		mObj := obj.(metav1.Object)
	// 		fmt.Println(mObj.GetName())
	// 		// fmt.Printf("Type : %T\n", obj)
	//         // fmt.Print("received add event!")
	//     },
	//     UpdateFunc: func(oldObj, obj interface{}) {
	//         // fmt.Println("received update event!")
	//     },
	//     DeleteFunc: func(obj interface{}) {
	//         // fmt.Println("received delete event!")
	//     },
	// }
	// s.AddEventHandler(handlers)
	s.Run(stopCh)
}
