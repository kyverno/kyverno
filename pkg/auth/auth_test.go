package auth

// import (
// 	"testing"
// 	"time"

// 	"github.com/golang/glog"
// 	"github.com/kyverno/kyverno/pkg/config"
// 	dclient "github.com/kyverno/kyverno/pkg/dclient"
// 	"github.com/kyverno/kyverno/pkg/signal"
// )

// func Test_Auth_pass(t *testing.T) {
// 	// needs running cluster
// 	var kubeconfig string
// 	stopCh := signal.SetupSignalHandler()
// 	kubeconfig = "/Users/shivd/.kube/config"
// 	clientConfig, err := config.CreateClientConfig(kubeconfig)
// 	if err != nil {
// 		glog.Fatalf("Error building kubeconfig: %v\n", err)
// 	}

// 	// DYNAMIC CLIENT
// 	// - client for all registered resources
// 	// - invalidate local cache of registered resource every 10 seconds
// 	client, err := dclient.NewClient(clientConfig, 10*time.Second, stopCh)
// 	if err != nil {
// 		glog.Fatalf("Error creating client: %v\n", err)
// 	}

// 	// Can i authenticate

// 	kind := "Deployment"
// 	namespace := "default"
// 	verb := "test"
// 	canI := NewCanI(client, kind, namespace, verb)
// 	ok, err := canI.RunAccessCheck()
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	if ok {
// 		t.Log("allowed")
// 	} else {
// 		t.Log("notallowed")
// 	}
// 	t.FailNow()

// }
